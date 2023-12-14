FROM openeuler/openeuler:22.03
RUN python3 -m ensurepip --default-pip && python3 -m pip install --upgrade pip
RUN pip config set global.index-url https://pypi.tuna.tsinghua.edu.cn/simple
RUN pip config set global.trusted-host pypi.tuna.tsinghua.edu.cn
RUN pip config set global.timeout 120

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