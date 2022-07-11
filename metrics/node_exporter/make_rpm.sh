#!/usr/bin/env bash
cur_path=$(pwd)
top_dir=$(rpm --eval "%{_topdir}")
cd cur_path"/../"
tar -zcvf node_exporter-1.3.1-0.tar.gz node_exporter
cp node_exporter-1.3.1-0.tar.gz top_dir"/SOURCES"
make
rpmbuild -ba node_exporter.spec

