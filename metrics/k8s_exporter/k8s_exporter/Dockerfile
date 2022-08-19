FROM golang:latest as BUILDER
LABEL maintainer="TomNewChao"
RUN mkdir -p /go/src/gitee.com/openeuler/k8s_exporter
COPY . /go/src/gitee.com/openeuler/k8s_exporter
RUN cd /go/src/gitee.com/openeuler/k8s_exporter && go mod tidy && CGO_ENABLED=1 go build -v -o ./k8s_exporter main.go

FROM openeuler/openeuler:21.03
RUN mkdir -p /opt/app/conf/
RUN mkdir -p /opt/app/log/
RUN touch /opt/app/log/k8s_exporter.log
RUN COPY ./conf/app.conf /opt/app/conf/app.conf
COPY --from=BUILDER /go/src/gitee.com/openeuler/k8s_exporter/k8s_exporter /opt/app
WORKDIR /opt/app/
ENTRYPOINT ["/opt/app/k8s_exporter"]
