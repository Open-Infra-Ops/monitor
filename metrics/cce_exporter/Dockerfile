FROM centos:7

MAINTAINER TomNewChao<353712216@qq.com>
ENV PYTHONUNBUFFERED 1

RUN mkdir /opt/open_infra
WORKDIR /opt/open_infra
COPY . /opt/open_infra/

RUN yum install -y gcc-c++
RUN yum install -y python3
RUN yum install -y python3-pip
RUN yum install -y python3-devel
RUN yum install -y java-1.8.0-openjdk*
RUN yum install -y nmap
RUN chmod 777 /opt/open_infra/docker-entrypoint.sh
RUN pip3 install -r /opt/open_infra/requirements.txt


ENV LANG=en_US.UTF-8
ENV PYTHONPATH=/opt/open_infra
CMD ["/bin/bash", "/opt/open_infra/docker-entrypoint.sh"]
ENTRYPOINT ["uwsgi", "--ini", "/opt/open_infra/uwsgi.ini"]
EXPOSE 80