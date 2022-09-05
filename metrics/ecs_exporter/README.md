# ECS采集器ecs_exporter

## 1.背景

​	使用ecs_exporter采集各个ecs节点上暴露的数据，并将数据推送给kafka实例。

## 2.安装与卸载

1.安装

~~~bash
chmod +x install.sh
./install.sh
~~~

2.配置:	/etc/ecs_exporter/ecs_exporter.yaml

~~~bash
interval: 60  # 从各个node_exporter采集数据的时间间隔
service_port: 8000  # ecs_exporter服务暴露的端口
log_path: /var/log/ecs_exporter/  # ecs_exporter服务的日志路径
kafka_server: 192.168.1.235:9092,192.168.1.196:9092,192.168.1.228:9092  # kafka服务
kafka_topic_name: aom-metrics  # kafka生产的topic
node_info:
  - node_name: hwstatff_zone_node10  # 采集的节点信息
    node_ip: 127.0.0.1:9100   # 采集的ip
~~~

3.启动容器

~~~bash
systemctl start ecs_exporter
~~~

4.卸载

~~~bash
chmod +x remove.sh
./remove.sh
~~~

