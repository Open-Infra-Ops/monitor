# CCE采集器cce_exporter

## 1.背景

​		针对于cce集群提供的数据，需要暴露给prometheus, AOM通过kafka将数据推给cce_exporter， cce_exporter将数据暴露给prometheus-agent.

## 2.安装

1.制作docker镜像

~~~bash
cd ecs_exporter
docker build -t ecs_exporter:v1.0 .
~~~

2.设置全局变量

~~~bash
启动容器之前需要设置全局变量:
kafka_server="192.168.1.235:9092,192.168.1.196:9092,192.168.1.228:9092" # kafka的集群ip
kafka_topic="aom-metrics" # kafka监听的topic
kafka_consumer_id="comsumer-aom-metrics" # kafka的消费id, 随便填，但是不能和其他的消费id重复。
port=8000 # cce_exporter服务端口，需要和Dockerfile的expose端口保持一致。
~~~

3.启动容器

~~~bash
docker run -dit --name ecs_exporter -p 8000:8000 ecs_exporter:v1.0
~~~

