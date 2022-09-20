# k8s_exporter

## 1.需求

​	非计算指标是k8s_exporter主要是采集prometheus通过remote_write推来的数据，再将数据推送给kafka.

​	计算指标是k8s_exporter通过每隔1分钟查询一次prometheus，并将数据推送到kafka，以避免数据异常抖动。

​	为什么要将数据区分为非计算指标和计算指标？

​	因为计算指标放在服务端计算，容易出现因为数据到达的时间不一致，导致prometheus采集的时候数据会有偏差，导致计算数据出现异常，所以计算指标需要在采集端计算好后发送给后台。

## 2.实现

### 	1.采集的指标

非计算指标：

~~~bash
container_cpu_usage_seconds_total       gauge       统计容器的 CPU 在一秒内消耗使用率，应注意的是该 container 所有的 CORE
container_spec_cpu_quota                gauge       是指容器的使用 CPU 时间周期总量，如果 quota 设置的是 700，000，就代表该容器可用的 CPU 时间是 7*100,000 微秒，通常对应 kubernetes 的 resource.cpu.limits 的值    
container_spec_cpu_period               gauge       当对容器进行 CPU 限制时，CFS 调度的时间窗口，又称容器 CPU 的时钟周期通常是 100，000 微秒       
container_memory_usage_bytes            gauge       容器当前的内存使用量（单位：字节）
container_memory_max_usage_bytes        gauge       容器的最大内存使用量（单位：字节）
container_fs_usage_bytes                gauge       容器文件系统的使用量(单位：字节)
container_fs_limit_bytes                gauge       容器文件系统的总量(单位：字节)
~~~

计算指标

~~~bash
container_cpu_usage_rate： 容器cpu使用率
container_mem_usage_rate： 容器内存使用率
container_fs_usage_rate ： 容器硬盘使用率
~~~

### 	2.采集的流程

​		c-advsior<----> prometheus1---->k8s_exporter----> kafka

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
[prometheus]
url = http://127.0.0.1:9898 # prometheus的url
interval = 60 # prometheus的采集时间
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



