package mysql_prometheus

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	// 	"github.com/satori/go.uuid"
	"crypto/md5"
	"math"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	// "syscall"
	//"github.com/timescale/prometheus-postgresql-adapter/log"
	"pm_adapter/log"

	//"github.com/timescale/prometheus-postgresql-adapter/util"

	_ "github.com/go-sql-driver/mysql"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
)

// Config for the database
type Config struct {
	host             string
	port             int
	user             string
	password         string
	database         string
	schema           string
	sslMode          string
	table            string
	copyTable        string
	maxOpenConns     int
	maxIdleConns     int
	logSamples       bool
	useTimescaleDb   bool
	dbConnectRetries int
	readOnly         bool
}

// ParseFlags parses the configuration flags specific to MySql
func ParseFlags(cfg *Config, url string, pass string) *Config {
	flag.StringVar(&cfg.host, "host", url, "The MySQL host")
	flag.IntVar(&cfg.port, "port", 3306, "The MySQL port")
	flag.StringVar(&cfg.user, "user", "root", "The MySQL user")
	flag.StringVar(&cfg.password, "password", pass, "The MySQL password")
	flag.IntVar(&cfg.maxOpenConns, "max-open-conns", 50, "The max number of open connections to the database")
	flag.IntVar(&cfg.maxIdleConns, "max-idle-conns", 10, "The max number of idle connections to the database")
	flag.IntVar(&cfg.dbConnectRetries, "db-connect-retries", 0, "How many times to retry connecting to the database")
	flag.BoolVar(&cfg.readOnly, "read-only", false, "Read-only mode. Don't write to database. Useful when pointing adapter to read replica")
	return cfg
}

// Client sends Prometheus samples to PostgreSQL
type Client struct {
	DB        *sql.DB
	cfg       *Config
	TableList []MonItem
}

type MonItem struct {
	Name   string
	Host   string
	Dev    string
	Module string
}

type MetricMonItem struct {
	Time   int64
	Host   string
	Dev    string
	Module string
	Value  map[string]float64
}

var (
	MemData map[string]MetricMonItem
	mutex   sync.Mutex
)

