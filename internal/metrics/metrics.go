/*
Copyright 2025 GPU_Orchestrator contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package metrics provides Prometheus metrics for the GPU_Orchestrator controller.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Metrics holds all Prometheus metrics for the GPU_Orchestrator controller.
type Metrics struct {
	// GPUWorkloadScheduledTotal counts the number of successfully scheduled GPUWorkloads
	GPUWorkloadScheduledTotal prometheus.CounterVec

	// GPUWorkloadFailedTotal counts the number of failed scheduling attempts
	GPUWorkloadFailedTotal prometheus.CounterVec

	// GPUWorkloadRetriesTotal counts the total number of retry attempts
	GPUWorkloadRetriesTotal prometheus.Counter

	// GPUWorkloadReconcileDurationSeconds measures the duration of reconciliation
	GPUWorkloadReconcileDurationSeconds prometheus.HistogramVec
}

var (
	// Global metrics instance
	metricsInstance *Metrics

	gpuWorkloadScheduledTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "warp_gpuworkload_scheduled_total",
			Help: "Total number of GPUWorkloads successfully scheduled",
		},
		[]string{"strategy"},
	)

	gpuWorkloadFailedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "warp_gpuworkload_failed_total",
			Help: "Total number of GPUWorkload scheduling failures",
		},
		[]string{"reason"},
	)

	gpuWorkloadRetriesTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "warp_gpuworkload_retries_total",
			Help: "Total number of GPUWorkload retry attempts",
		},
	)

	gpuWorkloadReconcileDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "warp_gpuworkload_reconcile_duration_seconds",
			Help:    "Duration of GPUWorkload reconciliation in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"result"},
	)
)

func init() {
	// Register metrics with the controller-runtime metrics registry
	metrics.Registry.MustRegister(
		gpuWorkloadScheduledTotal,
		gpuWorkloadFailedTotal,
		gpuWorkloadRetriesTotal,
		gpuWorkloadReconcileDurationSeconds,
	)

	metricsInstance = &Metrics{
		GPUWorkloadScheduledTotal:           *gpuWorkloadScheduledTotal,
		GPUWorkloadFailedTotal:              *gpuWorkloadFailedTotal,
		GPUWorkloadRetriesTotal:             gpuWorkloadRetriesTotal,
		GPUWorkloadReconcileDurationSeconds: *gpuWorkloadReconcileDurationSeconds,
	}
}

// GetMetrics returns the global metrics instance.
func GetMetrics() *Metrics {
	return metricsInstance
}

// RecordSchedulingSuccess increments the scheduled counter for a given strategy.
func (m *Metrics) RecordSchedulingSuccess(strategy string) {
	gpuWorkloadScheduledTotal.WithLabelValues(strategy).Inc()
}

// RecordSchedulingFailure increments the failed counter for a given reason.
func (m *Metrics) RecordSchedulingFailure(reason string) {
	gpuWorkloadFailedTotal.WithLabelValues(reason).Inc()
}

// RecordRetry increments the retry counter.
func (m *Metrics) RecordRetry() {
	gpuWorkloadRetriesTotal.Inc()
}

// RecordReconcileDuration records the duration of a reconciliation attempt.
// result should be "success" or "error".
func (m *Metrics) RecordReconcileDuration(duration float64, result string) {
	gpuWorkloadReconcileDurationSeconds.WithLabelValues(result).Observe(duration)
}
