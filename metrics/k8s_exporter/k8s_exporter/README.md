# k8s_exporter

## 1.需求

​	k8s_exporter主要是采集prometheus通过remote_write推来的数据，再通过web接口暴露给prometheus.

## 2.实现

### 	1.采集的指标

~~~bash
container_cpu_load_average_10s          gauge       过去10秒容器CPU的平均负载  	     
container_memory_usage_bytes            gauge       容器当前的内存使用量（单位：字节）
container_memory_max_usage_bytes        gauge       容器的最大内存使用量（单位：字节）
container_fs_usage_bytes                gauge       容器文件系统的使用量(单位：字节)
container_fs_limit_bytes                gauge       容器文件系统的总量(单位：字节)
~~~

### 	2.采集的流程

​		c-advsior----> prometheus1---->k8s_exporter----> prometheus2

### 	3.配置文件

#### 1.prometheus1配置文件

~~~bash
增加remote_write功能；详细参考sample-docker-prometheus.yml
remote_write:
    - url: "http://prometheus_k8s_exporter:9201/write"
~~~

#### 2.k8s_exporter的暴露点

~~~bash
暴露网址：
http://127.0.0.1:9201/metrics
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



