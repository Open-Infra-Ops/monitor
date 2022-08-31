#!/usr/bin/env bash

bin_path="/opt/ecs_exporter"
log_path="/var/log/ecs_exporter"
etc_path="/etc/ecs_exporter"
service_path="/usr/lib/systemd/system"

# remove
rm -rf $bin_path
rm -rf $log_path
rm -rf $etc_path
rm -rf $service_path"/ecs_exporter.service"