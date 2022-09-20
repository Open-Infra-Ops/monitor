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
	"github.com/Shopify/sarama"
	"github.com/astaxie/beego/config"
	"github.com/astaxie/beego/logs"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"io/ioutil"
	prometheusClient "k8s_exporter/src"
	"net/http"
	"os"
	"strings"
)

// write interface
type writer interface {
	Write(samples model.Samples) error
	Name() string
}

// noOpWriter struct
type noOpWriter struct{}

// read config
func readConfig() (e error, c config.Configer) {
	BConfig, err := config.NewConfig("ini", "conf/app.conf")
	if err != nil {
		fmt.Println("config init error:", err.Error())
		return err, BConfig
	}
	return nil, BConfig
}

// instantiate the client
func buildClients(p *sarama.SyncProducer, c *config.Configer) writer {
	mySqlClient := prometheusClient.NewClient(p, c)
	return mySqlClient
}

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

// write: Prometheus push data to this api, and handler data to kafka
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
		err = writer.Write(samples)
		if err != nil {
			logs.Info("msg", "Error sending samples to remote storage", "err", err, "storage", writer.Name(), "num_samples", len(samples))
		}
	})
}

// index: test the k8s-exporter service is ok.
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

func main() {
	// 1.first to read config
	err, serviceConfig := readConfig()
	if err != nil {
		fmt.Println("main:Get config err:", err)
		return
	}
	// 2.init log
	prometheusClient.LogInit(serviceConfig)
	// 3.init kafka
	saramaConfig := sarama.NewConfig()
	saramaConfig.Producer.RequiredAcks = sarama.WaitForAll
	saramaConfig.Producer.Partitioner = sarama.NewRandomPartitioner
	saramaConfig.Producer.Return.Successes = true
	brokers := serviceConfig.String("kafka::brokers")
	brokersSplit := strings.Split(brokers, ",")
	kafkaClient, err := sarama.NewSyncProducer(brokersSplit, saramaConfig)
	if err != nil {
		logs.Error("main: Producer closed, err:", err)
		return
	}
	defer func(kafkaClient sarama.SyncProducer) {
		err := kafkaClient.Close()
		if err != nil {
			logs.Error("main: close kafkaClient err:", err)
		}
	}(kafkaClient)
	// 4.Start the coroutine to start the collection
	go prometheusClient.StartCpuRateCollect(kafkaClient, serviceConfig)
	go prometheusClient.StartMemRateCollect(kafkaClient, serviceConfig)
	go prometheusClient.StartFsRateCollect(kafkaClient, serviceConfig)
	// 5.start to web
	serverAddr := serviceConfig.String("web::web-listen-address")
	writer := buildClients(&kafkaClient, &serviceConfig)
	http.Handle("/write", write(writer))
	http.Handle("/", index())
	logs.Info("msg", "Starting up...")
	logs.Info("msg", "Listening", "addr", serverAddr)
	err = http.ListenAndServe(serverAddr, nil)
	if err != nil {
		logs.Error("msg", "Listen failure", "err", err)
		os.Exit(1)
	}
}
