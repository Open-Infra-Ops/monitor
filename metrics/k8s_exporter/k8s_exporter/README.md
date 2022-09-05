# k8s_exporter

## 1.需求

​	k8s_exporter主要是采集prometheus通过remote_write推来的数据，再将数据推送给kafka.

## 2.实现

### 	1.采集的指标

~~~bash
container_cpu_usage_seconds_total       gauge
container_spec_cpu_quota                gauge
container_spec_cpu_period               gauge
container_memory_usage_bytes            gauge       容器当前的内存使用量（单位：字节）
container_memory_max_usage_bytes        gauge       容器的最大内存使用量（单位：字节）
container_fs_usage_bytes                gauge       容器文件系统的使用量(单位：字节)
container_fs_limit_bytes                gauge       容器文件系统的总量(单位：字节)
~~~

### 	2.采集的流程

​		c-advsior----> prometheus1---->k8s_exporter----> kafka

### 	3.配置文件

#### 1.prometheus1配置文件

~~~bash
增加remote_write功能；详细参考sample-docker-prometheus.yml
remote_write:
    - url: "http://prometheus_k8s_exporter:9201/write"
~~~

#### 2.k8s_exporter的配置文件 conf/app.conf

~~~bash
[web]
web-listen-address = 0.0.0.0:9201  #服务监听的端口
[log]
log_level = 6  # 日志级别
log_dir = ./logs # 日志路径
log_path = logs/k8s_exporter.log  #日志
maxlines=25000  # log文件最大行数
maxsize=204800   # log文件大小限制
[kafka]
brokers = 192.168.1.235:9092,192.168.1.196:9092,192.168.1.228:9092  # kafka服务端
topic_name = aom-metrics  # kafka生产的topic
~~~

### 4.生成docker镜像

~~~bash
docker build -t k8s_exporter:v1.0 .
~~~

### 5.启动容器

~~~bash
docker run -dit -p 9201:9201 --name k8s_exporter k8s_exporter:v1.0
~~~







​		



