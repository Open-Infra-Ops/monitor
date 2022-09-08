# prometheus_gateway网关

## 1.背景

​		针对于cce、ecs、裸金属将监控数据推送给kafka， prometheus-gateway从kafka中获取数据，再将数据暴露给prometheus.

## 2.安装

1.修改配置文件： prometheus_gateway.yaml

~~~bash
service_port: 8000  # prometheus_gateway服务端口
kafka_server: 192.168.1.235:9092,192.168.1.196:9092,192.168.1.228:9092  # kafka的服务ip
kafka_topic: aom-metrics  # kafka的消费topic
kafka_consumer_id: prometheus_gateway  # kafka的消费id
metrics_info:  # 采集项目
  - item: node_cpu_seconds_total
    type: gauge
    name: node_cpu_seconds_total
    desc: Scrape_node_cpu_seconds_total
    labels: "node_name,cpu,model"
  - item: node_memory_MemUsed
    type: gauge
    name: node_memory_MemUsed
    desc: Scrape_node_memory_useage
    labels: "node_name"
  - item: node_filesystem_free_bytes
    type: gauge
    name: node_filesystem_free_bytes
    desc: Scrape filesystem free space in bytes
    labels: "node_name,device,fstype,mountpoint"
  - item: node_filesystem_size_bytes
    type: gauge
    name: node_filesystem_size_bytes
    desc: Scrape filesystem total size bytes
    labels: "node_name,device,fstype,mountpoint"
  - item: container_cpu_usage_seconds_total
    type: gauge
    name: container_cpu_usage_seconds_total
    desc: Scrape_the_usage_seconds_total_of_cpu
    labels: "cluster,account,name,namespace,pod"
  - item: container_spec_cpu_quota
    type: gauge
    name: container_spec_cpu_quota
    desc: Scrape_container_spec_cpu_quota
    labels: "cluster,account,name,namespace,pod"
  - item: container_spec_cpu_period
    type: gauge
    name: container_spec_cpu_period
    desc: Scrape_container_spec_cpu_period
    labels: "cluster,account,name,namespace,pod"
  - item: container_memory_usage_bytes
    type: gauge
    name: container_memory_usage_bytes
    desc: Scrape_container_memory_usage_bytes
    labels: "cluster,account,name,namespace,pod"
  - item: container_memory_max_usage_bytes
    type: gauge
    name: container_memory_max_usage_bytes
    desc: Scrape_container_memory_max_usage_bytes
    labels: "cluster,account,name,namespace,pod"
  - item: container_fs_usage_bytes
    type: gauge
    name: container_fs_usage_bytes
    desc: Scrape_container_fs_usage_bytes
    labels: "cluster,account,name,namespace,pod"
  - item: container_fs_limit_bytes
    type: gauge
    name: container_fs_limit_bytes
    desc: Scrape_container_fs_limit_bytes
    labels: "cluster,account,name,namespace,pod"
~~~

2.生成镜像

~~~bash
cd prometheus_gateway
docker build -t prometheus_gateway:v1.0 .
~~~

3.启动容器

~~~bash
docker run -dit --name prometheus_gateway -p 8000:8000 prometheus_gateway:v1.0
~~~

