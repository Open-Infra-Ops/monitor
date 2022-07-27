// Copyright 2015 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !nocpu
// +build !nocpu

package collector

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/procfs"
	"time"
)

type cpuCollector struct {
	fs                 procfs.FS
	cpu                *prometheus.Desc
	cpuGuest           *prometheus.Desc
	cpuCoreThrottle    *prometheus.Desc
	cpuPackageThrottle *prometheus.Desc
}

func init() {
	registerCollector("cpu", defaultEnabled, NewCPUCollector)
}

// NewCPUCollector returns a new Collector exposing kernel/system statistics.
func NewCPUCollector() (Collector, error) {
	fs, err := procfs.NewFS(*procPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open procfs: %v", err)
	}
	return &cpuCollector{
		fs:  fs,
		cpu: nodeCPUSecondsDesc,
		cpuGuest: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, cpuCollectorSubsystem, "guest_seconds_total"),
			"Seconds the cpus spent in guests (VMs) for each mode.",
			[]string{"cpu", "mode"}, nil,
		),
		cpuCoreThrottle: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, cpuCollectorSubsystem, "core_throttles_total"),
			"Number of times this cpu core has been throttled.",
			[]string{"package", "core"}, nil,
		),
		cpuPackageThrottle: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, cpuCollectorSubsystem, "package_throttles_total"),
			"Number of times this cpu package has been throttled.",
			[]string{"package"}, nil,
		),
	}, nil
}

// Update implements Collector and exposes cpu related metrics from /proc/stat and /sys/.../cpu/.
func (c *cpuCollector) Update(ch chan<- prometheus.Metric) error {
	if err := c.updateStat(ch); err != nil {
		return err
	}
	return nil
}

// updateStat reads /proc/stat through procfs and exports cpu related metrics.
func (c *cpuCollector) updateStat(ch chan<- prometheus.Metric) error {
	stats, err := c.fs.NewStat()
	if err != nil {
		return err
	}
	cpuStat := stats.CPUTotal
	use_before := cpuStat.User + cpuStat.Iowait + cpuStat.IRQ + cpuStat.System + cpuStat.SoftIRQ + cpuStat.Nice
	total_before := use_before + cpuStat.Idle

	time.Sleep(1 * time.Second)

	stats, err = c.fs.NewStat()
	if err != nil {
		return err
	}
	cpuStat = stats.CPUTotal
	use_after := cpuStat.User + cpuStat.Iowait + cpuStat.IRQ + cpuStat.System + cpuStat.SoftIRQ + cpuStat.Nice
	total_after := use_after + cpuStat.Idle

	use_ratio := (use_after - use_before) / (total_after - total_before)
	use_ratio = use_ratio * 100
	if use_ratio < 0.0001 {
		use_ratio = 0
	}

	if use_ratio > 100 {
		use_ratio = 100
	}

	ch <- prometheus.MustNewConstMetric(c.cpu, prometheus.CounterValue, use_ratio, "0", "use")

	return nil
}
