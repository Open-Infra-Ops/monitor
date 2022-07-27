# ecs采集器node_exporter

## 1.背景

​	node_expoter主要是采集节点的内存，cpu， 硬盘总容量，剩余容量的数据， 它以web服务的方式将数据暴露给prometheus，它使用systemd服务进行托管，并设置为开机自启动。

## 2.编译

进入node_exporter， 参考README文档进行源码编译。

## 3.安装

​	将pkg下的rpm包推到安装节点进行安装

~~~bash
rpm -ivh node_exporter-0.18.1-0.x86_64.rpm
安装后可以查看服务是否正常：
systemctl status node_exporter

后续考虑到升级， 请执行升级包操作：
rpm -Uvh node_exporter-0.18.1-0.x86_64.rpm

卸载操作：
rpm -e node_exporter-0.18.1-0.x86_64.rpm
~~~









​		