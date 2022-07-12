一、node_exporter_proxy

1.node_exporter_proxy的作用

node_exporter_proxy是作为node_exporter的代理，对上游是和node_exporter通讯，对下游是prometheus，作为局域网采集点的代理。

![Image](https://github.com/Open-Infra-Ops/monitor/raw/main/metrics/node_exporter_proxy/docs/node_exporter_proxy.png)

2. node_exporter_proxy的选型
 + nginx：成熟产品，它是根据url分流，而prometheus请求的url是相同的，无法代理多个node_exporter，只能代理一个, 所以淘汰。
 + prometheus-proxy： https://github.com/pambrose/prometheus-proxy ； 开源项目，由java开发，中间代理服务，功能太多， 实现细节不可控。  
 + 自己开发中间转发服务： 实现细节完全可控，花费时间可能会较长。