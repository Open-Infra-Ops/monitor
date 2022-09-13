package PrometheusClient

import (
	"encoding/json"
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/astaxie/beego/config"
	"github.com/astaxie/beego/logs"
	"github.com/prometheus/common/model"
	"math"
	"strconv"
	"time"
)

type Client struct {
	TableList   []MonItem
	kafkaClient sarama.SyncProducer
	baseConfig  *config.Configer
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
	Metrics string            `json:"metrics"`
	Items   map[string]string `json:"items"`
	Value   string            `json:"value"`
	Time    string            `json:"time"`
}

// NewClient creates a new client
func NewClient(p *sarama.SyncProducer, c *config.Configer) *Client {
	client := &Client{
		kafkaClient: *p,
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
	case "container_memory_working_set_bytes":
		isCheckOk = true
	case "container_spec_memory_limit_bytes":
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
	startCountTime := time.Now()
	serviceConfig := *c.baseConfig
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
		itemsMap := make(map[string]string)
		itemsMap["account"] = t.Job
		itemsMap["cluster"] = t.Cluster
		itemsMap["namespace"] = t.NameSpace
		itemsMap["pod"] = t.Pod
		itemsMap["container"] = t.Container
		c := CollectMonItem{
			Metrics: t.Name,
			Items:   itemsMap,
			Value:   strconv.FormatFloat(t.Value, 'E', -1, 64),
			Time:    fmt.Sprintf("%d", time.Now().Unix()),
		}
		collectMonItemList = append(collectMonItemList, c)
	}
	if len(collectMonItemList) == 0 {
		return nil
	}
	paymentDataBuf, _ := json.Marshal(&collectMonItemList)
	logs.Info("Collect data is:", string(paymentDataBuf))

	msg := &sarama.ProducerMessage{}
	msg.Topic = topics
	msg.Value = sarama.StringEncoder(paymentDataBuf)
	_, _, err := c.kafkaClient.SendMessage(msg)
	if err != nil {
		logs.Info("send msg failed, err:", err)
		return nil
	}
	EndCountTime := time.Now()
	spendTime := EndCountTime.Sub(startCountTime)
	logs.Info("Collect spend time:", spendTime)
	return nil
}

// HealthCheck implements the healthCheck interface
func (c *Client) HealthCheck() error {
	return nil
}

// Name identifies the client as a client.
func (c Client) Name() string {
	return "K8S_EXPORTER"
}
