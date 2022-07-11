#!/usr/bin/env bash
version=$1
if [[ -z $version ]];then
  version="1.3.1"
fi
cur_path=$(pwd)
top_dir=$(rpm --eval "%{_topdir}")
cd $cur_path
# arm: CGO_ENABLED=1 GOOS=linux GOARCH=arm64 CC_FOR_TARGET=gcc-aarch64-linux-gnu CC=aarch64-linux-gnu-gcc go build
go build node_exporter.go
pkg_name="/node_exporter-"$version"-0.x86_64"
build_root_path="/BUILDROOT"
rm -rf $top_dir$build_root_path$pkg_name
mkdir -p $top_dir$build_root_path$pkg_name"/usr/bin"
mkdir -p $top_dir$build_root_path$pkg_name"/usr/lib/systemd/system"
cp node_exporter $top_dir$build_root_path$pkg_name"/usr/bin"
cp examples/systemd/node_exporter.service $top_dir$build_root_path$pkg_name"/usr/lib/systemd/system"
sed "s#{version}#$version#g" node_exporter.ba.spec > node_exporter.spec
rpmbuild -ba node_exporter.spec