const (
	DbName = "monitor"

	moduleHost        = "host"
	moduleHostNetCard = "hostNetCard"
	moduleDisk        = "disk"
	modulePools       = "pools"
	moduleRbd         = "rbd"
	moduleRgwService  = "rgw_service"
	moduleRgwUser     = "rgw_user"

	sqlInsertHostMonValue   = "insert ignore host_mon(time, host, node_cpu_seconds_total, node_memory_MemUsed) values(?,?,?,?)"
	sqlInsertHostMonNetCard = "insert ignore host_mon_netcard(time,host,dev,node_network_receive_bytes_total, node_network_receive_packets_total," +
		"node_network_transmit_bytes_total, node_network_transmit_packets_total," +
		"node_network_drop_total, node_network_errs_total) values(?,?,?,?,?,?,?,?,?);"
	sqlInsertDiskValue       = "insert ignore disks_mon(time, name, ceph_r_ops, ceph_w_ops, ceph_r_bytes, ceph_w_bytes, ceph_r_await, ceph_w_await) values(?,?,?,?,?,?,?,?);"
	sqlInsertPoolValue       = "insert ignore pool_mon(time, name, ceph_pool_rd, ceph_pool_wr, ceph_pool_rd_bytes, ceph_pool_wr_bytes) values(?,?,?,?,?,?);"
	sqlInsertRbdValue        = "insert ignore %s(time, ceph_rbd_read_ops, ceph_rbd_write_ops, ceph_rbd_read_bytes, ceph_rbd_write_bytes) values(?,?,?,?,?);"
	sqlInsertRgwServiceValue = "insert ignore rgw_service_mon(time, name, ceph_rgw_service_get, ceph_rgw_service_put, ceph_rgw_service_delete," +
		"ceph_rgw_service_suc_ops, ceph_rgw_service_failed_ops, ceph_rgw_service_r_bytes, ceph_rgw_service_w_bytes," +
		"ceph_rgw_service_r_wait, ceph_rgw_service_w_wait) values(?,?,?,?,?,?,?,?,?,?,?);"
	sqlInsertRgwUserValue = "insert ignore %s(time, ceph_rgw_user_get, ceph_rgw_user_put, ceph_rgw_user_delete," +
		"ceph_rgw_user_suc_ops, ceph_rgw_user_failed_ops," +
		"ceph_rgw_user_r_bytes, ceph_rgw_user_w_bytes," +
		"ceph_rgw_user_r_wait, ceph_rgw_user_w_wait) values(?,?,?,?,?,?,?,?,?,?);"

	sqlCreateRbdTable = "create table if not exists %s(time bigint(11),ceph_rbd_read_ops bigint(20),ceph_rbd_write_ops bigint(20),ceph_rbd_read_bytes bigint(20)," +
		"ceph_rbd_write_bytes bigint(20),INDEX time (time ASC))ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci partition by range(time) (partition pm values less than(maxvalue));"
	sqlCreateRbdDumpTable = "create table if not exists %s(time bigint(11), ceph_rbd_read_ops float(32,2),ceph_rbd_write_ops float(32,2),ceph_rbd_read_bytes float(32,2)," +
		"ceph_rbd_write_bytes float(32,2),type_int tinyint(2),INDEX time (time ASC))ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci partition by range(time) (partition pm values less than(maxvalue));"

	sqlCreateRgwUserTable = "create table if not exists %s(time bigint(11), ceph_rgw_user_get bigint(20), ceph_rgw_user_put bigint(20),ceph_rgw_user_delete bigint(20)," +
		"ceph_rgw_user_suc_ops bigint(20),ceph_rgw_user_failed_ops bigint(20), ceph_rgw_user_r_wait bigint(20), ceph_rgw_user_w_wait bigint(20)," +
		"ceph_rgw_user_r_bytes bigint(20),ceph_rgw_user_w_bytes bigint(20), INDEX time (time ASC))ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci partition by range(time) (partition pm values less than(maxvalue));"

	sqlCreateRgwUserDumpTable = "create table if not exists %s(time bigint(11), ceph_rgw_user_get float(32,2), ceph_rgw_user_put float(32,2),ceph_rgw_user_delete float(32,2)," +
		"ceph_rgw_user_suc_ops float(32,2),ceph_rgw_user_failed_ops float(32,2),ceph_rgw_user_r_wait bigint(20),ceph_rgw_user_w_wait bigint(20)," +
		"ceph_rgw_user_r_bytes float(32,2),ceph_rgw_user_w_bytes float(32,2),type_int tinyint(2),INDEX time (time ASC))ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci partition by range(time) (partition pm values less than(maxvalue));"
)

func init() {
	MemData = make(map[string]MetricMonItem)
}

// NewClient creates a new PostgreSQL client
func NewClient(cfg *Config) *Client {
	configPath := "/etc/dsm/monitor/dbconfig"
	file, err := os.Open(configPath)
	if err != nil {
		panic("read config error")
	}
	reader := bufio.NewReader(file)
	buf, _, _ := reader.ReadLine()
	host := string(buf)
	buf, _, _ = reader.ReadLine()
	password := string(buf)
	file.Close()
	os.Remove(configPath)
	cfg.password = password
	cfg.host = host
	connStr := fmt.Sprintf("%v:%v@tcp(%v:%v)/%s?charset=utf8&parseTime=true",
		cfg.user, cfg.password, cfg.host, cfg.port, DbName)
	db, err := sql.Open("mysql", connStr)
	if err != nil {
		log.Error("err", err)
		os.Exit(1)
	}
	db.SetMaxOpenConns(cfg.maxOpenConns)
	db.SetMaxIdleConns(cfg.maxIdleConns)
	client := &Client{
		DB:  db,
		cfg: cfg,
	}

	count_i := 1
	for {
		result := client.initTable()
		if result == true {
			break
		} else {
			if count_i > 10 {
				log.Error("err", "Connect mysql error.")
				os.Exit(1)
			}
		}
		count_i++
		time.Sleep(time.Second * 5)
	}

	return client
}

func (c *Client) ReadOnly() bool {
	return c.cfg.readOnly
}

func (c *Client) initTable() bool {
	rows, err := c.DB.Query("SELECT 1")
	if err != nil {
		log.Error("err", "initTable error")
		return false
	}
	defer rows.Close()
	return true
}

