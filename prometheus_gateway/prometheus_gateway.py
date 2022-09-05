# -*- coding: utf-8 -*-
# @Time    : 2022/7/14 17:15
# @Author  : Tom_zc
# @FileName: cce_exporter.py
# @Software: PyCharm
import json
import math
import re
import yaml
import requests
import logging

from flask import Flask, Response
from threading import Thread, Lock, Event
from requests.packages.urllib3.exceptions import InsecureRequestWarning
from logging.handlers import RotatingFileHandler
from kafka import KafkaConsumer

requests.packages.urllib3.disable_warnings(InsecureRequestWarning)

APP = Flask(__name__)

logger = logging.getLogger('prometheus_gateway')


class GlobalConfig(object):
    config_path = "/opt/prometheus_gateway/prometheus_gateway.yaml"


class MetricData(object):
    """save the metric data"""
    _cur_metrics_data = dict()
    _lock = Lock()

    _cur_web_data = str()
    _web_lock = Lock()

    @classmethod
    def set(cls, metrics_data):
        with cls._lock:
            cls._cur_metrics_data.update(metrics_data)

    @classmethod
    def get(cls):
        with cls._lock:
            return cls._cur_metrics_data

    @classmethod
    def web_set(cls, metrics_data):
        with cls._web_lock:
            cls._cur_web_data = metrics_data

    @classmethod
    def web_get(cls):
        with cls._web_lock:
            return cls._cur_web_data


class Metric(object):

    def __init__(self, mtype, name, desc, labels=None):
        self.mtype = mtype
        self.name = name
        self.desc = desc
        self.labelnames = labels  # tuple if present
        self.value = {}  # indexed by label values

    def clear(self):
        self.value = {}

    def set(self, value, labelvalues=None):
        # labelvalues must be a tuple
        labelvalues = labelvalues or ('',)
        self.value[labelvalues] = value

    def str_expfmt(self):

        def promethize(path):
            ''' replace illegal metric name characters '''
            result = re.sub(r'[./\s]|::', '_', path).replace('+', '_plus')

            # Hyphens usually turn into underscores, unless they are
            # trailing
            if result.endswith("-"):
                result = result[0:-1] + "_minus"
            else:
                result = result.replace("-", "_")

            return "cce_{0}".format(result)

        def floatstr(value):
            ''' represent as Go-compatible float '''
            if value == float('inf'):
                return '+Inf'
            if value == float('-inf'):
                return '-Inf'
            if math.isnan(value):
                return 'NaN'
            return repr(float(value))

        name = promethize(self.name)
        expfmt = '# HELP {name} {desc}\n# TYPE {name} {mtype}\n'.format(
            name=name,
            desc=self.desc,
            mtype=self.mtype,
        )
        for labelvalues, value in self.value.items():
            if self.labelnames:
                labels = zip(self.labelnames, labelvalues)
                labels = ','.join('%s="%s"' % (k, v) for k, v in labels)
            else:
                labels = ''
            if labels:
                fmtstr = '{name}{{{labels}}} {value}\n'
            else:
                fmtstr = '{name} {value}\n'
            expfmt += fmtstr.format(
                name=name,
                labels=labels,
                value=floatstr(value),
            )
        return expfmt


class ExposeMetric(object):
    def __init__(self, cur_metric_dict, metrics_config):
        self.metrics_dict = self._setup_expose_metric(cur_metric_dict, metrics_config)

    @staticmethod
    def _setup_expose_metric(cur_metric_dict, metrics_config):
        metric_dict = dict()
        for tuple_info, _ in cur_metric_dict.items():
            metrics_type = metrics_config["type"]
            metrics_name = metrics_config["name"]
            metrics_desc = metrics_config["desc"]
            metrics_labels = metrics_config["labels"]
            metric_dict[tuple_info] = Metric(metrics_type, metrics_name, metrics_desc, metrics_labels)
        return metric_dict

    def set_metric_data(self, metric_dict):
        for tuple_info, value in metric_dict.items():
            self.metrics_dict[tuple_info].clear()
        # for tuple_info, value in metric_dict.items():
            self.metrics_dict[tuple_info].set(value, tuple_info[0:-1])


