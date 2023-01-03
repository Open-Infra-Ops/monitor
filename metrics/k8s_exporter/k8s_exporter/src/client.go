package PrometheusClient

import (
	"encoding/json"
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/astaxie/beego/config"
	"github.com/astaxie/beego/logs"
	"github.com/prometheus/common/model"
	"math"
	"net/http"
	"strconv"
	"time"
)

const (
	cpuOldRateQuery = `/api/v1/query?query=sum(rate(container_cpu_usage_seconds_total[5m]))by(job,cluster,namespace,name,pod)/sum(container_spec_cpu_quota/container_spec_cpu_period)by(job,cluster,namespace,name,pod)&time=`
	memOldRateQuery = `/api/v1/query?query=sum(container_memory_working_set_bytes)by(job,cluster,namespace,name,pod)/sum(container_spec_memory_limit_bytes)by(job,cluster,namespace,name,pod)*100!=+inf&time=`
	fsOldRateQuery  = `/api/v1/query?query=sum(container_fs_usage_bytes)by(job,cluster,namespace,name,pod)/sum(container_fs_limit_bytes)by(job,cluster,namespace,name,pod)*100!=+inf&time=`

	cpuRateQuery = `/api/v1/query?query=sum%28rate%28container_cpu_usage_seconds_total%5B5m%5D%29%29+by+%28job%2C+cluster%2Cnamespace%2Cname%2C+pod_name%29+%2F+sum%28container_spec_cpu_quota%2Fcontainer_spec_cpu_period%29+by+%28job%2C+cluster%2Cnamespace%2Cname%2C+pod_name%29+*+100+%21%3D+%2Binf&time=`
	memRateQuery = `/api/v1/query?query=sum%28container_memory_working_set_bytes%29+by+%28job%2C+cluster%2Cnamespace%2Cname%2C+pod_name%29+%2F+sum%28container_spec_memory_limit_bytes%29+by+%28job%2C+cluster%2Cnamespace%2Cname%2C+pod_name%29+*+100+%21%3D+%2Binf&time=`
	fsRateQuery  = `/api/v1/query?query=sum%28container_fs_usage_bytes%29+by%28job%2C+cluster%2Cnamespace%2Cname%2C+pod_name%29+%2F+sum%28container_fs_limit_bytes%29+by%28job%2C+cluster%2Cnamespace%2Cname%2C+pod_name%29+*+100+%21%3D+%2Binf&time=`
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

type PrometheusMetricData struct {
	Job       string `json:"job"`
	Name      string `json:"name"`
	NameSpace string `json:"namespace"`
	PodName   string `json:"pod_name"`
}

type PrometheusMetricObj struct {
	Metric PrometheusMetricData `json:"metric"`
	Value  []interface{}        `json:"value"`
}

type PrometheusData struct {
	Result     []PrometheusMetricObj `json:"result"`
	ResultType string                `json:"resultType"`
}

type PrometheusResponse struct {
	Data   PrometheusData `json:"data"`
	Status string         `json:"status"`
}

type OldPrometheusMetricData struct {
	Job       string `json:"job"`
	Name      string `json:"name"`
	NameSpace string `json:"namespace"`
	Pod       string `json:"pod"`
}

type OldPrometheusMetricObj struct {
	Metric OldPrometheusMetricData `json:"metric"`
	Value  []interface{}           `json:"value"`
}

type OldPrometheusData struct {
	Result     []OldPrometheusMetricObj `json:"result"`
	ResultType string                   `json:"resultType"`
}

type OldPrometheusResponse struct {
	Data   OldPrometheusData `json:"data"`
	Status string            `json:"status"`
}

// Name identifies the client as a client.
func (c Client) Name() string {
	return "K8S_EXPORTER"
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
		} else if pod, ok = labelStrings["pod_name"]; ok {
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
	if t.Job == "" || t.Cluster == "" || t.NameSpace == "" || t.Pod == "" || t.Name == "" || t.Container == "" || t.Container == t.Name {
		return false
	} else {
		return true
	}
}

// private func: parse value
func parseValue(f model.SampleValue) float64 {
	var value float64
	if math.IsNaN(float64(f)) {
		value = float64(-1)
	} else {
		value = float64(f)
	}
	return value
}

// private func: check job and name and namespace and pod_name is empty
func checkPrometheusParam(v PrometheusMetricData) bool {
	if v.PodName == "" || v.Name == "" || v.Job == "" || v.NameSpace == "" {
		return false
	} else {
		return true
	}
}

// private func: check job and name and namespace and pod is empty, and it is for the old version of k8s
func checkOldPrometheusParam(v OldPrometheusMetricData) bool {
	if v.Pod == "" || v.Name == "" || v.Job == "" || v.NameSpace == "" {
		return false
	} else {
		return true
	}
}

// private func: parse prometheus value
func parsePrometheusValue(i interface{}) float64 {
	var value float64
	strValue := i.(string)
	floatValue, _ := strconv.ParseFloat(strValue, 64)
	if math.IsNaN(floatValue) {
		value = float64(-1)
	} else {
		value = floatValue
	}
	return value
}

// Write implements the Writer interface and writes metric samples to the database
func (c *Client) Write(samples model.Samples) error {
	collectMonItemList := []CollectMonItem{}
	for _, sample := range samples {
		t := parseMetric(sample.Metric)
		if !checkName(t.Name) {
			continue
		} else if !checkNamespace(t.NameSpace) {
			continue
		} else if !checkParam(t) {
			continue
		}
		t.Value = parseValue(sample.Value)
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
	serviceConfig := *c.baseConfig
	topics := serviceConfig.String("kafka::topic_name")
	msg := &sarama.ProducerMessage{}
	msg.Topic = topics
	msg.Value = sarama.StringEncoder(paymentDataBuf)
	_, _, err := c.kafkaClient.SendMessage(msg)
	if err != nil {
		logs.Info("send msg failed, err:", err)
		return nil
	}
	return nil
}

// CollectRateData Collect rate data
func CollectRateData(p sarama.SyncProducer, c config.Configer, cn string, qu string) error {
	// 1 request prometheus data
	curTimeStamps := time.Now().Unix()
	prometheusUrl := c.String("prometheus::url")
	RateQueryFull := fmt.Sprintf(`%s%s%s`, prometheusUrl, qu, strconv.FormatInt(curTimeStamps, 10))
	resp, err := http.Get(RateQueryFull)
	if err != nil {
		logs.Error("[CollectRateData] request data failed! qu: %s, err:%s", qu, err)
		return err
	}
	// 2. parse data
	promResp := PrometheusResponse{}
	err = json.NewDecoder(resp.Body).Decode(&promResp)
	if err != nil {
		logs.Error("[CollectRateData] decode data failed! qu: %s, err:%s", qu, err)
		return err
	}
	collectMonItemList := []CollectMonItem{}
	for _, v := range promResp.Data.Result {
		if !checkNamespace(v.Metric.NameSpace) {
			continue
		} else if !checkPrometheusParam(v.Metric) {
			continue
		}
		value := parsePrometheusValue(v.Value[1])
		itemsMap := make(map[string]string)
		itemsMap["account"] = v.Metric.Job
		itemsMap["cluster"] = v.Metric.Job
		itemsMap["namespace"] = v.Metric.NameSpace
		itemsMap["pod"] = v.Metric.PodName
		itemsMap["container"] = v.Metric.Name
		c := CollectMonItem{
			Metrics: cn,
			Items:   itemsMap,
			Value:   strconv.FormatFloat(value, 'E', -1, 64),
			Time:    fmt.Sprintf("%d", time.Now().Unix()),
		}
		collectMonItemList = append(collectMonItemList, c)
	}
	if len(collectMonItemList) == 0 {
		return nil
	}
	// 3.send data to kafka
	paymentDataBuf, _ := json.Marshal(&collectMonItemList)
	logs.Info("Collect rate data is:", string(paymentDataBuf))
	topics := c.String("kafka::topic_name")
	msg := &sarama.ProducerMessage{}
	msg.Topic = topics
	msg.Value = sarama.StringEncoder(paymentDataBuf)
	_, _, err = p.SendMessage(msg)
	if err != nil {
		logs.Info("send msg failed, err:", err)
		return err
	}
	return nil
}

// CollectOldRateData Collect the new version k8s of rate data
func CollectOldRateData(p sarama.SyncProducer, c config.Configer, cn string, qu string) error {
	// 1 request prometheus data
	curTimeStamps := time.Now().Unix()
	prometheusUrl := c.String("prometheus::url")
	RateQueryFull := fmt.Sprintf(`%s%s%s`, prometheusUrl, qu, strconv.FormatInt(curTimeStamps, 10))
	resp, err := http.Get(RateQueryFull)
	if err != nil {
		logs.Error("[CollectRateData] request data failed! qu: %s, err:%s", qu, err)
		return err
	}
	// 2. parse data
	promResp := OldPrometheusResponse{}
	err = json.NewDecoder(resp.Body).Decode(&promResp)
	if err != nil {
		logs.Error("[CollectRateData] decode data failed! qu: %s, err:%s", qu, err)
		return err
	}
	collectMonItemList := []CollectMonItem{}
	for _, v := range promResp.Data.Result {
		if !checkNamespace(v.Metric.NameSpace) {
			continue
		} else if !checkOldPrometheusParam(v.Metric) {
			continue
		}
		value := parsePrometheusValue(v.Value[1])
		itemsMap := make(map[string]string)
		itemsMap["account"] = v.Metric.Job
		itemsMap["cluster"] = v.Metric.Job
		itemsMap["namespace"] = v.Metric.NameSpace
		itemsMap["pod"] = v.Metric.Pod
		itemsMap["container"] = v.Metric.Name
		c := CollectMonItem{
			Metrics: cn,
			Items:   itemsMap,
			Value:   strconv.FormatFloat(value, 'E', -1, 64),
			Time:    fmt.Sprintf("%d", time.Now().Unix()),
		}
		collectMonItemList = append(collectMonItemList, c)
	}
	if len(collectMonItemList) == 0 {
		return nil
	}
	// 3.send data to kafka
	paymentDataBuf, _ := json.Marshal(&collectMonItemList)
	logs.Info("Collect rate data is:", string(paymentDataBuf))
	topics := c.String("kafka::topic_name")
	msg := &sarama.ProducerMessage{}
	msg.Topic = topics
	msg.Value = sarama.StringEncoder(paymentDataBuf)
	_, _, err = p.SendMessage(msg)
	if err != nil {
		logs.Info("send msg failed, err:", err)
		return err
	}
	return nil
}

// StartCpuRateCollect Start a coroutine to collect cpu data
func StartCpuRateCollect(p sarama.SyncProducer, c config.Configer) {
	intervalTimer, _ := c.Int64("prometheus::interval")
	podName := c.String("prometheus::pod_name")
	for range time.Tick(time.Duration(intervalTimer) * time.Second) {
		cn := "container_cpu_usage_rate"
		if podName == "pod" {
			qu := cpuOldRateQuery
			_ = CollectOldRateData(p, c, cn, qu)
		} else {
			qu := cpuRateQuery
			_ = CollectRateData(p, c, cn, qu)
		}
	}
}

// StartMemRateCollect Start a coroutine to collect mem data
func StartMemRateCollect(p sarama.SyncProducer, c config.Configer) {
	intervalTimer, _ := c.Int64("prometheus::interval")
	podName := c.String("prometheus::pod_name")
	for range time.Tick(time.Duration(intervalTimer) * time.Second) {
		cn := "container_mem_usage_rate"
		if podName == "pod" {
			qu := memOldRateQuery
			_ = CollectOldRateData(p, c, cn, qu)
		} else {
			qu := memRateQuery
			_ = CollectRateData(p, c, cn, qu)
		}

	}

}

// StartFsRateCollect Start a coroutine to collect fs data
func StartFsRateCollect(p sarama.SyncProducer, c config.Configer) {
	intervalTimer, _ := c.Int64("prometheus::interval")
	podName := c.String("prometheus::pod_name")
	for range time.Tick(time.Duration(intervalTimer) * time.Second) {
		cn := "container_fs_usage_rate"
		if podName == "pod" {
			qu := fsOldRateQuery
			_ = CollectOldRateData(p, c, cn, qu)
		} else {
			qu := fsRateQuery
			_ = CollectRateData(p, c, cn, qu)
		}

	}

}
