# ECS采集器ecs_exporter

## 1.背景

​	使用ecs_exporter采集各个ecs节点上暴露的数据，并通过nginx暴露给prometheus.

## 2.安装

1.安装

~~~bash
chmod +x install.sh
./install.sh
~~~

2.配置:	/etc/ecs_exporter/ecs_exporter.yaml

~~~bash
interval: 10  # 从各个node_exporter采集数据的时间间隔
service_port: 8000  # ecs_exporter服务暴露的端口
log_path: /var/log/ecs_exporter/  # ecs_exporter服务的日志路径
node_info:
  - node_name: node_10  # 采集的exporter的node_name和node_ip
    node_ip: 43.154.162.239
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

