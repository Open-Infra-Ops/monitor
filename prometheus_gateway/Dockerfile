FROM openeuler/openeuler:21.03

MAINTAINER TomNewChao<tom_toworld@163.com>
ENV PYTHONUNBUFFERED 1

RUN mkdir /opt/prometheus_gateway
WORKDIR /opt/prometheus_gateway
COPY . /opt/prometheus_gateway/
RUN yum install -y python3
RUN yum install -y python3-pip
RUN yum install -y python3-devel
RUN pip3 install -r /opt/prometheus_gateway/requirements.txt
ENV LANG=en_US.UTF-8
ENV PYTHONPATH=/opt/prometheus_gateway
ENTRYPOINT ["python3", "/opt/prometheus_gateway/prometheus_gateway.py"]
EXPOSE 8000