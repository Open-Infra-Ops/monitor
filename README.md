# 性能监控

## 1.背景

​		运维离不开监控，想知道自己的集群节点是否正常运行，监控必不可少，因此，基础设施的监控模块的开发任务迫在眉睫。

## 2.需求

​		当前运维监控主要有两种类型，华为云的ECS，CCE，裸金属。

​		对于ECS和裸金属，主要是通过node_exporter获取cpu，内存，硬盘总容量，硬盘使用容量，并将数据暴露给prometheus。

​		对于CCE，监控以容器为单位获取cpu, 内存，硬盘总容量，硬盘剩余容量；主要有两种方法实现：

+ 通过华为云自有的AOM模块向kafka数据上报，cce_exporter监听对应kafka的topic，并将数据暴露给prometheus；但是存在限制条件： 只能使用华为云的kafka实例。
+ 通过c-advisor对接prometheus，再通过prometheus的rewriter将数据导出到k8s_exporter，k8s_exporter再将数据暴露给prometheus。

## 3.解决方案

​		<img src="doc/solution.png" alt="解决方案"/>

## 4.部署方案

ECS/裸金属:

​	node_exporter： 使用rpm包的方式进行安装

​    prometheus-agent:  使用容器的方式部署该服务

CCE_方案一:

​	kafka： 在cce集群部署kafka集群。

​	cce_exporter： 在cce集群部署cce_exporter服务。

​	prometheus-agent:  在cce集群中部署prometheus-agent该服务

CCE_方案二:

​	prometheus： 在cce集群部署prometheus。

​	k8s_exporter： 在cce集群部署k8s_exporter。

​	prometheus-agent:  在cce集群中部署prometheus-agent该服务

服务点：

​	prometheus： 在cce集群中部署prometheus服务。

​    prometheus-proxy:  在cce集群中部署prometheus-proxy该服务。

具体方案见：

​    kafka见kafka(kafka只能用华为云的kafka实例)

​	node_exporter见metrics/node_exporter

​    cce_exporter见metrics/cce_exporter

​    k8s_exporter见metrics/k8s_exporter

​    prometheus-agent见prometheus-agent

​    prometheus-proxy见prometheus-proxy

​    prometheus见prometheus