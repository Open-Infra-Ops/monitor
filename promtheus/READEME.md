# 采集器prometheus

## 1.背景

​		使用prometheus主要是采集数据，将采集的数据放到本地的tsdb数据库中，方便后续使用。

## 2.安装

1.使用docker.io中的prometheus.

~~~~bash
docker.io/prom/prometheus     latest       8c01021bb2f4        5 weeks ago         211 MB
~~~~

2.修改配置文件

~~~bash
mkdir /opt/prometheus
cd /opt/prometheus/
vim prometheus.yml

global:
  scrape_interval:     120s
  evaluation_interval: 120s
  scrape_timeout: 120s

scrape_configs:
  - job_name: node_exporter_node10     # 针对于不同集群的同名的节点，可以修改job_name来做唯一性
    metrics_path: "/metrics_node10"    # promtheus-proxy设置的路径
    static_configs:
      - targets: ['tomtoworld.xyz:8080']  # promtheus-proxy设置的ip.

  - job_name: node_exporter_node11
    metrics_path: "/metrics_node11"
    static_configs:
      - targets: ['tomtoworld.xyz:8080']

~~~



3.启动容器

~~~bash
docker run -dit -p 9091:9091 \
-v /opt/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml \
-v /opt/prometheus/prometheus-data:/prometheus 
-v /data/prometheus/conf:/etc/prometheus/conf --name prometheus prom/prometheus \
--config.file=/etc/prometheus/prometheus.yml  \
--storage.tsdb.path=/prometheus  \
--storage.tsdb.retention=90d 

storage.tsdb.path: 存数据的位置
config.file: 以指定的配置启动
storage.tsdb.retention=90d： 数据有效期的时间90天
~~~
