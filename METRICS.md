# Go-Throttle Metrics

## HTTP General

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `http_requests_total` | Counter | `method`, `endpoint`, `status_code` | Total HTTP requests. Use `rate(http_requests_total[1m])` for RPS. |
| `http_request_duration_seconds` | Histogram | `method`, `endpoint`, `status_code` | Duration of HTTP requests. Buckets: `0.005s` - `10s`. |
| `http_requests_in_flight` | Gauge | `method`, `endpoint` | Number of HTTP requests currently being processed. |
| `http_response_size_bytes` | Histogram | `method`, `endpoint` | Size of HTTP response body in bytes. Buckets: `100B` - `1GB`. |
| `http_errors_total` | Counter | `method`, `endpoint`, `error_type` | Total number of HTTP 5xx errors. |

## Throttle

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `throttle_requests_total` | Counter | `type`, `method`, `endpoint`, `result` | Total throttle requests. `result`: `allowed`, `rejected`, `queued`, `timeout`. `type`: `inmem`, `redis`. |
| `throttle_queue_length` | Gauge | `type`, `method`, `endpoint` | Current number of requests in throttle queue. |
| `throttle_request_duration_seconds` | Histogram | `type`, `method`, `endpoint`, `result` | Duration of throttle requests (only allowed & queued). Buckets: `0.005s` - `10s`. |
| `throttle_window_usage_ratio` | Gauge | `type`, `method`, `endpoint` | Ratio of current window usage (`count / rate_limit`). |
| `throttle_wait_time_seconds` | Histogram | `type`, `method`, `endpoint` | Time spent waiting in throttle queue before being processed. Buckets: `0.01s` - `10s`. |

## Redis

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `throttle_redis_errors_total` | Counter | `operation` | Total Redis errors in throttle middleware. `operation`: `eval_allow`, `eval_dequeue`. |

## Runtime

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `go_*` | Various | — | Go runtime metrics (GC, goroutines, memory, etc.). |
| `process_*` | Various | — | Process metrics (CPU, memory, file descriptors, etc.). |

## Example PromQL Queries

```promql
# RPS
rate(http_requests_total[1m])

# P95 latency
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))

# Active requests
http_requests_in_flight

# Error rate (5xx)
rate(http_errors_total[5m])

# Throttle rejection rate
rate(throttle_requests_total{result="rejected"}[5m])

# Average wait time in queue
rate(throttle_wait_time_seconds_sum[5m]) / rate(throttle_wait_time_seconds_count[5m])

# Throttle window usage
throttle_window_usage_ratio

# Redis error rate
rate(throttle_redis_errors_total[5m])
```
