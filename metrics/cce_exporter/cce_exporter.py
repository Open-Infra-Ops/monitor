# -*- coding: utf-8 -*-
# @Time    : 2022/7/14 17:15
# @Author  : Tom_zc
# @FileName: cce_exporter.py
# @Software: PyCharm
import json
import math
import re
import requests
import logging
import os

from flask import Flask, Response
from threading import Thread, Lock, Event
from requests.packages.urllib3.exceptions import InsecureRequestWarning
from logging.handlers import RotatingFileHandler
from kafka import KafkaConsumer

requests.packages.urllib3.disable_warnings(InsecureRequestWarning)

APP = Flask(__name__)

logger = logging.getLogger('cce_exporter')


class GlobalConfig(object):
    bootstrap_servers = ["192.168.1.235:9092", "192.168.1.196:9092", "192.168.1.228:9092"]
    topic_name = 'aom-metrics'
    consumer_id = 'comsumer-aom-metrics'
    collect_metrics = ["memUsage", "cpuUsage", "filesystemCapacity", "filesystemAvailable"]


class MetricData(object):
    _cur_metrics_data = dict()
    _lock = Lock()

    @classmethod
    def set(cls, metrics_data):
        with cls._lock:
            cls._cur_metrics_data.update(metrics_data)

    @classmethod
    def get(cls):
        with cls._lock:
            return cls._cur_metrics_data


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
    def __init__(self, cur_metric_dict):
        self.metrics_dict = self._setup_expose_metric(cur_metric_dict)

    @staticmethod
    def _setup_expose_metric(cur_metric_dict):
        metric_dict = dict()
        for tuple_info, _ in cur_metric_dict.items():
            if tuple_info[-1] == "memUsage":
                metric_dict[tuple_info] = Metric('gauge', 'scrape_mem_usage', 'Scrape_memory_useage',
                                                 ("cluster_name", "namespace", "container_name"))
            elif tuple_info[-1] == "cpuUsage":
                metric_dict[tuple_info] = Metric('gauge', 'scrape_cpu_usage', 'Scrape_cpu_useage',
                                                 ("cluster_name", "namespace", "container_name"))
            elif tuple_info[-1] == "filesystemAvailable":
                metric_dict[tuple_info] = Metric('gauge', 'scrape_filesystem_available',
                                                 'Total available capacity of the file system',
                                                 ("cluster_name", "namespace", "container_name"))
            elif tuple_info[-1] == "filesystemCapacity":
                metric_dict[tuple_info] = Metric('gauge', 'scrape_filesystem_capacity',
                                                 'Total capacity of the file system',
                                                 ("cluster_name", "namespace", "container_name"))
            else:
                logger.error("need to adapter")
        return metric_dict

    def set_metric_data(self, metric_dict):
        for tuple_info, value in metric_dict.items():
            self.metrics_dict[tuple_info].clear()
        for tuple_info, value in metric_dict.items():
            self.metrics_dict[tuple_info].set(value, tuple_info[0:-1])


class EipTools(object):
    _lock = Lock()
    _event = Event()

    def __init__(self, *args, **kwargs):
        super(EipTools, self).__init__(*args, **kwargs)

    @classmethod
    def parse_metrics_data(cls, metrics_info):
        ret_dict = dict()
        cluster_name, namespace_name, container_name = str(), str(), str()
        for dimensions_temp in metrics_info["metric"]["dimensions"]:
            if dimensions_temp["name"] == "clusterName":
                cluster_name = dimensions_temp["value"]
            elif dimensions_temp["name"] == "nameSpace":
                namespace_name = dimensions_temp["value"]
            elif dimensions_temp["name"] == "containerName":
                container_name = dimensions_temp["value"]
        if not container_name:
            return ret_dict
        for values_temp in metrics_info["values"]:
            if values_temp["metric_name"] in GlobalConfig.collect_metrics:
                ret_dict[(cluster_name, namespace_name, container_name, values_temp["metric_name"])] = float(values_temp["value"])
        return ret_dict

    @classmethod
    def loop_collect_data(cls):
        consumer = KafkaConsumer(GlobalConfig.topic_name, bootstrap_servers=GlobalConfig.bootstrap_servers,
                                 group_id=GlobalConfig.consumer_id)
        for message in consumer:
            content = message.value.decode("utf-8")
            dict_data = json.loads(content)
            for metrics_info in dict_data["metrics"]:
                if metrics_info["metric"]["namespace"] != "PAAS.CONTAINER":
                    continue
                dimensions_name_list = [dimensions_info["name"] for dimensions_info in
                                        metrics_info["metric"]["dimensions"]]
                if "clusterName" not in dimensions_name_list:
                    continue
                if metrics_info.get("values") is None:
                    continue
                parse_data = cls.parse_metrics_data(metrics_info)
                if parse_data:
                    print("***************************:{}".format(parse_data))
                    MetricData.set(parse_data)

    @classmethod
    def init_logger(cls):
        global logger
        logger.setLevel(level=logging.INFO)
        formatter = logging.Formatter('%(asctime)s %(name)-12s %(levelname)-8s %(message)s')
        size_rotate_file = RotatingFileHandler(filename='cce_exporter.log', maxBytes=5 * 1024 * 1024, backupCount=10)
        size_rotate_file.setFormatter(formatter)
        size_rotate_file.setLevel(logging.INFO)

        console_handler = logging.StreamHandler()
        console_handler.setLevel(level=logging.INFO)
        console_handler.setFormatter(formatter)

        logger.addHandler(size_rotate_file)
        logger.addHandler(console_handler)

    @classmethod
    def init_task(cls):
        logger.info("##################start to collect thread#############")
        th = Thread(target=cls.loop_collect_data, daemon=True)
        th.start()
        # cls.loop_collect_data()

    @classmethod
    def query_data(cls):
        with cls._lock:
            cur_metric_dict = MetricData.get()
            expose_metric = ExposeMetric(cur_metric_dict)
            expose_metric.set_metric_data(cur_metric_dict)
            ret_metric = [m.str_expfmt() for m in expose_metric.metrics_dict.values()]
            return ''.join(ret_metric)


@APP.route("/")
def metrics():
    templates = '''<!DOCTYPE html>
    <html>
        <head><title>CCE Exporter</title></head>
        <body>
            <h1>CCE Exporter</h1>
            <p><a href='/metrics'>Metrics</a></p>
        </body>
    </html>'''
    return templates


@APP.route("/metrics")
def collect_metrics():
    content = EipTools.query_data()
    resp = Response(content)
    resp.headers['Content-Type'] = 'text/plain'
    return resp


def main():
    EipTools.init_logger()
    EipTools.init_task()
    APP.run(port=8080, debug=False)


if __name__ == '__main__':
    main()