class EipTools(object):
    _lock = Lock()
    _event = Event()
    _metrics_config = None

    def __init__(self, *args, **kwargs):
        super(EipTools, self).__init__(*args, **kwargs)

    @classmethod
    def loop_collect_data(cls, config_info):
        consumer = KafkaConsumer(config_info["kafka_topic"], bootstrap_servers=config_info["kafka_server"],
                                 group_id=config_info["prometheus_gateway"])
        metrics_config = cls.parse_metrics_info(config_info)
        for message in consumer:
            content = message.value.decode("utf-8")
            list_data = json.loads(content)
            for metrics_info in list_data:
                metrics_key_list = list(metrics_info["items"].values())
                metrics_key_list.append(metrics_info["metrics"])
                dict_data = {
                    tuple(metrics_key_list): list_data["value"]
                }
                MetricData.set(dict_data)
            # refresh web data
            cur_metric_dict = MetricData.get()
            expose_metric = ExposeMetric(cur_metric_dict, metrics_config)
            expose_metric.set_metric_data(cur_metric_dict)
            ret_metric = [m.str_expfmt() for m in expose_metric.metrics_dict.values()]
            MetricData.web_set(''.join(ret_metric))

    @staticmethod
    def load_yaml(path=GlobalConfig.config_path):
        with open(path, "r", encoding="utf-8") as f:
            return yaml.load(f, Loader=yaml.FullLoader)

    @classmethod
    def check_yaml_config(cls, config):
        if not isinstance(config, dict):
            raise Exception("[check_yaml_config] invalid config")
        if not config.get("service_port"):
            raise Exception("[check_yaml_config] invalid service_port")
        if not config.get("kafka_server"):
            raise Exception("[check_yaml_config] invalid kafka_server")
        if not config.get("kafka_topic"):
            raise Exception("[check_yaml_config] invalid kafka_topic")
        if not config.get("kafka_consumer_id"):
            raise Exception("[check_yaml_config] invalid kafka_consumer_id")
        if not config.get("metrics_info") or not isinstance(config["metrics_info"], list):
            raise Exception("[check_yaml_config] invalid metrics_info")
        for metrics_temp in config["metrics_info"]:
            if not metrics_temp.get("item"):
                raise Exception("[check_yaml_config] invalid node_info:{}".format(metrics_temp["item"]))
            if not metrics_temp.get("type"):
                raise Exception("[check_yaml_config] invalid node_info:{}".format(metrics_temp["type"]))
            if not metrics_temp.get("name"):
                raise Exception("[check_yaml_config] invalid node_info:{}".format(metrics_temp["name"]))
            if not metrics_temp.get("desc"):
                raise Exception("[check_yaml_config] invalid node_info:{}".format(metrics_temp["desc"]))
            if not metrics_temp.get("labels"):
                raise Exception("[check_yaml_config] invalid node_info:{}".format(metrics_temp["labels"]))

    @classmethod
    def get_config_info(cls):
        config_info = cls.load_yaml()
        cls.check_yaml_config(config_info)
        return config_info

    @classmethod
    def parse_metrics_info(cls, config_dict):
        metrics_info = config_dict["metrics_info"]
        dict_data = dict()
        for metrics_dict in metrics_info:
            dict_data[metrics_dict["item"]] = {
                "type": metrics_dict["type"],
                "name": metrics_dict["name"],
                "desc": metrics_dict["desc"],
                "labels": tuple(metrics_dict["labels"].split(",")),
            }
        return dict_data

    @classmethod
    def get_metrics_config(cls):
        if cls._metrics_config is None:
            config = cls.get_config_info()
            cls._metrics_config = cls.parse_metrics_info(config)
        return cls._metrics_config

    @classmethod
    def init_logger(cls):
        global logger
        logger.setLevel(level=logging.INFO)
        formatter = logging.Formatter('%(asctime)s %(name)-12s %(levelname)-8s %(message)s')
        size_rotate_file = RotatingFileHandler(filename='prometheus_gateway.log', maxBytes=5 * 1024 * 1024,
                                               backupCount=10)
        size_rotate_file.setFormatter(formatter)
        size_rotate_file.setLevel(logging.INFO)

        console_handler = logging.StreamHandler()
        console_handler.setLevel(level=logging.INFO)
        console_handler.setFormatter(formatter)

        logger.addHandler(size_rotate_file)
        logger.addHandler(console_handler)

    @classmethod
    def init_task(cls, config_info):
        logger.info("##################start to collect thread#############")
        th = Thread(target=cls.loop_collect_data, args=(config_info,), daemon=True)
        th.start()



@APP.route("/")
def metrics():
    templates = '''<!DOCTYPE html>
    <html>
        <head><title>Prometheus_Gateway Exporter</title></head>
        <body>
            <h1>Prometheus_Gateway Exporter</h1>
            <p><a href='/metrics'>Metrics</a></p>
        </body>
    </html>'''
    return templates


@APP.route("/metrics")
def collect_metrics():
    content = MetricData.web_get()
    resp = Response(content)
    resp.headers['Content-Type'] = 'text/plain'
    return resp


def main():
    EipTools.init_logger()
    config_info = EipTools.get_config_info()
    EipTools.init_task(config_info)
    APP.run(port=config_info["service_port"], debug=False)


if __name__ == '__main__':
    main()
