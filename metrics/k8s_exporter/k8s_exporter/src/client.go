package PrometheusClient

import (
	"encoding/json"
	"fmt"
	"github.com/astaxie/beego/config"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"log"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	TableList   []MonItem
	kafkaClient *kafka.Producer
	baseConfig  config.Configer
}

type MonItem struct {
	Job       string
	Cluster   string
	NameSpace string
	Pod       string
	Name      string
	Container string
	Value     float64
	Info      map[string]string
}

type CollectMonItem struct {
	metrics string
	items   MonItem
	value   string
	time    string
}

// NewClient creates a new client
func NewClient(p *kafka.Producer, c config.Configer) *Client {
	client := &Client{
		kafkaClient: p,
		baseConfig:  c,
	}
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
		Container: "",
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
		if container, ok := labelStrings["name"]; ok {
			if container != "POD" {
				item.Container = container
			}
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
	if t.Container == "" {
		return false
	}
	if t.Container == t.Name {
		return false
	}
	return true
}

// Write implements the Writer interface and writes metric samples to the database
func (c *Client) Write(samples model.Samples) error {
	serviceConfig := c.baseConfig
	topics := serviceConfig.String("kafka::topic_name")
	collectMonItemList := []CollectMonItem{}
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
		c := CollectMonItem{
			metrics: t.Name,
			items:   t,
			value:   strconv.FormatFloat(t.Value, 'E', -1, 64),
			time:    fmt.Sprintf("%d", time.Now().Unix()),
		}
		collectMonItemList = append(collectMonItemList, c)
	}
	paymentDataBuf, _ := json.Marshal(&collectMonItemList)
	err := c.kafkaClient.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topics, Partition: kafka.PartitionAny},
		Value:          paymentDataBuf,
	}, nil)
	if err != nil {
		log.Panicf("send message fail, err: %v", err)
		return err
	}
	for e := range c.kafkaClient.Events() {
		switch ev := e.(type) {
		case *kafka.Message:
			if ev.TopicPartition.Error != nil {
				log.Printf("Delivery failed: %v\n", ev.TopicPartition)
			} else {
				log.Printf("Delivered message to %v\n", ev.TopicPartition)
			}
		}
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
