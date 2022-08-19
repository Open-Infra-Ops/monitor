FROM golang:latest as BUILDER
LABEL maintainer="TomNewChao"
# build binary
RUN mkdir -p /go/src/gitee.com/openeuler/k8s_exporter
COPY . /go/src/gitee.com/openeuler/k8s_exporter
RUN cd /go/src/gitee.com/openeuler/k8s_exporter && go mod tidy && CGO_ENABLED=1 go build -v -o ./k8s_exporter main.go

# copy binary config and utils
FROM openeuler/openeuler:21.03
RUN mkdir -p /opt/app/conf/
RUN mkdir -p /opt/app/log/
# COPY ./conf/product_app.conf /opt/app/conf/app.conf
# overwrite config yaml
COPY --from=BUILDER /go/src/gitee.com/openeuler/k8s_exporter/k8s_exporter /opt/app
WORKDIR /opt/app/
ENTRYPOINT ["/opt/app/k8s_exporter > /opt/app/log/k8s_exporter.log"]
