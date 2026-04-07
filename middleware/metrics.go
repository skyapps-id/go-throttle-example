package middleware

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

var (
	ThrottleRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "throttle_requests_total",
			Help: "Total number of throttle requests.",
		},
		[]string{"type", "method", "endpoint", "result"},
	)

	ThrottleQueueLength = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "throttle_queue_length",
			Help: "Current number of requests in throttle queue.",
		},
		[]string{"type", "method", "endpoint"},
	)

	ThrottleRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "throttle_request_duration_seconds",
			Help:    "Duration of throttle requests (only allowed and queued).",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"type", "method", "endpoint", "result"},
	)

	RequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests. Use rate(http_requests_total[1m]) for RPS.",
		},
		[]string{"method", "endpoint", "status_code"},
	)

	RequestsDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "endpoint", "status_code"},
	)
)

func InitMetrics(registry *prometheus.Registry) {
	registry.MustRegister(collectors.NewGoCollector())
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	registry.MustRegister(ThrottleRequestsTotal)
	registry.MustRegister(ThrottleQueueLength)
	registry.MustRegister(ThrottleRequestDuration)
	registry.MustRegister(RequestsTotal)
	registry.MustRegister(RequestsDuration)
}
