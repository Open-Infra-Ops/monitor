# k8s_exporter

## 1.需求

​	k8s_exporter主要是采集prometheus通过remote_write推来的数据，再通过web接口暴露给prometheus.

## 2.实现

​	1.采集的指标

~~~bash
container_cpu_load_average_10s          gauge       过去10秒容器CPU的平均负载  	     
container_memory_usage_bytes            gauge       容器当前的内存使用量（单位：字节）
container_memory_max_usage_bytes        gauge       容器的最大内存使用量（单位：字节）
container_fs_usage_bytes                gauge       容器文件系统的使用量(单位：字节)
container_fs_limit_bytes                gauge       容器文件系统的总量(单位：字节)
~~~

​	2.采集的流程

​		c-advsior----> prometheus1---->k8s_exporter----> prometheus2

​	3.配置文件

+ prometheus1配置文件

  ~~~bash
  增加remote_write功能；详细参考sample-docker-prometheus.yml
  remote_write:
      - url: "http://prometheus_k8s_exporter:9201/write"
  ~~~

+ k8s_exporter的暴露点

  ~~~bash
  暴露网址：
  http://127.0.0.1:9201/metrics
  暴露信息：
  # HELP container_cpu_load_average_10s Average load of container CPU over the past 10 seconds
  # TYPE container_cpu_load_average_10s gauge
  container_cpu_load_average_10s{cluster="k8s-cluster",item="container_cpu_load_average_10s",job="hwstaff_beijing4",namespace="default",pod="nginx-deployment-585449566-lnknq"} 0
  container_cpu_load_average_10s{cluster="k8s-cluster",item="container_cpu_load_average_10s",job="hwstaff_beijing4",namespace="default",pod="nginx-deployment-585449566-m8mws"} 0
  container_cpu_load_average_10s{cluster="k8s-cluster",item="container_cpu_load_average_10s",job="hwstaff_beijing4",namespace="default",pod="nginx-deployment-585449566-n7ltf"} 0
  container_cpu_load_average_10s{cluster="k8s-cluster",item="container_cpu_load_average_10s",job="hwstaff_beijing4",namespace="default",pod="task-pv-pod"} 0
  container_cpu_load_average_10s{cluster="k8s-cluster",item="container_cpu_load_average_10s",job="hwstaff_beijing4",namespace="monitoring",pod="prometheus-69757bb88f-w98vk"} 0
  # HELP container_fs_limit_bytes The total amount of file system that the container can use (unit: bytes)
  # TYPE container_fs_limit_bytes gauge
  container_fs_limit_bytes{cluster="k8s-cluster",item="container_fs_limit_bytes",job="hwstaff_beijing4",namespace="default",pod="nginx-deployment-585449566-lnknq"} 4.2090070016e+10
  container_fs_limit_bytes{cluster="k8s-cluster",item="container_fs_limit_bytes",job="hwstaff_beijing4",namespace="default",pod="nginx-deployment-585449566-m8mws"} 4.2090070016e+10
  container_fs_limit_bytes{cluster="k8s-cluster",item="container_fs_limit_bytes",job="hwstaff_beijing4",namespace="default",pod="nginx-deployment-585449566-n7ltf"} 4.2090070016e+10
  container_fs_limit_bytes{cluster="k8s-cluster",item="container_fs_limit_bytes",job="hwstaff_beijing4",namespace="default",pod="task-pv-pod"} 4.2090070016e+10
  container_fs_limit_bytes{cluster="k8s-cluster",item="container_fs_limit_bytes",job="hwstaff_beijing4",namespace="monitoring",pod="prometheus-69757bb88f-w98vk"} 4.2090070016e+10
  # HELP container_fs_usage_bytes The usage of the file system in the container (unit: bytes)
  # TYPE container_fs_usage_bytes gauge
  container_fs_usage_bytes{cluster="k8s-cluster",item="container_fs_usage_bytes",job="hwstaff_beijing4",namespace="default",pod="nginx-deployment-585449566-lnknq"} 122880
  container_fs_usage_bytes{cluster="k8s-cluster",item="container_fs_usage_bytes",job="hwstaff_beijing4",namespace="default",pod="nginx-deployment-585449566-m8mws"} 40960
  container_fs_usage_bytes{cluster="k8s-cluster",item="container_fs_usage_bytes",job="hwstaff_beijing4",namespace="default",pod="nginx-deployment-585449566-n7ltf"} 122880
  container_fs_usage_bytes{cluster="k8s-cluster",item="container_fs_usage_bytes",job="hwstaff_beijing4",namespace="default",pod="task-pv-pod"} 1.8055168e+07
  container_fs_usage_bytes{cluster="k8s-cluster",item="container_fs_usage_bytes",job="hwstaff_beijing4",namespace="monitoring",pod="prometheus-69757bb88f-w98vk"} 1.28868352e+08
  # HELP container_memory_max_usage_bytes The maximum memory usage of the container (unit: bytes)
  # TYPE container_memory_max_usage_bytes gauge
  container_memory_max_usage_bytes{cluster="k8s-cluster",item="container_memory_max_usage_bytes",job="hwstaff_beijing4",namespace="default",pod="nginx-deployment-585449566-lnknq"} 4.38272e+06
  container_memory_max_usage_bytes{cluster="k8s-cluster",item="container_memory_max_usage_bytes",job="hwstaff_beijing4",namespace="default",pod="nginx-deployment-585449566-m8mws"} 5.8368e+06
  container_memory_max_usage_bytes{cluster="k8s-cluster",item="container_memory_max_usage_bytes",job="hwstaff_beijing4",namespace="default",pod="nginx-deployment-585449566-n7ltf"} 4.681728e+06
  container_memory_max_usage_bytes{cluster="k8s-cluster",item="container_memory_max_usage_bytes",job="hwstaff_beijing4",namespace="default",pod="task-pv-pod"} 6.9070848e+07
  container_memory_max_usage_bytes{cluster="k8s-cluster",item="container_memory_max_usage_bytes",job="hwstaff_beijing4",namespace="monitoring",pod="prometheus-69757bb88f-w98vk"} 5.771784192e+09
  # HELP container_memory_usage_bytes The current memory usage of the container (unit: bytes)
  # TYPE container_memory_usage_bytes gauge
  container_memory_usage_bytes{cluster="k8s-cluster",item="container_memory_usage_bytes",job="hwstaff_beijing4",namespace="default",pod="nginx-deployment-585449566-lnknq"} 229376
  container_memory_usage_bytes{cluster="k8s-cluster",item="container_memory_usage_bytes",job="hwstaff_beijing4",namespace="default",pod="nginx-deployment-585449566-m8mws"} 233472
  container_memory_usage_bytes{cluster="k8s-cluster",item="container_memory_usage_bytes",job="hwstaff_beijing4",namespace="default",pod="nginx-deployment-585449566-n7ltf"} 3.985408e+06
  container_memory_usage_bytes{cluster="k8s-cluster",item="container_memory_usage_bytes",job="hwstaff_beijing4",namespace="default",pod="task-pv-pod"} 5.111808e+06
  container_memory_usage_bytes{cluster="k8s-cluster",item="container_memory_usage_bytes",job="hwstaff_beijing4",namespace="monitoring",pod="prometheus-69757bb88f-w98vk"} 266240
  # HELP promhttp_metric_handler_errors_total Total number of internal errors encountered by the promhttp metric handler.
  # TYPE promhttp_metric_handler_errors_total counter
  promhttp_metric_handler_errors_total{cause="encoding"} 0
  promhttp_metric_handler_errors_total{cause="gathering"} 0
  ~~~

  





​		



