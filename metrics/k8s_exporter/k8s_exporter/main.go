// Copyright 2017 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// The main package for the Prometheus server executable.
package main

import (
	"fmt"
	"github.com/astaxie/beego/config"
	"github.com/astaxie/beego/logs"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"io/ioutil"
	prometheusClient "k8s_exporter/src"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

type JsonResult struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

var (
	receivedSamples = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "received_samples_total",
			Help: "Total number of received samples.",
		},
	)
	sentSamples = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sent_samples_total",
			Help: "Total number of processed samples sent to remote storage.",
		},
		[]string{"remote"},
	)
	failedSamples = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "failed_samples_total",
			Help: "Total number of processed samples which failed on send to remote storage.",
		},
		[]string{"remote"},
	)
	sentBatchDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sent_batch_duration_seconds",
			Help:    "Duration of sample batch send calls to the remote storage.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"remote"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_ms",
			Help:    "Duration of HTTP request in milliseconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path"},
	)
	lastRequestUnixNano = time.Now().UnixNano()
)

func init() {
	prometheus.MustRegister(receivedSamples)
	prometheus.MustRegister(sentSamples)
	prometheus.MustRegister(failedSamples)
	prometheus.MustRegister(sentBatchDuration)
	prometheus.MustRegister(httpRequestDuration)
}

// read config
func readConfig() (e error, c config.Configer) {
	BConfig, err := config.NewConfig("ini", "conf/app.conf")
	if err != nil {
		fmt.Println("config init error:", err.Error())
		return err, BConfig
	}
	return nil, BConfig
}

func main() {
	// 1.first to read config
	err, serviceConfig := readConfig()
	if err != nil {
		fmt.Println("main:get config error")
		return
	}
	// 2.init log
	prometheusClient.LogInit(serviceConfig)
	// 3.init kafka
	brokers := serviceConfig.String("kafka::brokers")
	kafkaConfig := &kafka.ConfigMap{
		"bootstrap.servers": brokers,
	}
	producer, err := kafka.NewProducer(kafkaConfig)
	if err != nil {
		logs.Info("producer error, err: %v", err)
		return
	}
	// 4.start to web
	serverAddr := serviceConfig.String("web::web-listen-address")
	writer := buildClients(producer, serviceConfig)
	http.Handle("/write", timeHandler("write", write(writer)))
	http.Handle("/", index())
	logs.Info("msg", "Starting up...")
	logs.Info("msg", "Listening", "addr", serverAddr)
	err = http.ListenAndServe(serverAddr, nil)
	if err != nil {
		logs.Error("msg", "Listen failure", "err", err)
		os.Exit(1)
	}
	// 5.over kafka
	// Wait for message deliveries before shutting down
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sigterm:
		logs.Info("terminating: via signal")
	}
	producer.Flush(15 * 1000)
	producer.Close()
}

// instantiate the client
func buildClients(p *kafka.Producer, c config.Configer) writer {
	mySqlClient := prometheusClient.NewClient(p, c)
	return mySqlClient
}

type reader interface {
	Read(req *prompb.ReadRequest) (*prompb.ReadResponse, error)
	Name() string
	HealthCheck() error
}

type writer interface {
	Write(samples model.Samples) error
	Name() string
}

type noOpWriter struct{}

func (no *noOpWriter) Write(samples model.Samples) error {
	logs.Debug("msg", "Noop writer", "num_samples", len(samples))
	return nil
}

func (no *noOpWriter) Name() string {
	return "noopWriter"
}

func protoToSamples(req *prompb.WriteRequest) model.Samples {
	var samples model.Samples
	for _, ts := range req.Timeseries {
		metric := make(model.Metric, len(ts.Labels))
		for _, l := range ts.Labels {
			metric[model.LabelName(l.Name)] = model.LabelValue(l.Value)
		}

		for _, s := range ts.Samples {
			samples = append(samples, &model.Sample{
				Metric:    metric,
				Value:     model.SampleValue(s.Value),
				Timestamp: model.Time(s.Timestamp),
			})
		}
	}
	return samples
}

func sendSamples(w writer, samples model.Samples) error {
	atomic.StoreInt64(&lastRequestUnixNano, time.Now().UnixNano())
	begin := time.Now()
	var err error
	err = w.Write(samples)
	duration := time.Since(begin).Seconds()
	if err != nil {
		failedSamples.WithLabelValues(w.Name()).Add(float64(len(samples)))
		return err
	}
	sentSamples.WithLabelValues(w.Name()).Add(float64(len(samples)))
	sentBatchDuration.WithLabelValues(w.Name()).Observe(duration)
	return nil
}

func write(writer writer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		compressed, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logs.Error("msg", "Read error", "err", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		reqBuf, err := snappy.Decode(nil, compressed)
		if err != nil {
			logs.Error("msg", "Decode error", "err", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req prompb.WriteRequest
		if err := proto.Unmarshal(reqBuf, &req); err != nil {
			logs.Error("msg", "Unmarshal error", "err", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		samples := protoToSamples(&req)
		receivedSamples.Add(float64(len(samples)))

		err = sendSamples(writer, samples)
		if err != nil {
			logs.Info("msg", "Error sending samples to remote storage", "err", err, "storage", writer.Name(), "num_samples", len(samples))
		}
	})
}

func index() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		indexTemplates := `<!DOCTYPE html>
		<html>
		<head><title>K8S Exporter</title></head>
		<body>
		<h1>K8S Exporter</h1>
		<p>Health</p>
		</body>
		</html>`
		_, err := fmt.Fprintf(w, indexTemplates)
		if err != nil {
			logs.Error("msg", "index api", "err", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}

// timeHandler uses Prometheus histogram to track request time
func timeHandler(path string, handler http.Handler) http.Handler {
	f := func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		handler.ServeHTTP(w, r)
		elapsedMs := time.Since(start).Nanoseconds() / int64(time.Millisecond)
		httpRequestDuration.WithLabelValues(path).Observe(float64(elapsedMs))
	}
	return http.HandlerFunc(f)
}
