package middleware

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

var (
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

	RequestsInFlight = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Number of HTTP requests currently being processed.",
		},
		[]string{"method", "endpoint"},
	)

	ResponseSizeBytes = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "Size of HTTP response body in bytes.",
			Buckets: prometheus.ExponentialBuckets(100, 10, 7),
		},
		[]string{"method", "endpoint"},
	)

	HTTPErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_errors_total",
			Help: "Total number of HTTP 5xx errors.",
		},
		[]string{"method", "endpoint", "error_type"},
	)

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

	ThrottleWindowUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "throttle_window_usage_ratio",
			Help: "Ratio of current window usage (current_count / rate_limit).",
		},
		[]string{"type", "method", "endpoint"},
	)

	ThrottleWaitTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "throttle_wait_time_seconds",
			Help:    "Time spent waiting in throttle queue before being processed.",
			Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"type", "method", "endpoint"},
	)

	ThrottleRedisErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "throttle_redis_errors_total",
			Help: "Total number of Redis errors in throttle middleware.",
		},
		[]string{"operation"},
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
	registry.MustRegister(RequestsInFlight)
	registry.MustRegister(ResponseSizeBytes)
	registry.MustRegister(HTTPErrorsTotal)
	registry.MustRegister(ThrottleWindowUsage)
	registry.MustRegister(ThrottleWaitTime)
	registry.MustRegister(ThrottleRedisErrors)
}
