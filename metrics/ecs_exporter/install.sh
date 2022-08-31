#!/usr/bin/env bash

bin_path="/opt/ecs_exporter"
log_path="/var/log/ecs_exporter"
etc_path="/etc/ecs_exporter"
service_path="/usr/lib/systemd/system"
cur_path=$(dirname $(readlink -f "$0"))

# install
pip3 install -r requirements.txt
# mkdir
if [ ! -d $bin_path ]; then
  mkdir $bin_path
fi
if [ ! -d $log_path ]; then
  mkdir $log_path
fi
if [ ! -d $etc_path ]; then
  mkdir $etc_path
fi
if [ ! -d $service_path ]; then
  mkdir $service_path
fi
# cp file
if [ ! -f $bin_path"/ecs_exporter.py" ]; then
  cp $cur_path"/ecs_exporter.py" $bin_path"/ecs_exporter.py"
  chmod 755 $bin_path"/ecs_exporter.py"
  mv $bin_path"/ecs_exporter.py" "/usr/bin/ecs_exporter"
fi
if [ ! -f $log_path"/ecs_exporter.log" ]; then
  touch $log_path"/ecs_exporter.log"
  chmod 755 $log_path"/ecs_exporter.log"
fi
if [ ! -f $etc_path"/ecs_exporter.yaml" ]; then
  cp $cur_path"/ecs_exporter.yaml" $etc_path"/ecs_exporter.yaml"
  chmod 755 $etc_path"/ecs_exporter.yaml"
fi
if [ ! -f $service_path"/ecs_exporter.service" ]; then
  cp $cur_path"/ecs_exporter.service" $service_path"/ecs_exporter.service"
  chmod 755 $service_path"/ecs_exporter.service"
fi
systemctl enable ecs_exporter.service

