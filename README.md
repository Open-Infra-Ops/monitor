# 性能监控

## 1.背景

​		运维离不开监控，想知道自己的集群节点是否正常运行，监控必不可少，因此，基础设施的监控模块的开发任务迫在眉睫。

## 2.需求

​		当前运维监控主要有两种类型，华为云的ECS和CCE，对于ECS，主要是通过node_exporter获取cpu，内存，硬盘总容量，硬盘使用容量；将数据暴露给prometheus；对于CCE，监控以容器为单位获取cpu, 内存，硬盘总容量，硬盘剩余容量，主要通过华为云自有的AOM模块向kafka数据上报，cce_exporter监控对应kafka的topic, 并将数据暴露给prometheus.

## 3.解决方案

![Image](https://github.com/Open-Infra-Ops/monitor/raw/main/doc/solution.png)

​		对于ecs局域网内，使用node_exporter获取节点的内存，cpu，硬盘总容量，硬盘的使用容量；部署时每个节点都需要部署一个node_exporter。

​		对于cce局域网内，使用aom+kafka+cce_exporter获取容器的内存，cpu，挂载盘总容量，挂载盘的使用容量；部署时一个CCE集群需要部署一个cce_exporter和kafka集群。

## 4.部署方案

ECS:

​	node_exporter： 使用rpm包的方式进行安装

​    prometheus-agent:  使用容器的方式部署该服务

CCE:

​	kafka： 在cce集群部署kafka集群。

​	cce_exporter： 在cce集群部署cce_exporter服务。

​	prometheus-agent:  在cce集群中部署prometheus-agent该服务

服务点：

​	prometheus： 在cce集群中部署prometheus服务。

​    prometheus-proxy:  在cce集群中部署prometheus-proxy该服务。



具体方案见：

​	node_exporter见metrics/node_exporter

​    cce_exporter见metrics/cce_exporter

​    kafka见kafka

​    prometheus-agent见prometheus-agent

​    prometheus-proxy见prometheus-proxy

​    prometheus见prometheus