func parseMetric(m model.Metric) MonItem {
	metricName, hasName := m[model.MetricNameLabel]
	numLabels := len(m) - 1
	if !hasName {
		numLabels = len(m)
	}
	labelStrings := make(map[string]string, numLabels)
	for label, value := range m {
		if label != model.MetricNameLabel {
			strLabel := string(label)
			strValue := string(value)
			labelStrings[strLabel] = strValue
		}
	}
	item := MonItem{
		Name:   "",
		Host:   "",
		Dev:    "",
		Module: "",
	}

	switch numLabels {
	case 0:
		if hasName {
			item.Name = string(metricName)
			return item
		}
		return item
	default:
		dev := ""
		item.Name = string(metricName)
		if s, ok := labelStrings["device"]; ok {
			dev = s
			item.Module = moduleHostNetCard
		} else if _, ok := labelStrings["mode"]; ok {
			dev = "host"
			item.Module = moduleHost
		} else if p, ok := labelStrings["pool_name"]; ok {
			dev = p
			item.Module = modulePools
		} else if rs, ok := labelStrings["rgw_service"]; ok {
			dev = rs
			item.Module = moduleRgwService
		} else if ru, ok := labelStrings["rgw_user"]; ok {
			dev = ru
			item.Module = moduleRgwUser
		} else if rb, ok := labelStrings["disks"]; ok {
			dev = rb
			item.Module = moduleDisk
		} else if i, ok := labelStrings["image"]; ok {
			if pool, p_ok := labelStrings["pool"]; p_ok {
				dev = pool + "/" + i
			} else {
				dev = "/" + i
			}
			item.Module = moduleRbd
		} else {
			if item.Name == "node_memory_MemUsed" {
				dev = "host"
				item.Module = moduleHost
			}
		}
		host := ""
		if s, ok := labelStrings["instance"]; ok {
			s := strings.Split(s, ":")
			if len(s) > 0 {
				host = s[0]
			}
		}
		item.Host = host
		item.Dev = dev
		return item
	}

}

func checkName(name string, dev string) bool {
	prefix1 := "promhttp_metric_handler_"
	n1 := len(prefix1)
	prefix2 := "scrape_"
	n2 := len(prefix2)

	if len(name) > n1 && name[:n1] == prefix1 {
		return false
	}

	if len(name) > n2 && name[:n2] == prefix2 {
		return false
	}

	if name == "up" {
		return false
	}

	if dev == ".rgw.extra" {
		return false
	}

	if dev == ".rgw.root" {
		return false
	}

	if name == "ceph_scrape_duration_secs" {
		return false
	}
	return true
}

func getMdValue(name string) string {
	data := []byte(name)
	tableStr := md5.Sum(data)
	tableName := fmt.Sprintf("%x", tableStr)
	return tableName
}

func deleteMapKey(mapMonData map[string]MetricMonItem, map_key string) map[string]MetricMonItem {
	newMapMonData := make(map[string]MetricMonItem)
	for key, value := range mapMonData {
		if key != map_key {
			newMapMonData[key] = value
		}
	}
	return newMapMonData
}

func isValidMetric(metricMon MetricMonItem) bool {
	l := len(metricMon.Value)
	if metricMon.Module == moduleHost && l == 2 {
		return true
	} else if metricMon.Module == moduleHostNetCard && l == 6 {
		return true
	} else if metricMon.Module == moduleDisk && l == 6 {
		return true
	} else if metricMon.Module == modulePools && l == 4 {
		return true
	} else if metricMon.Module == moduleRbd && l == 4 {
		return true
	} else if metricMon.Module == moduleRgwService && l == 9 {
		return true
	} else if metricMon.Module == moduleRgwUser && l == 9 {
		return true
	} else {
		return false
	}
}

func getInCompleteData(mapMonData map[string]MetricMonItem) []MetricMonItem {
	tempMetricMonArrayItems := make([]MetricMonItem, 0, len(mapMonData))
	for _, item := range mapMonData {
		if isValidMetric(item) == false {
			tempMetricMonArrayItems = append(tempMetricMonArrayItems, item)
		}
	}
	return tempMetricMonArrayItems
}

