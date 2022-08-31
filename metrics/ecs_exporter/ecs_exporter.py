#! /usr/bin/python3
# -*- coding: utf-8 -*-
# @Time    : 2022/7/14 17:15
# @Author  : Tom_zc
# @FileName: cce_exporter.py
# @Software: PyCharm
import datetime
import re
import yaml
import requests
import logging
import os
import asyncio
from apscheduler.schedulers.background import BackgroundScheduler
from flask import Flask, Response
from threading import Lock
from requests.packages.urllib3.exceptions import InsecureRequestWarning
from logging.handlers import RotatingFileHandler

requests.packages.urllib3.disable_warnings(InsecureRequestWarning)

APP = Flask(__name__)
loop = asyncio.get_event_loop()
logger = logging.getLogger('cce_exporter')


class GlobalConfig(object):
    config_path = "/etc/ecs_exporter/ecs_exporter.yaml"
    collect_url = "http://{}/metrics"
    default_log_name = "ecs_exporter.log"


class MetricData(object):
    _cur_metrics_data = dict()  # {"node_name": "content"}
    _lock = Lock()

    @classmethod
    def set(cls, metrics_data):
        with cls._lock:
            cls._cur_metrics_data.update(metrics_data)

    @classmethod
    def get(cls):
        with cls._lock:
            return cls._cur_metrics_data

    @classmethod
    def get_all_content(cls):
        all_content_dict = cls.get()
        content_str = str()
        for _, content in all_content_dict.items():
            content_str = "{}{}".format(content_str, content)
        return content_str


# noinspection DuplicatedCode
async def collect_node_data(node_name, node_ip):
    node_name = node_name.strip()
    content = str()
    try:
        url = GlobalConfig.collect_url.format(node_ip)
        ret = requests.get(url, timeout=(10, 10))
        if not str(ret.status_code).startswith("2"):
            raise Exception("get url:{} failed, code:{}".format(url, ret.status_code))
        content = ret.content.decode("utf-8")
    except Exception as e:
        logger.error("[query_data] {}".format(e))
    content_list = content.split("\n")
    ret_content = list()
    filed = 'node_name="{}",'.format(node_name)
    for content in content_list:
        if content.startswith("promhttp"):
            continue
        elif content.startswith("# HELP promhttp"):
            continue
        elif content.startswith("# TYPE promhttp"):
            continue
        elif content.startswith("node_memory_MemUsed"):
            content_line_list = content.split("node_memory_MemUsed")
            content = r'node_memory_MemUsed{%s}%s' % (filed[0:-1], content_line_list[-1])
        elif not content.startswith("#"):
            char_index = content.find(r"{")
            if char_index != -1:
                content = content[:char_index + 1] + filed + content[char_index + 1:]
        ret_content.append(content)
    collect_data_dict = {node_name: "\n".join(ret_content)}
    MetricData.set(collect_data_dict)


class CollectMetric(object):
    _lock = Lock()
    _instance = None
    _executor = None
    _apscheduler = BackgroundScheduler()

    def __new__(cls, *args, **kwargs):
        if cls._instance is None:
            cls._instance = super(CollectMetric, cls).__init__(*args, **kwargs)
        return cls._instance

    def __init__(self, *args, **kwargs):
        super(CollectMetric, self).__init__(*args, **kwargs)

    @classmethod
    def loop_collect_data(cls, node_info):
        try:
            global loop
            logger.info("[loop_collect_data] start to collect data")
            coroutine_task = [collect_node_data(node_temp["node_name"], node_temp["node_ip"]) for node_temp in
                              node_info]
            loop.run_until_complete(asyncio.wait(coroutine_task))
        except Exception as e:
            logger.error("[loop_collect_data] {}".format(e))

    @classmethod
    def init_task(cls, config_info):
        cls._apscheduler.add_job(cls.loop_collect_data, 'interval', args=(config_info["node_info"],),
                                 seconds=config_info["interval"], next_run_time=datetime.datetime.now())
        cls._apscheduler.start()


class InitProgress(object):

    @staticmethod
    def is_ip(ip_str):
        ip_str = ip_str.split(":")[0]
        p = re.compile('^((25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(25[0-5]|2[0-4]\d|[01]?\d\d?)$')
        if p.match(ip_str):
            return True
        else:
            return False

    @staticmethod
    def load_yaml(path=GlobalConfig.config_path):
        with open(path, "r", encoding="utf-8") as f:
            return yaml.load(f, Loader=yaml.FullLoader)

    @staticmethod
    def check_yaml_config(config):
        if not isinstance(config, dict):
            raise Exception("[check_yaml_config] invalid config")
        if not config.get("interval"):
            raise Exception("[check_yaml_config] invalid interval")
        if not config.get("service_port"):
            raise Exception("[check_yaml_config] invalid service_port")
        if not config.get("log_path"):
            raise Exception("[check_yaml_config] invalid log_path")
        if not os.path.exists(config["log_path"]):
            raise Exception("[check_yaml_config] invalid log_path")
        if not config.get("node_info") or not isinstance(config["node_info"], list):
            raise Exception("[check_yaml_config] invalid node_info")
        for node_temp in config["node_info"]:
            if not node_temp.get("node_name"):
                raise Exception("[check_yaml_config] invalid node_info: node_name:{}".format(node_temp["node_name"]))
            if not node_temp.get("node_ip"):
                raise Exception("[check_yaml_config] invalid node_info: ip:{}".format(node_temp["node_ip"]))
            elif not InitProgress.is_ip(node_temp["node_ip"]):
                raise Exception("[check_yaml_config] invalid node_info: ip:{}".format(node_temp["node_ip"]))

    @staticmethod
    def get_config_info():
        config_info = InitProgress.load_yaml()
        InitProgress.check_yaml_config(config_info)
        return config_info

    # noinspection DuplicatedCode
    @staticmethod
    def init_logger(config_info):
        global logger
        logger.setLevel(level=logging.INFO)
        file_name = "{}{}".format(config_info["log_path"], GlobalConfig.default_log_name)
        formatter = logging.Formatter('%(asctime)s %(name)-12s %(levelname)-8s %(message)s')
        size_rotate_file = RotatingFileHandler(filename=file_name, maxBytes=5 * 1024 * 1024,
                                               backupCount=10)
        size_rotate_file.setFormatter(formatter)
        size_rotate_file.setLevel(logging.INFO)
        logger.addHandler(size_rotate_file)


@APP.route("/")
def metrics():
    templates = '''<!DOCTYPE html>
    <html>
        <head><title>ECS Exporter</title></head>
        <body>
            <h1>CCE Exporter</h1>
            <p><a href='/metrics'>Metrics</a></p>
        </body>
    </html>'''
    return templates


@APP.route("/metrics")
def collect_metrics():
    content = MetricData.get_all_content()
    resp = Response(content)
    resp.headers['Content-Type'] = 'text/plain'
    return resp


def main():
    print("start to check config")
    config_info = InitProgress.get_config_info()
    InitProgress.init_logger(config_info)
    logger.info("start to init collect metrics task!")
    CollectMetric.init_task(config_info)
    logger.info("start to web, port:{}!".format(config_info["service_port"]))
    APP.run(host="0.0.0.0", port=config_info["service_port"], debug=False)


if __name__ == '__main__':
    main()
