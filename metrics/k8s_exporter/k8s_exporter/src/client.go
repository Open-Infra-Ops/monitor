package PrometheusClient

import (
	"encoding/json"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"math"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
)

type Client struct {
	TableList     []MonItem
	telemetryPath string
}

type Config struct {
	remoteTimeout     time.Duration
	listenAddr        string
	telemetryPath     string
	logLevel          string
	prometheusTimeout time.Duration
}

type MonItem struct {
	Job       string
	Cluster   string
	NameSpace string
	Pod       string
	Name      string
	Value     float64
	Info      map[string]string
}

var (
	RwMutex sync.RWMutex
	MemData map[string]MonItem
)

func init() {
	MemData = make(map[string]MonItem)
}

// NewClient creates a new client
func NewClient(telemetryPath string) *Client {
	client := &Client{telemetryPath: telemetryPath}
	return client
}

// private func: parse metric data to MonItem
func parseMetric(m model.Metric) MonItem {
	metricName, hasName := m[model.MetricNameLabel]
	numLabels := len(m) - 1
	if !hasName {
		numLabels = len(m)
	}
	labelStrings := make(map[string]string, numLabels)
	for label, value := range m {
		strValue := string(value)
		if label != model.MetricNameLabel {
			strLabel := string(label)
			labelStrings[strLabel] = strValue
		} else {
			labelStrings["name"] = strValue
		}
	}
	item := MonItem{
		Job:       "",
		Cluster:   "",
		NameSpace: "",
		Pod:       "",
		Name:      "",
		Value:     0,
		Info:      nil,
	}
	switch numLabels {
	case 0:
		if hasName {
			item.Name = string(metricName)
			item.Info = labelStrings
			return item
		}
		return item
	default:
		item.Name = string(metricName)
		item.Info = labelStrings
		if job, ok := labelStrings["job"]; ok {
			item.Job = job
		}
		if cluster, ok := labelStrings["cluster"]; ok {
			item.Cluster = cluster
		}
		if namespace, ok := labelStrings["namespace"]; ok {
			item.NameSpace = namespace
		}
		if pod, ok := labelStrings["pod"]; ok {
			item.Pod = pod
		}
		return item
	}
}

// private func: check metric name
func checkName(name string) bool {
	isCheckOk := false
	switch name {
	case "container_cpu_usage_seconds_total":
		isCheckOk = true
	case "container_spec_cpu_quota":
		isCheckOk = true
	case "container_spec_cpu_period":
		isCheckOk = true
	case "container_memory_usage_bytes":
		isCheckOk = true
	case "container_memory_max_usage_bytes":
		isCheckOk = true
	case "container_fs_usage_bytes":
		isCheckOk = true
	case "container_fs_limit_bytes":
		isCheckOk = true
	default:
		isCheckOk = false
	}
	if isCheckOk {
		return true
	} else {
		return false
	}
}

// private func: check Namespace
func checkNamespace(namespace string) bool {
	if namespace == "kube-system" {
		return false
	} else {
		return true
	}
}

// private func: check param is empty
func checkParam(t MonItem) bool {
	if t.Job == "" {
		return false
	}
	if t.Cluster == "" {
		return false
	}
	if t.NameSpace == "" {
		return false
	}
	if t.Pod == "" {
		return false
	}
	if t.Name == "" {
		return false
	}
	return true
}

// private func: set Mem Data
func setMemData(t MonItem) {
	RwMutex.Lock()
	defer RwMutex.Unlock()
	mapKey := t.Job + "_" + t.Cluster + "_" + t.NameSpace + "_" + t.Pod + "_" + t.Name
	MemData[mapKey] = t
}

// GetMemData get mem data
func GetMemData() map[string]MonItem {
	RwMutex.RLock()
	defer RwMutex.RUnlock()
	return MemData
}

// Write implements the Writer interface and writes metric samples to the database
func (c *Client) Write(samples model.Samples) error {
	for _, sample := range samples {
		t := parseMetric(sample.Metric)
		if !checkName(t.Name) {
			continue
		}
		if !checkNamespace(t.NameSpace) {
			continue
		}
		if !checkParam(t) {
			continue
		}
		var value float64
		if math.IsNaN(float64(sample.Value)) {
			value = float64(-1)
		} else {
			value = float64(sample.Value)
		}
		t.Value = value
		setMemData(t)
		//temp := t.Job + "_" + t.Cluster + "_" + t.NameSpace + "_" + t.Pod + "_" + t.Name + "_" + strconv.FormatFloat(t.Value, 'E', -1, 64)
		//logs.Info("msg", "data:", "collect--->", temp)
	}
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
	// 0xff cannot occur in valid UTF-8 sequences, so use it
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
	return &resp, nil
}

// HealthCheck implements the healthCheck interface
func (c *Client) HealthCheck() error {
	return nil
}

// Name identifies the client as a client.
func (c Client) Name() string {
	return "K8S_EXPORTER"
}

// Describe implements prometheus.Collector.
func (c *Client) Describe(ch chan<- *prometheus.Desc) {
}

// Collect implements prometheus.Collector.
func (c *Client) Collect(ch chan<- prometheus.Metric) {
}