// Write implements the Writer interface and writes metric samples to the database
func (c *Client) Write(samples model.Samples) error {
	begin := time.Now()
	allMetricMonItems := make(map[string]MetricMonItem)
	for _, sample := range samples {
		item := MetricMonItem{}
		t := parseMetric(sample.Metric)
		if !checkName(t.Name, t.Dev) {
			continue
		}
		if math.IsNaN(float64(sample.Value)) {
			sample.Value = -1
		}
		value, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", float64(sample.Value)), 64)
		milliseconds := sample.Timestamp.UnixNano() / 1000000000
		f := strconv.FormatInt(milliseconds, 10)
		map_k := f + "_" + string(t.Module) + "_" + string(t.Host) + "_" + string(t.Dev)
		if _, ok := allMetricMonItems[map_k]; ok {
			allMetricMonItems[map_k].Value[t.Name] = value
		} else {
			item.Time = milliseconds
			item.Host = t.Host
			item.Dev = t.Dev
			item.Module = t.Module
			m := make(map[string]float64)
			m[t.Name] = value
			item.Value = m
			allMetricMonItems[map_k] = item
		}
	}
	mutex.Lock()
	tempMetricMonArrayItems := getInCompleteData(allMetricMonItems)
	for _, t := range tempMetricMonArrayItems {
		f := strconv.FormatInt(t.Time, 10)
		mapK := f + "_" + string(t.Module) + "_" + string(t.Host) + "_" + string(t.Dev)
		if _, ok := MemData[mapK]; ok {
			for key, value := range t.Value {
				MemData[mapK].Value[key] = value
			}
			if isValidMetric(MemData[mapK]) == true {
				allMetricMonItems[mapK] = MemData[mapK]
				MemData = deleteMapKey(MemData, mapK)
			} else {
				allMetricMonItems = deleteMapKey(allMetricMonItems, mapK)
			}
		} else {
			MemData[mapK] = t
			allMetricMonItems = deleteMapKey(allMetricMonItems, mapK)
		}

	}
	mutex.Unlock()

	txTable, err := c.DB.Begin()
	defer txTable.Rollback()
	if err != nil {
		log.Error("msg", "Error on Begin when create table", "err", err)
		return err
	}
	for _, item := range allMetricMonItems {
		if item.Module == moduleHost {
			hostStmt, err := txTable.Prepare(sqlInsertHostMonValue)
			if err != nil {
				log.Error("msg", "Error Prepare hostStmt", "err", err)
				return err
			}
			_, err = hostStmt.Exec(item.Time, item.Host, item.Value["node_cpu_seconds_total"], item.Value["node_memory_MemUsed"])
			if err != nil {
				log.Error("msg", "Error executing hostStmt", "err", err)
				return err
			}
			hostStmt.Close()
		} else if item.Module == moduleHostNetCard {
			HostNetCardStmt, err := txTable.Prepare(sqlInsertHostMonNetCard)
			if err != nil {
				log.Error("msg", "Error Prepare HostNetCardStmt", "err", err)
				return err
			}
			_, err = HostNetCardStmt.Exec(item.Time, item.Host, item.Dev,
				item.Value["node_network_receive_bytes_total"], item.Value["node_network_receive_packets_total"],
				item.Value["node_network_transmit_bytes_total"], item.Value["node_network_transmit_packets_total"],
				item.Value["node_network_drop_total"], item.Value["node_network_errs_total"])
			if err != nil {
				log.Error("msg", "Error executing HostNetCardStmt", "err", err)
				return err
			}
			HostNetCardStmt.Close()
		} else if item.Module == moduleDisk {
			DiskStmt, err := txTable.Prepare(sqlInsertDiskValue)
			if err != nil {
				log.Error("msg", "Error Prepare DiskStmt", "err", err)
				return err
			}
			_, err = DiskStmt.Exec(item.Time, item.Dev, item.Value["ceph_r_ops"], item.Value["ceph_w_ops"], item.Value["ceph_r_bytes"], item.Value["ceph_w_bytes"],
				item.Value["ceph_r_await"], item.Value["ceph_w_await"])
			if err != nil {
				log.Error("msg", "Error executing DiskStmt", "err", err)
				return err
			}
			DiskStmt.Close()
		} else if item.Module == modulePools {
			PoolStmt, err := txTable.Prepare(sqlInsertPoolValue)
			if err != nil {
				log.Error("msg", "Error Prepare PoolStmt", "err", err)
				return err
			}
			_, err = PoolStmt.Exec(item.Time, item.Dev, item.Value["ceph_pool_rd"], item.Value["ceph_pool_wr"],
				item.Value["ceph_pool_rd_bytes"], item.Value["ceph_pool_wr_bytes"])
			if err != nil {
				log.Error("msg", "Error executing PoolStmt", "err", err)
				return err
			}
			PoolStmt.Close()
		} else if item.Module == moduleRbd {
			tableName := "rbd_mon_" + getMdValue(item.Dev)
			sqlRbdTable := fmt.Sprintf(sqlCreateRbdTable, tableName)
			rbdTableStmt, err := txTable.Prepare(sqlRbdTable)
			if err != nil {
				log.Error("msg", "Error Prepare rbdTableStmt", "err", err)
				return err
			}
			_, err = rbdTableStmt.Exec()
			if err != nil {
				log.Error("msg", "Error executing rbdTableStmt", "err", err)
				return err
			}
			rbdTableStmt.Close()
			tableDumpName := "rbd_mon_dump_" + getMdValue(item.Dev)
			sqlRbdDumpTable := fmt.Sprintf(sqlCreateRbdDumpTable, tableDumpName)
			rbdDumpTableStmt, err := txTable.Prepare(sqlRbdDumpTable)
			if err != nil {
				log.Error("msg", "Error Prepare rbdDumpTableStmt", "err", err)
				return err
			}
			_, err = rbdDumpTableStmt.Exec()
			if err != nil {
				log.Error("msg", "Error executing rbdDumpTableStmt", "err", err)
				return err
			}
			rbdDumpTableStmt.Close()
			sqlInsertRbd := fmt.Sprintf(sqlInsertRbdValue, tableName)
			RbdStmt, err := txTable.Prepare(sqlInsertRbd)
			if err != nil {
				log.Error("msg", "Error Prepare RbdStmt", "err", err)
				return err
			}
			_, err = RbdStmt.Exec(item.Time, item.Value["ceph_rbd_read_ops"], item.Value["ceph_rbd_write_ops"],
				item.Value["ceph_rbd_read_bytes"], item.Value["ceph_rbd_write_bytes"])
			if err != nil {
				log.Error("msg", "Error executing RbdStmt", "err", err)
				return err
			}
			RbdStmt.Close()
		} else if item.Module == moduleRgwService {
			RgwServiceStmt, err := txTable.Prepare(sqlInsertRgwServiceValue)
			if err != nil {
				log.Error("msg", "Error Prepare RgwServiceStmt", "err", err)
				return err
			}
			_, err = RgwServiceStmt.Exec(item.Time, item.Dev, item.Value["ceph_rgw_service_get"],
				item.Value["ceph_rgw_service_put"], item.Value["ceph_rgw_service_delete"],
				item.Value["ceph_rgw_service_suc_ops"], item.Value["ceph_rgw_service_failed_ops"],
				item.Value["ceph_rgw_service_r_bytes"], item.Value["ceph_rgw_service_w_bytes"],
				item.Value["ceph_rgw_service_r_wait"], item.Value["ceph_rgw_service_w_wait"])
			if err != nil {
				log.Error("msg", "Error executing RgwServiceStmt", "err", err)
				return err
			}
			RgwServiceStmt.Close()
		} else if item.Module == moduleRgwUser {
			tableName := "rgw_user_mon_" + getMdValue(item.Dev)
			sqlRgwUserTable := fmt.Sprintf(sqlCreateRgwUserTable, tableName)
			rgwUserTableStmt, err := txTable.Prepare(sqlRgwUserTable)
			if err != nil {
				log.Error("msg", "Error Prepare rgwUserTableStmt", "err", err)
				return err
			}
			_, err = rgwUserTableStmt.Exec()
			if err != nil {
				log.Error("msg", "Error executing rgwUserTableStmt", "err", err)
				return err
			}
			rgwUserTableStmt.Close()
			tableDumpName := "rgw_user_mon_dump_" + getMdValue(item.Dev)
			sqlRgwUserDumpTable := fmt.Sprintf(sqlCreateRgwUserDumpTable, tableDumpName)
			rgwUserDumpTableStmt, err := txTable.Prepare(sqlRgwUserDumpTable)
			if err != nil {
				log.Error("msg", "Error Prepare rgwUserDumpTableStmt", "err", err)
				return err
			}
			_, err = rgwUserDumpTableStmt.Exec()
			if err != nil {
				log.Error("msg", "Error executing rgwUserDumpTableStmt", "err", err)
				return err
			}
			rgwUserDumpTableStmt.Close()
			sqlInsertRgwUser := fmt.Sprintf(sqlInsertRgwUserValue, tableName)
			RgwUserStmt, err := txTable.Prepare(sqlInsertRgwUser)
			if err != nil {
				log.Error("msg", "Error Prepare RgwUserStmt", "err", err)
				return err
			}
			_, err = RgwUserStmt.Exec(item.Time, item.Value["ceph_rgw_user_get"], item.Value["ceph_rgw_user_put"], item.Value["ceph_rgw_user_delete"],
				item.Value["ceph_rgw_user_suc_ops"], item.Value["ceph_rgw_user_failed_ops"], item.Value["ceph_rgw_user_r_bytes"],
				item.Value["ceph_rgw_user_w_bytes"], item.Value["ceph_rgw_user_r_wait"], item.Value["ceph_rgw_user_w_wait"])
			if err != nil {
				log.Error("msg", "Error executing RgwUserStmt", "err", err)
				return err
			}
			RgwUserStmt.Close()
		}
	}
	txTable.Commit()
	duration := time.Since(begin).Seconds()
	duration = duration
	//log.Debug("msg", "Wrote samples", "count", len(samples), "duration", duration)
	return nil
}

