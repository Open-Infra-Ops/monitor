#! /usr/bin/python3
# -*- coding: utf-8 -*-
# @Time    : 2022/7/14 17:15
# @Author  : Tom_zc
# @FileName: cce_exporter.py
# @Software: PyCharm
import datetime
import json
import re
import time
import traceback
import yaml
import requests
import logging
import os
import asyncio
from flask import Flask
from threading import Lock
from kafka import KafkaProducer
from apscheduler.schedulers.background import BackgroundScheduler
from requests.packages.urllib3.exceptions import InsecureRequestWarning
from logging.handlers import RotatingFileHandler

requests.packages.urllib3.disable_warnings(InsecureRequestWarning)

APP = Flask(__name__)
loop = asyncio.get_event_loop()
logger = logging.getLogger('ecs_exporter')


class GlobalConfig(object):
    config_path = "/etc/ecs_exporter/ecs_exporter.yaml"
    collect_url = "http://{}/metrics"
    default_log_name = "ecs_exporter.log"


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
    for content in content_list:
        items_dict = {"node_name": node_name}
        if content.startswith("promhttp"):
            continue
        elif content.startswith("#"):
            continue
        elif not content:
            continue
        else:
            content_temp_list = content.split(r"{")
            if len(content_temp_list) > 1:
                remain_list = content_temp_list[-1].split(r"}")

                items_list = remain_list[0].split(",")
                for items in items_list:
                    items_temp = items.split("=")
                    items_dict[items_temp[0]] = items_temp[1]
                dict_data = {
                    "metrics": content_temp_list[0],
                    "items": items_dict,
                    "value": remain_list[-1].strip(),
                    "time": time.time()
                }
                ret_content.append(dict_data)
            elif len(content_temp_list) == 1:
                items_list = content_temp_list[0].split()
                dict_data = {
                    "metrics": items_list[0],
                    "items": items_dict,
                    "value": items_list[-1].strip(),
                    "time": int(time.time())
                }
                ret_content.append(dict_data)
            else:
                logger.info("[collect_node_data] invalid data:{}".format(content))
    return ret_content


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
    def loop_collect_data(cls, config_info, consumer):
        global loop
        try:
            node_info = config_info["node_info"]
            topic_name = config_info["kafka_topic_name"]
            logger.info("[loop_collect_data] start to collect data")
            coroutine_task = [collect_node_data(node_temp["node_name"], node_temp["node_ip"]) for node_temp in
                              node_info]
            ret_tuple = loop.run_until_complete(asyncio.wait(coroutine_task))
            ret_list = list()
            for i in list(ret_tuple[0]):
                ret_list.extend(i.result())
            if ret_list:
                ret_str = json.dumps(ret_list)
                data = bytes(ret_str, encoding="utf-8")
                consumer.send(topic_name, data)
            else:
                logger.info("[loop_collect_data] Getting data is empty")
        except Exception as e:
            logger.error("[loop_collect_data] {}:{}".format(e, traceback.format_exc()))

    @classmethod
    def init_task(cls, config_info):
        kafka_server_list = config_info["kafka_server"].split(",")
        consumer = KafkaProducer(bootstrap_servers=kafka_server_list)
        cls._apscheduler.add_job(cls.loop_collect_data, 'interval', args=(config_info, consumer),
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
        if not config.get("kafka_server"):
            raise Exception("[check_yaml_config] invalid kafka_server")
        if not config.get("kafka_topic_name"):
            raise Exception("[check_yaml_config] invalid kafka_topic_name")
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
            <h1>ECS Exporter</h1>
            <p>health</p>
        </body>
    </html>'''
    return templates


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
