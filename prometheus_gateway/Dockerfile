FROM openeuler/openeuler:22.03
RUN python3 -m ensurepip --default-pip && python3 -m pip install --upgrade pip
RUN pip config set global.index-url https://pypi.tuna.tsinghua.edu.cn/simple
RUN pip config set global.trusted-host pypi.tuna.tsinghua.edu.cn
RUN pip config set global.timeout 120

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