type sampleLabels struct {
	JSON        []byte
	Map         map[string]string
	OrderedKeys []string
}

func createOrderedKeys(m *map[string]string) []string {
	keys := make([]string, 0, len(*m))
	for k := range *m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (c *Client) Close() {
	if c.DB != nil {
		if err := c.DB.Close(); err != nil {
			log.Error("msg", err.Error())
		}
	}
}

func (l *sampleLabels) Scan(value interface{}) error {
	if value == nil {
		l = &sampleLabels{}
		return nil
	}

	switch t := value.(type) {
	case []uint8:
		m := make(map[string]string)
		err := json.Unmarshal(t, &m)

		if err != nil {
			return err
		}

		*l = sampleLabels{
			JSON:        t,
			Map:         m,
			OrderedKeys: createOrderedKeys(&m),
		}
		return nil
	}
	return fmt.Errorf("invalid labels value %s", reflect.TypeOf(value))
}

func (l sampleLabels) String() string {
	return string(l.JSON)
}

func (l sampleLabels) key(extra string) string {
	// 0xff cannot cannot occur in valid UTF-8 sequences, so use it
	// as a separator here.
	separator := "\xff"
	pairs := make([]string, 0, len(l.Map)+1)
	pairs = append(pairs, extra+separator)

	for _, k := range l.OrderedKeys {
		pairs = append(pairs, k+separator+l.Map[k])
	}
	return strings.Join(pairs, separator)
}

func (l *sampleLabels) len() int {
	return len(l.OrderedKeys)
}

// Read implements the Reader interface and reads metrics samples from the database
func (c *Client) Read(req *prompb.ReadRequest) (*prompb.ReadResponse, error) {
	labelsToSeries := map[string]*prompb.TimeSeries{}
	resp := prompb.ReadResponse{
		Results: []*prompb.QueryResult{
			{
				Timeseries: make([]*prompb.TimeSeries, 0, len(labelsToSeries)),
			},
		},
	}
	log.Debug("msg", "Returned response", "#timeseries", len(labelsToSeries))
	return &resp, nil
}

// HealthCheck implements the healtcheck interface
func (c *Client) HealthCheck() error {
	rows, err := c.DB.Query("SELECT 1")
	if err != nil {
		log.Debug("msg", "Health check error", "err", err)
		return err
	}
	rows.Close()
	return nil
}

// Name identifies the client as a PostgreSQL client.
func (c Client) Name() string {
	return "MySQL"
}

// Describe implements prometheus.Collector.
func (c *Client) Describe(ch chan<- *prometheus.Desc) {
}

// Collect implements prometheus.Collector.
func (c *Client) Collect(ch chan<- prometheus.Metric) {
}
