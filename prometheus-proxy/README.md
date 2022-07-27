# prometheus-proxy

## 1.背景

​	prometheus-proxy是针对采集局域网的每个节点信息，不可能对所有节点都开放公网ip, 所以需要prometheus-agent和prometheus-proxy进行代理处理，该项目是引自于开源项目：https://github.com/pambrose/prometheus-prox; 它上是对接prometheus-proxy， 对下是对接promthues。

## 2.安装

1.拉取容器并启动

~~~bash
docker pull pambrose/prometheus-proxy:1.13.0

docker run -dit --name prometheus-proxy -p 8082:8082 -p 8092:8092 -p 50051:50051 -p 8080:8080  --env ADMIN_ENABLED=true --env METRICS_ENABLED=true  pambrose/prometheus-proxy:1.13.0

METRICS_ENABLED: true  提供prometheus-agent的metrics指标，这些都可以屏蔽，开放端口8082
ADMIN_ENABLED: true   提供管理者进行查询使用，这些都可以屏蔽，开放端口8092

50051：监听各个prometheus-agent的数据推送
8080： 暴露数据给prometheus的端口
~~~

