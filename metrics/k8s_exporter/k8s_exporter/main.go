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
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"os"
	"sync/atomic"
	"time"

	prometheusClient "k8s_exporter/src"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/jamiealquiza/envy"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
)

type config struct {
	listenAddr    string
	telemetryPath string
	logLevel      string
}

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

func parseFlags() *config {
	cfg := &config{}
	flag.StringVar(&cfg.listenAddr, "web-listen-address", ":9201", "Address to listen on for web endpoints.")
	flag.StringVar(&cfg.telemetryPath, "web-telemetry-path", "/metrics", "Address to listen on for web endpoints.")
	flag.StringVar(&cfg.logLevel, "log-level", "debug", "The log level to use [ \"error\", \"warn\", \"info\", \"debug\" ].")
	envy.Parse("TS_PROM")
	flag.Parse()
	return cfg
}

func main() {
	cfg := parseFlags()
	prometheusClient.Init(cfg.logLevel)
	writer, reader := buildClients(cfg)
	metricsPath := cfg.telemetryPath
	http.Handle(metricsPath, timeHandler(metricsPath[1:], read(reader)))
	http.Handle("/write", timeHandler("write", write(writer)))
	http.Handle("/health", health(reader))
	http.Handle("/", index(cfg))
	prometheusClient.Info("msg", "Starting up...")
	prometheusClient.Info("msg", "Listening", "addr", cfg.listenAddr)
	err := http.ListenAndServe(cfg.listenAddr, nil)
	if err != nil {
		prometheusClient.Error("msg", "Listen failure", "err", err)
		os.Exit(1)
	}
}

// instantiate the client
func buildClients(cfg *config) (writer, reader) {
	mySqlClient := prometheusClient.NewClient(cfg.telemetryPath)
	return mySqlClient, mySqlClient
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
	prometheusClient.Debug("msg", "Noop writer", "num_samples", len(samples))
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
			prometheusClient.Error("msg", "Read error", "err", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		reqBuf, err := snappy.Decode(nil, compressed)
		if err != nil {
			prometheusClient.Error("msg", "Decode error", "err", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req prompb.WriteRequest
		if err := proto.Unmarshal(reqBuf, &req); err != nil {
			prometheusClient.Error("msg", "Unmarshal error", "err", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		samples := protoToSamples(&req)
		receivedSamples.Add(float64(len(samples)))

		err = sendSamples(writer, samples)
		if err != nil {
			prometheusClient.Warn("msg", "Error sending samples to remote storage", "err", err, "storage", writer.Name(), "num_samples", len(samples))
		}

	})
}

func read(reader reader) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req prompb.ReadRequest
		var resp *prompb.ReadResponse
		var err error
		resp, err = reader.Read(&req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/x-protobuf")
		w.Header().Set("Content-Encoding", "snappy")

	})
}

func health(reader reader) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := reader.HealthCheck()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		msg, _ := json.Marshal(JsonResult{Code: 200, Msg: "ok"})
		w.Header().Set("content-type", "text/json")
		_, err = w.Write(msg)
		if err != nil {
			prometheusClient.Error("msg", "health api", "err", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}

func index(cfg *config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		indexTemplates := `<!DOCTYPE html>
		<html>
		<head><title>K8S Exporter</title></head>
		<body>
		<h1>K8S Exporter</h1>
		<p><a href=` + cfg.telemetryPath + `>Metrics</a></p>
		</body>
		</html>`
		_, err := fmt.Fprintf(w, indexTemplates)
		if err != nil {
			prometheusClient.Error("msg", "index api", "err", err.Error())
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
