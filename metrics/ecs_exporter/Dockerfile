FROM openeuler/openeuler:22.03

MAINTAINER TomNewChao<tom_toworld@163.com>
ENV PYTHONUNBUFFERED 1

RUN mkdir /opt/ecs_exporter
WORKDIR /opt/ecs_exporter
COPY . /opt/ecs_exporter/
RUN yum install -y python3
RUN yum install -y python3-pip
RUN yum install -y python3-devel
RUN pip3 install -r /opt/ecs_exporter/requirements.txt
ENV LANG=en_US.UTF-8
ENV PYTHONPATH=/opt/ecs_exporter
ENTRYPOINT ["python3", "/opt/ecs_exporter/ecs_exporter.py"]
EXPOSE 8000