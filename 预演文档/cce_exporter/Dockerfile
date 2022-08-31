FROM centos:7

MAINTAINER TomNewChao<tom_toworld@163.com>
ENV PYTHONUNBUFFERED 1

RUN mkdir /opt/cce_exporter
WORKDIR /opt/cce_exporter
COPY . /opt/cce_exporter/
RUN yum install -y python3
RUN yum install -y python3-pip
RUN yum install -y python3-devel
RUN pip3 install -r /opt/cce_exporter/requirements.txt
ENV LANG=en_US.UTF-8
ENV PYTHONPATH=/opt/cce_exporter
ENTRYPOINT ["python3", "/opt/cce_exporter/cce_exporter.py"]
EXPOSE 8000