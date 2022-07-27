# prometheus-agent

## 1.背景

​		prometheus-agent是针对采集局域网的每个节点信息，不可能对所有节点都开放公网ip, 所以需要prometheus-agent和prometheus-proxy进行代理处理，该项目是引自于开源项目：https://github.com/pambrose/prometheus-prox; 它上是对接exporter， 对下是对接prometheus-proxy。

## 2.安装

1.修改配置文件：prometheus-proxy-agent.conf， 然后推到github上。

~~~bash
agent {
  proxy.hostname = 43.154.162.239  # proxy的ip.
  #metrics.enabled: true  提供prometheus-agent的metrics指标，这些都可以屏蔽，开放端口8083
  #admin.enabled: true   提供管理者进行查询使用，这些都可以屏蔽，开放端口8093
  pathConfigs: [
    {
      name: "node10"   # 采集的节点名称
      path: metrics_node10  # # 暴露给prometheus的路径，需要和prometheus的metrics_path保持一致
      url: "http://1.14.102.183:9101/metrics"  # exporter的ip
    }
    {
      name: "node11"
      path: metrics_node11
      url:"http://1.14.102.183:9102/metrics"
    }
  ]
}
~~~

2.安装

~~~
docker pull pambrose/prometheus-agent:1.13.0

docker run -dit --name prometheus-agent --env AGENT_CONFIG='https://raw.githubusercontent.com/Open-Infra-Ops/monitor/main/prometheus-agent/prometheus-proxy-agent.conf' pambrose/prometheus-agent:1.13.0
~~~





