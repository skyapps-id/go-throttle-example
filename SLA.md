# Service Level Agreement (SLA) — Go-Throttle

> **Version**: 1.0  
> **Last Updated**: April 2026  
> **Owner**: Platform Engineering  
> **Status**: Active

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Definitions & Scope](#2-definitions--scope)
3. [Service Description](#3-service-description)
4. [SLI & SLO Targets](#4-sli--slo-targets)
5. [Per-Endpoint SLO](#5-per-endpoint-slo)
6. [Throttle-Specific SLO](#6-throttle-specific-slo)
7. [Infrastructure SLO](#7-infrastructure-slo)
8. [Error Budget Policy](#8-error-budget-policy)
9. [Alerting Framework](#9-alerting-framework)
10. [Grafana Dashboard Panels](#10-grafana-dashboard-panels)
11. [Incident Management](#11-incident-management)
12. [On-Call Procedures](#12-on-call-procedures)
13. [Runbook](#13-runbook)
14. [Capacity Planning](#14-capacity-planning)
15. [Load Testing & Validation](#15-load-testing--validation)
16. [Change Management](#16-change-management)
17. [Reporting & Review Cadence](#17-reporting--review-cadence)
18. [Compliance & Audit](#18-compliance--audit)
19. [Appendix A: Full PromQL Reference](#19-appendix-a-full-promql-reference)
20. [Appendix B: Metrics Catalog](#20-appendix-b-metrics-catalog)

---

## 1. Executive Summary

Go-Throttle menyediakan layanan rate limiting dengan dua strategi: **in-memory** dan **Redis-based**. Layanan ini dirancang untuk melindungi downstream services dari traffic spike, mencegah OOM, dan menjamin fair resource allocation.

### Key SLO Commitments

| Commitment | Target |
|-----------|--------|
| Availability | 99.9% per bulan |
| p99 Latency | < 1000ms |
| Error Rate | < 0.1% |
| Throughput | > 100 RPS |
| Zero Data Loss | Queue must not silently drop requests |

### Business Impact

Berdasarkan load test yang dilakukan (lihat `LOADTEST.md`):

| Scenario | Success Rate | OOM | Server Crash |
|----------|-------------|-----|-------------|
| Tanpa Throttle | 15% | Ya | Ya |
| Dengan Throttle | 75% + 25% graceful rejection | Tidak | Tidak |

---

## 2. Definitions & Scope

### 2.1 Definitions

| Term | Definition |
|------|-----------|
| **SLI** (Service Level Indicator) | Metric kuantitatif yang mengukur performa layanan |
| **SLO** (Service Level Objective) | Target nilai SLI yang harus dicapai |
| **SLA** (Service Level Agreement) | Komitmen formal terhadap SLO, termasuk konsekuensi jika tidak terpenuhi |
| **Error Budget** | Persentase toleransi error di luar SLO target |
| **Burn Rate** | Kecepatan konsumsi error budget per unit waktu |
| **MTTD** (Mean Time to Detect) | Rata-rata waktu dari terjadinya insiden hingga terdeteksi sistem |
| **MTTR** (Mean Time to Resolve) | Rata-rata waktu dari deteksi hingga insiden terselesaikan |

### 2.2 Scope

| Item | Included | Excluded |
|------|----------|----------|
| `/no-throttle` endpoint | Monitoring only | SLO guarantee |
| `/throttle` endpoint (in-memory) | Full SLO | N/A |
| `/throttle-with-redis` endpoint (redis) | Full SLO | N/A |
| `/metrics` endpoint | Best effort | SLO guarantee |
| Redis infrastructure | Monitoring | SLO (separate SLA) |
| Network / CDN | Monitoring | SLO (separate SLA) |

### 2.3 Measurement Windows

| Window | Use Case |
|--------|----------|
| 5 minutes | Real-time alerting, burn rate calculation |
| 1 hour | Short-term trend analysis |
| 24 hours | Daily reporting |
| 30 days | SLO compliance calculation, error budget |
| 90 days | Quarterly review, capacity planning |

---

## 3. Service Description

### 3.1 Architecture

```
Client Request
       │
       ▼
┌──────────────┐
│  Echo Server  │
│  (:8000)      │
├──────────────┤
│ HTTPMetrics   │ ← Global middleware (total, duration, in-flight, size, errors)
│ Middleware    │
├──────────────┤
│ Throttle      │ ← Per-route (allowed/rejected/queued/timeout)
│ Middleware    │
├──────────────┤
│ Handler       │
└──────────────┘
       │
       ▼
┌──────────────┐
│  Redis       │ ← Optional (only /throttle-with-redis)
│  (rate limit  │
│   store)      │
└──────────────┘
```

### 3.2 Rate Limiting Flow

```
Request → Check Window → Slot Available?
                              │
                 ┌────────────┴────────────┐
                 │ Yes                     │ No
                 ▼                         ▼
            Allow Request            Queue Available?
            (200 OK)                     │
                              ┌──────────┴──────────┐
                              │ Yes                  │ No
                              ▼                      ▼
                         Add to Queue           Reject (503)
                         Wait & Retry
                              │
                         Slot Free?
                              │
                    ┌─────────┴─────────┐
                    │ Yes               │ No (timeout)
                    ▼                    ▼
               Process (200)         Timeout (408)
```

### 3.3 Degradation Strategy

| Level | Condition | Action |
|-------|-----------|--------|
| Healthy | All SLO met | Normal operation |
| Warning | p95 > SLO, window usage > 60% | Alert on-call, prepare scaling |
| Degraded | p99 > 2x SLO, window usage > 80% | Reject queue overflow (503), shed load |
| Critical | Error rate > 5%, error budget exhausted | Circuit breaker, fail open, escalation |

---

## 4. SLI & SLO Targets

### 4.1 Primary SLO

| # | SLI | SLO Target | Metric | PromQL |
|---|-----|-----------|--------|--------|
| 1 | **Availability** | >= 99.9% | `http_requests_total`, `http_errors_total` | `1 - (rate(http_errors_total{status_code=~"5.."}[5m]) / rate(http_requests_total[5m]))` |
| 2 | **Latency (p50)** | < 200ms | `http_request_duration_seconds` | `histogram_quantile(0.50, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))` |
| 3 | **Latency (p95)** | < 500ms | `http_request_duration_seconds` | `histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))` |
| 4 | **Latency (p99)** | < 1000ms | `http_request_duration_seconds` | `histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))` |
| 5 | **Error Rate** | < 0.1% | `http_errors_total` | `rate(http_errors_total[5m]) / rate(http_requests_total[5m])` |
| 6 | **Throughput** | >= 100 RPS | `http_requests_total` | `sum(rate(http_requests_total[1m]))` |
| 7 | **Concurrent Requests** | < 1000 | `http_requests_in_flight` | `sum(http_requests_in_flight)` |

### 4.2 Composite SLO

SLO fulfillment dihitung berdasarkan **weighted composite score**:

```
Composite Score = (w1 * Availability) + (w2 * Latency) + (w3 * ErrorRate) + (w4 * Throughput)

Weight:
  Availability   = 40%
  Latency        = 30%
  Error Rate     = 20%
  Throughput     = 10%
```

| Status | Composite Score |
|--------|----------------|
| Healthy | >= 95% |
| Warning | 80% - 94% |
| Degraded | 50% - 79% |
| Critical | < 50% |

---

## 5. Per-Endpoint SLO

### 5.1 Latency Targets

| Endpoint | p50 | p95 | p99 | Max |
|----------|-----|-----|-----|-----|
| `/no-throttle` | < 150ms | < 300ms | < 500ms | < 1000ms |
| `/throttle` (inmem) | < 200ms | < 1000ms | < 3000ms | < 5000ms |
| `/throttle-with-redis` (redis) | < 200ms | < 1000ms | < 3000ms | < 5000ms |

### 5.2 Availability Targets

| Endpoint | Availability | Error Rate | Rejection Rate | Timeout Rate |
|----------|-------------|------------|----------------|-------------|
| `/no-throttle` | 99.9% | < 0.1% | N/A | N/A |
| `/throttle` | 99.0% | < 0.1% | < 5% | < 1% |
| `/throttle-with-redis` | 99.0% | < 0.1% | < 5% | < 1% |

### 5.3 Per-Endpoint PromQL

```promql
# Latency per endpoint
histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket{endpoint="/throttle"}[5m])) by (le))

# Error rate per endpoint
rate(http_errors_total{endpoint="/throttle-with-redis"}[5m]) / rate(http_requests_total{endpoint="/throttle-with-redis"}[5m])

# Rejection rate (throttle-specific)
rate(throttle_requests_total{result="rejected", endpoint="/throttle"}[5m])
  / rate(throttle_requests_total{endpoint="/throttle"}[5m])
```

---

## 6. Throttle-Specific SLO

### 6.1 Queue Performance

| SLI | Target | Metric | PromQL |
|-----|--------|--------|--------|
| Queue Wait Time (p50) | < 500ms | `throttle_wait_time_seconds` | `histogram_quantile(0.50, sum(rate(throttle_wait_time_seconds_bucket[5m])) by (le))` |
| Queue Wait Time (p95) | < 2000ms | `throttle_wait_time_seconds` | `histogram_quantile(0.95, sum(rate(throttle_wait_time_seconds_bucket[5m])) by (le))` |
| Queue Wait Time (p99) | < 5000ms | `throttle_wait_time_seconds` | `histogram_quantile(0.99, sum(rate(throttle_wait_time_seconds_bucket[5m])) by (le))` |

### 6.2 Resource Utilization

| SLI | Target | Metric | PromQL |
|-----|--------|--------|--------|
| Window Saturation (normal) | < 60% | `throttle_window_usage_ratio` | `throttle_window_usage_ratio` |
| Window Saturation (peak) | < 80% | `throttle_window_usage_ratio` | `max_over_time(throttle_window_usage_ratio[1h])` |
| Max Queue Length | < 80% capacity | `throttle_queue_length` | `throttle_queue_length` |

### 6.3 Traffic Distribution

| SLI | Target | Metric | PromQL |
|-----|--------|--------|--------|
| Allowed Rate | > 70% | `throttle_requests_total` | `rate(throttle_requests_total{result="allowed"}[5m]) / rate(throttle_requests_total[5m])` |
| Rejection Rate | < 20% | `throttle_requests_total` | `rate(throttle_requests_total{result="rejected"}[5m]) / rate(throttle_requests_total[5m])` |
| Queued Rate | < 10% | `throttle_requests_total` | `rate(throttle_requests_total{result="queued"}[5m]) / rate(throttle_requests_total[5m])` |
| Timeout Rate | < 1% | `throttle_requests_total` | `rate(throttle_requests_total{result="timeout"}[5m]) / rate(throttle_requests_total[5m])` |

---

## 7. Infrastructure SLO

### 7.1 Runtime Metrics

| SLI | Target | Metric | PromQL |
|-----|--------|--------|--------|
| Goroutine Count | < 10000 | `go_goroutines` | `go_goroutines` |
| GC Pause (p99) | < 1ms | `go_gc_duration_seconds` | `histogram_quantile(0.99, rate(go_gc_duration_seconds_bucket[5m]))` |
| Heap Allocation | < 512MB | `go_memstats_heap_alloc_bytes` | `go_memstats_heap_alloc_bytes` |
| Open FDs | < 10000 | `process_open_fds` | `process_open_fds` |
| CPU Usage | < 80% | `process_cpu_seconds_total` | `rate(process_cpu_seconds_total[5m])` |
| RSS Memory | < 1GB | `process_resident_memory_bytes` | `process_resident_memory_bytes` |

### 7.2 Redis-Specific

| SLI | Target | Metric | PromQL |
|-----|--------|--------|--------|
| Redis Error Rate | < 0.01% | `throttle_redis_errors_total` | `rate(throttle_redis_errors_total[5m]) / rate(http_requests_total[5m])` |
| Eval Allow Errors | 0 | `throttle_redis_errors_total{operation="eval_allow"}` | `rate(throttle_redis_errors_total{operation="eval_allow"}[5m])` |
| Eval Dequeue Errors | < 0.01% | `throttle_redis_errors_total{operation="eval_dequeue"}` | `rate(throttle_redis_errors_total{operation="eval_dequeue"}[5m])` |

---

## 8. Error Budget Policy

### 8.1 Budget Calculation

| Period | Availability SLO | Error Budget | Allowed Errors (per 1M req) | Allowed Downtime |
|--------|-----------------|-------------|---------------------------|-----------------|
| Hourly | 99.9% | 0.1% | 1,000 | 3.6 seconds |
| Daily | 99.9% | 0.1% | 1,440 | 1 min 26s |
| Weekly | 99.9% | 0.1% | 10,080 | 10 min 4s |
| Monthly (30d) | 99.9% | 0.1% | 43,200 | 43 min 12s |
| Quarterly (90d) | 99.9% | 0.1% | 129,600 | 2h 9m 36s |

### 8.2 Budget Consumption Tiers

| Tier | Burn Rate | Budget Remaining | Status | Action |
|------|-----------|-----------------|--------|--------|
| Green | < 0.5x | > 50% | Healthy | Normal operations |
| Yellow | 0.5x - 2x | 25% - 50% | Watch | Increase monitoring, review recent deploys |
| Orange | 2x - 5x | 10% - 25% | Warning | Freeze deployments, focus on reliability |
| Red | > 5x | < 10% | Critical | All-hands, reliability-only work |

### 8.3 Budget Exhaustion Policy

```
IF error_budget_consumed >= 100%:
    1. STOP all feature deployments
    2. CREATE reliability sprint (next sprint)
    3. DEDICATE 100% engineering time to reliability
    4. REVIEW and potentially tighten SLO targets
    5. POST-MORTEM on all incidents in budget period
```

### 8.4 Budget Rollover

- Error budget **does not** roll over to next period
- Unused budget is forfeited (not a savings account)
- However, consistent under-budget may indicate SLO is too loose

---

## 9. Alerting Framework

### 9.1 Alert Severity Matrix

| Severity | Symbol | Response | Examples |
|----------|--------|----------|----------|
| **P1** | Critical | < 5 min | Service down, error rate > 10%, all requests failing |
| **P2** | High | < 15 min | p99 > 5s, error rate > 1%, Redis connection lost |
| **P3** | Warning | < 1 hour | p95 > SLO, queue usage > 80%, GC pause spike |
| **P4** | Info | Next business day | SLO approaching threshold, capacity trending up |

### 9.2 Alert Rules

#### Availability Alerts

| Rule | Condition | Severity | For |
|------|-----------|----------|-----|
| Service Down | `up == 0` | P1 | 1m |
| High Error Rate | `rate(http_errors_total[5m]) / rate(http_requests_total[5m]) > 0.05` | P2 | 5m |
| Error Budget Fast Burn | Burn rate > 14.4x over 30d window | P1 | 5m |
| Error Budget Slow Burn | Burn rate > 6x over 30d window | P2 | 1h |
| 5xx Spike | `rate(http_errors_total{status_code=~"5.."}[1m]) > 10` | P2 | 2m |

#### Latency Alerts

| Rule | Condition | Severity | For |
|------|-----------|----------|-----|
| p99 Breach | `histogram_quantile(0.99, ...) > 5` | P2 | 5m |
| p95 Breach | `histogram_quantile(0.95, ...) > 2` | P3 | 10m |
| p50 Degradation | `histogram_quantile(0.50, ...) > 0.5` | P3 | 15m |

#### Throttle Alerts

| Rule | Condition | Severity | For |
|------|-----------|----------|-----|
| Rejection Spike | Rejection rate > 20% | P3 | 5m |
| Queue Full | `throttle_queue_length > max_queue * 0.9` | P2 | 5m |
| Timeout Spike | Timeout rate > 5% | P2 | 5m |
| Window Saturated | `throttle_window_usage_ratio > 0.9` | P3 | 5m |
| Wait Time High | p95 wait time > 3s | P3 | 10m |

#### Infrastructure Alerts

| Rule | Condition | Severity | For |
|------|-----------|----------|-----|
| OOM Risk | `process_resident_memory_bytes > 800MB` | P2 | 5m |
| Goroutine Leak | `go_goroutines > 5000` | P3 | 10m |
| Redis Errors | `rate(throttle_redis_errors_total[5m]) > 0` | P2 | 5m |
| GC Pressure | GC pause p99 > 10ms | P3 | 10m |
| High In-Flight | `sum(http_requests_in_flight) > 500` | P3 | 5m |

### 9.3 Alerting Channels

| Severity | Channel | Escalation |
|----------|---------|-----------|
| P1 | PagerDuty + Slack #incidents | Engineering Manager in 5 min |
| P2 | PagerDuty + Slack #incidents | On-call lead in 15 min |
| P3 | Slack #alerts | Daily digest |
| P4 | Slack #alerts | Weekly digest |

### 9.4 Burn Rate Alert PromQL

```promql
# Multi-window multi-burn-rate alerts (Google SRE recommended)
# Fast Burn — 1h window, 14.4x burn rate
(
  sum(rate(http_errors_total[1h]))
  /
  sum(rate(http_requests_total[1h]))
) > (14.4 * (1 - 0.999))

# Slow Burn — 6h window, 6x burn rate
(
  sum(rate(http_errors_total[6h]))
  /
  sum(rate(http_requests_total[6h]))
) > (6 * (1 - 0.999))

# Slow Burn — 3d window, 1x burn rate
(
  sum(rate(http_errors_total[3d]))
  /
  sum(rate(http_requests_total[3d]))
) > (1 * (1 - 0.999))
```

---

## 10. Grafana Dashboard Panels

### 10.1 Overview Dashboard

| Row | Panel | Type | Query |
|-----|-------|------|-------|
| 1 | Availability (30d) | Stat | `1 - (sum(increase(http_errors_total[30d])) / sum(increase(http_requests_total[30d])))` |
| 1 | RPS | Stat | `sum(rate(http_requests_total[1m]))` |
| 1 | p99 Latency | Stat | `histogram_quantile(0.99, ...)` |
| 1 | Error Rate | Stat | `rate(http_errors_total[5m]) / rate(http_requests_total[5m])` |
| 1 | Error Budget Remaining | Gauge | `100% - consumed_budget` |
| 2 | Request Rate | Time series | `sum(rate(http_requests_total[5m])) by (endpoint)` |
| 2 | Error Rate | Time series | `sum(rate(http_errors_total[5m])) by (endpoint, status_code)` |
| 2 | Latency Heatmap | Heatmap | `rate(http_request_duration_seconds_bucket[5m])` |
| 3 | p50/p95/p99 Latency | Time series | `histogram_quantile(...)` per percentile |
| 3 | Response Size | Time series | `histogram_quantile(0.95, rate(http_response_size_bytes_bucket[5m]))` |
| 4 | Active Requests | Time series | `sum(http_requests_in_flight) by (endpoint)` |
| 4 | Goroutines | Time series | `go_goroutines` |
| 4 | Heap Usage | Time series | `go_memstats_heap_alloc_bytes` |

### 10.2 Throttle Dashboard

| Row | Panel | Type | Query |
|-----|-------|------|-------|
| 1 | Window Usage | Gauge | `throttle_window_usage_ratio` |
| 1 | Queue Length | Gauge | `throttle_queue_length` |
| 1 | Allowed/Rejected/Queued/Timeout | Pie | `sum(increase(throttle_requests_total[1h])) by (result)` |
| 2 | Throttle Rate by Result | Time series | `sum(rate(throttle_requests_total[5m])) by (result)` |
| 2 | Queue Wait Time | Time series | `histogram_quantile(0.95, rate(throttle_wait_time_seconds_bucket[5m]))` |
| 2 | Throttle Duration | Time series | `histogram_quantile(0.95, rate(throttle_request_duration_seconds_bucket[5m]))` |
| 3 | Redis Errors | Time series | `sum(rate(throttle_redis_errors_total[5m])) by (operation)` |
| 3 | Window Usage Trend | Time series | `throttle_window_usage_ratio` by endpoint |

### 10.3 SLO Dashboard

| Panel | Description |
|-------|-------------|
| 30-day Availability | Current availability vs SLO target |
| Error Budget Bar | Visual bar showing remaining error budget |
| Burn Rate Chart | Multi-window burn rate over time |
| SLO Compliance Calendar | Green/yellow/red days based on daily SLO |
| Incident Timeline | Overlay incidents on SLO chart |

---

## 11. Incident Management

### 11.1 Severity Matrix

| Severity | Definition | Example |
|----------|-----------|---------|
| **P1 — Critical** | Complete service outage or data loss | All requests returning 5xx, Redis unavailable, OOM crash |
| **P2 — High** | Major degradation affecting > 50% users | Error rate > 1%, p99 > 5s, queue consistently full |
| **P3 — Medium** | Minor degradation, partial impact | p95 > SLO, queue usage > 80%, intermittent Redis errors |
| **P4 — Low** | No user impact, informational | Trend approaching threshold, non-critical metric anomaly |

### 11.2 Response SLA

| Severity | Acknowledge | Initial Update | Resolution | Executive Update |
|----------|------------|---------------|------------|-----------------|
| P1 | < 5 min | < 15 min | < 1 hour | < 30 min |
| P2 | < 15 min | < 30 min | < 4 hours | < 1 hour |
| P3 | < 1 hour | < 2 hours | < 24 hours | N/A |
| P4 | < 4 hours | < 8 hours | < 1 week | N/A |

### 11.3 Incident Lifecycle

```
Detect → Triage → Respond → Mitigate → Resolve → Post-Mortem → Action Items
   │        │         │         │          │            │              │
   ▼        ▼         ▼         ▼          ▼            ▼              ▼
 Alert    Assess    Execute   Reduce     Fix root    Document   Track to
 trigger  severity  playbook  impact     cause       learnings  completion
```

### 11.4 Post-Mortem Template

```markdown
## Incident Post-Mortem: [INC-XXXX]

### Summary
- **Date**: YYYY-MM-DD
- **Duration**: X hours Y minutes
- **Severity**: P1/P2/P3/P4
- **Impact**: X users affected, Y% error rate, Z requests failed
- **Error Budget Consumed**: X%

### Timeline
| Time (UTC) | Event |
|-----------|-------|
| HH:MM | Alert triggered |
| HH:MM | On-call acknowledged |
| HH:MM | Root cause identified |
| HH:MM | Mitigation applied |
| HH:MM | Service restored |

### Root Cause
[Detailed explanation]

### Contributing Factors
1. ...
2. ...

### Action Items
| # | Action | Owner | Priority | Due Date | Status |
|---|--------|-------|----------|----------|--------|
| 1 | | | | | |

### Lessons Learned
- What went well: ...
- What could be improved: ...
- Where did we get lucky: ...
```

---

## 12. On-Call Procedures

### 12.1 On-Call Rotation

| Role | Responsibility | Escalation |
|------|---------------|-----------|
| Primary On-Call | First responder, 24/7 | Secondary after 15 min (P1) |
| Secondary On-Call | Backup, P1 escalation | Engineering Manager after 30 min |
| Engineering Manager | Executive escalation | VP Engineering for P1 > 1 hour |

### 12.2 Handoff Checklist

- [ ] Verify no active P1/P2 incidents
- [ ] Review open P3/P4 tickets
- [ ] Check error budget status (current consumption %)
- [ ] Review recent deployments (last 24h)
- [ ] Check scheduled maintenance windows
- [ ] Verify PagerDuty/Slack notification routing

### 12.3 On-Call Compensation

| Severity | Per-incident | Max per-week |
|----------|-------------|-------------|
| P1 | $X | $Y |
| P2 | $X | $Y |
| Weekday on-call | $X/day | — |
| Weekend on-call | $X/day | — |

---

## 13. Runbook

### 13.1 Service Down (P1)

```
1. CHECK service health: curl http://localhost:8000/no-throttle
2. CHECK metrics endpoint: curl http://localhost:8000/metrics
3. CHECK container status: docker ps / kubectl get pods
4. CHECK logs: docker logs <container> / kubectl logs <pod>
5. CHECK Redis connectivity: redis-cli ping
6. IF OOM detected:
   a. Restart container
   b. Review memory limit configuration
   c. Check for goroutine leak: go_goroutines metric
7. IF Redis unreachable:
   a. Check Redis server status
   b. Check network connectivity
   c. Consider failover to in-memory throttle
8. ESCELATE to secondary on-call if unresolved in 15 min
```

### 13.2 High Error Rate (P2)

```
1. IDENTIFY error type from http_errors_total by status_code
2. CHECK throttle rejection rate:
   rate(throttle_requests_total{result="rejected"}[5m])
3. CHECK timeout rate:
   rate(throttle_requests_total{result="timeout"}[5m])
4. CHECK Redis errors:
   rate(throttle_redis_errors_total[5m])
5. IF high rejection rate:
   a. Review rate_limit and max_queue config
   b. Consider temporarily increasing rate_limit
   c. Review traffic patterns for spike
6. IF high timeout rate:
   a. Check downstream handler latency
   b. Review queue polling interval
   c. Consider increasing max_queue
```

### 13.3 High Latency (P2/P3)

```
1. CHECK p99 latency: histogram_quantile(0.99, ...)
2. CHECK queue wait time: histogram_quantile(0.95, throttle_wait_time_seconds)
3. CHECK window usage: throttle_window_usage_ratio
4. CHECK GC pause: go_gc_duration_seconds
5. CHECK goroutine count: go_goroutines
6. IF queue wait time high:
   a. Rate limit is saturated — expected behavior
   b. Consider increasing rate_limit or window_seconds
7. IF GC pause high:
   a. Review heap allocation patterns
   b. Reduce per-request memory allocation
8. IF goroutine leak:
   a. Take goroutine stack dump: pprof goroutine
   b. Identify leaking goroutines
   c. Restart service as immediate fix
```

### 13.4 Queue Full / High Rejection (P3)

```
1. CHECK current queue: throttle_queue_length
2. CHECK window usage: throttle_window_usage_ratio
3. CHECK traffic: sum(rate(http_requests_total[5m])) by (endpoint)
4. IF legitimate traffic spike:
   a. Increase rate_limit (if downstream allows)
   b. Increase max_queue (monitor memory)
   c. Add more instances (horizontal scaling)
5. IF abuse / DDoS:
   a. Identify source IP
   b. Add IP-based blocking upstream
   c. Contact security team
```

---

## 14. Capacity Planning

### 14.1 Resource Estimation

| Resource | Per Request | At 100 RPS | At 1000 RPS | Headroom |
|----------|------------|-----------|------------|----------|
| Memory | ~1MB | ~100MB | ~1GB | 2x estimated |
| Goroutines | ~2 | ~200 | ~2000 | < 10000 limit |
| CPU | ~0.1ms | ~10% | ~100% | Scale horizontally |
| Redis Ops | ~3 (eval + llen + lrem) | ~300/s | ~3000/s | Redis cluster if > 10K/s |

### 14.2 Scaling Strategy

| Traffic Level | Strategy | Action |
|--------------|----------|--------|
| < 100 RPS | Single instance | Default deployment |
| 100 - 500 RPS | Horizontal scaling | 2-3 instances + load balancer |
| 500 - 2000 RPS | Redis cluster | Migrate to Redis Cluster |
| > 2000 RPS | Redis + sharding | Per-endpoint Redis keys |

### 14.3 Load Testing Schedule

| Test Type | Frequency | Tool | Duration |
|-----------|-----------|------|----------|
| Baseline | Weekly | k6 | 10 min |
| Peak Simulation | Monthly | k6 | 30 min |
| Stress Test | Quarterly | k6 | 1 hour |
| Soak Test | Quarterly | k6 | 4+ hours |
| Chaos Test | Semi-annually | Gremlin/Litmus | Varies |

### 14.4 Capacity Headroom Targets

| Metric | Warning Threshold | Critical Threshold |
|--------|------------------|-------------------|
| CPU | > 60% | > 80% |
| Memory | > 70% | > 85% |
| Goroutines | > 3000 | > 5000 |
| Redis OPS | > 5000/s | > 8000/s |
| Active Connections | > 500 | > 800 |

---

## 15. Load Testing & Validation

### 15.1 Test Scenarios

Based on `LOADTEST.md`:

| Scenario | Config | Expected Outcome |
|----------|--------|-----------------|
| Normal Load | 10 RPS, 1000 req | 100% success, p99 < 500ms |
| Sustained Load | Rate limit, 30 min | No memory leak, stable latency |
| Burst Traffic | 100 RPS spike, 1 min | Queue absorbs burst, graceful rejection |
| Stress Test | 2x capacity, 1 hour | No OOM, no crash, all responses |
| Failover | Kill Redis mid-test | Graceful error, no silent failure |

### 15.2 Validation Checklist

- [ ] All endpoints respond within SLO latency targets
- [ ] No OOM under max expected load
- [ ] Queue correctly absorbs and drains
- [ ] Rejected requests return proper 503 with body
- [ ] Timeout requests return proper 408
- [ ] Memory stable (no leak) over 30+ min soak test
- [ ] Metrics accurately reflect system state
- [ ] Alerts trigger at correct thresholds

---

## 16. Change Management

### 16.1 Deployment Risk Assessment

| Risk Level | Criteria | Approval Required |
|-----------|----------|------------------|
| Low | Config change, docs update | 1 engineer |
| Medium | Code change, new endpoint | 1 engineer + 1 reviewer |
| High | Throttle algorithm change, Redis schema | Team lead + reviewer |
| Critical | Rate limit parameters, infrastructure | Engineering manager + team lead |

### 16.2 Deployment Checklist

- [ ] All tests passing (`go test ./...`)
- [ ] Load test validated at current traffic level
- [ ] Rollback plan documented
- [ ] Alerts reviewed for new metrics/changes
- [ ] Deployment window: low-traffic period (if applicable)
- [ ] Monitoring dashboards open during deploy
- [ ] Error budget checked (sufficient budget remaining)

### 16.3 Rollback Criteria

| Time After Deploy | Condition | Action |
|-------------------|-----------|--------|
| 0 - 5 min | Error rate > 1% | Immediate rollback |
| 5 - 30 min | Error rate > 0.5% | Rollback + investigate |
| 30 - 60 min | p99 > 2x baseline | Rollback + investigate |
| 60+ min | SLO trend declining | Evaluate at next review |

---

## 17. Reporting & Review Cadence

### 17.1 Regular Reviews

| Meeting | Frequency | Attendees | Agenda |
|---------|-----------|-----------|--------|
| Standup | Daily | Team | Active incidents, error budget, dashboard review |
| SLO Review | Weekly | Team + EM | Error budget status, near-miss incidents, alert tuning |
| Retrospective | Bi-weekly | Team | Process improvements, tooling, automation |
| SLO Deep Dive | Monthly | Team + EM + PM | SLO adjustments, capacity review, cost analysis |
| Quarterly Business Review | Quarterly | EM + PM + CTO | SLO compliance, budget, roadmap alignment |

### 17.2 Report Template

```markdown
## Monthly SLO Report — YYYY-MM

### Summary
| SLO | Target | Actual | Status | Error Budget Consumed |
|-----|--------|--------|--------|----------------------|
| Availability | 99.9% | XX.XX% | ✅/❌ | XX% |
| p99 Latency | < 1000ms | XXXms | ✅/❌ | N/A |
| Error Rate | < 0.1% | 0.XX% | ✅/❌ | N/A |
| Throughput | > 100 RPS | XX RPS | ✅/❌ | N/A |

### Incident Summary
- Total incidents: X
- P1: X, P2: X, P3: X
- Total error budget consumed by incidents: XX%

### Key Changes
- ...

### Recommendations
- ...
```

---

## 18. Compliance & Audit

### 18.1 Audit Trail

| Item | Requirement | Implementation |
|------|------------|----------------|
| Metric Collection | 99.9% collection uptime | Prometheus local storage + remote write |
| Alert Delivery | < 30s delivery | PagerDuty + Slack webhook |
| Incident Records | Immutable, timestamped | Incident management system |
| SLO Reports | Monthly, retained 2 years | Automated generation + storage |
| Deployment Logs | Immutable audit trail | Git history + CI/CD logs |

### 18.2 Data Retention

| Data Type | Retention Period |
|-----------|-----------------|
| Metrics (high-res) | 30 days |
| Metrics (downsampled) | 1 year |
| Incident records | Indefinite |
| SLO reports | 2 years |
| Post-mortems | Indefinite |
| Load test results | 1 year |

---

## 19. Appendix A: Full PromQL Reference

### HTTP General

```promql
# RPS
sum(rate(http_requests_total[1m])) by (endpoint)

# RPS by status code
sum(rate(http_requests_total[1m])) by (endpoint, status_code)

# Availability
1 - (sum(rate(http_errors_total{status_code=~"5.."}[5m])) / sum(rate(http_requests_total[5m])))

# Latency percentiles
histogram_quantile(0.50, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, endpoint))
histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, endpoint))
histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, endpoint))

# Average latency
rate(http_request_duration_seconds_sum[5m]) / rate(http_request_duration_seconds_count[5m])

# Active requests
sum(http_requests_in_flight) by (endpoint)

# Response size p95
histogram_quantile(0.95, sum(rate(http_response_size_bytes_bucket[5m])) by (le, endpoint))

# Error rate
sum(rate(http_errors_total[5m])) by (endpoint) / sum(rate(http_requests_total[5m])) by (endpoint)

# 5xx vs 4xx
sum(rate(http_errors_total{status_code=~"5.."}[5m]))
sum(rate(http_errors_total{status_code=~"4.."}[5m]))
```

### Throttle

```promql
# Throttle result distribution
sum(rate(throttle_requests_total[5m])) by (result)

# Rejection rate
rate(throttle_requests_total{result="rejected"}[5m]) / rate(throttle_requests_total[5m])

# Timeout rate
rate(throttle_requests_total{result="timeout"}[5m]) / rate(throttle_requests_total[5m])

# Allowed rate
rate(throttle_requests_total{result="allowed"}[5m]) / rate(throttle_requests_total[5m])

# Queue length
throttle_queue_length

# Window usage
throttle_window_usage_ratio

# Queue wait time percentiles
histogram_quantile(0.50, sum(rate(throttle_wait_time_seconds_bucket[5m])) by (le, endpoint))
histogram_quantile(0.95, sum(rate(throttle_wait_time_seconds_bucket[5m])) by (le, endpoint))
histogram_quantile(0.99, sum(rate(throttle_wait_time_seconds_bucket[5m])) by (le, endpoint))

# Average wait time
rate(throttle_wait_time_seconds_sum[5m]) / rate(throttle_wait_time_seconds_count[5m])

# Redis error rate
sum(rate(throttle_redis_errors_total[5m])) by (operation)
```

### Infrastructure

```promql
# Goroutines
go_goroutines

# GC pause p99
histogram_quantile(0.99, rate(go_gc_duration_seconds_bucket[5m]))

# Heap allocation
go_memstats_heap_alloc_bytes

# RSS memory
process_resident_memory_bytes

# CPU
rate(process_cpu_seconds_total[5m])

# Open file descriptors
process_open_fds
```

### Error Budget

```promql
# Error budget consumed (30d window)
(
  sum(increase(http_errors_total{status_code=~"5.."}[30d]))
  /
  sum(increase(http_requests_total[30d]))
) / (1 - 0.999) * 100

# Error budget remaining
100 - (
  sum(increase(http_errors_total{status_code=~"5.."}[30d]))
  /
  sum(increase(http_requests_total[30d]))
) / (1 - 0.999) * 100

# 30-day rolling availability
1 - (
  sum(increase(http_errors_total{status_code=~"5.."}[30d]))
  /
  sum(increase(http_requests_total[30d]))
)
```

---

## 20. Appendix B: Metrics Catalog

### HTTP General

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `http_requests_total` | Counter | `method`, `endpoint`, `status_code` | Total HTTP requests. Use `rate()` for RPS. |
| `http_request_duration_seconds` | Histogram | `method`, `endpoint`, `status_code` | Duration of HTTP requests. Buckets: `0.005s` - `10s`. |
| `http_requests_in_flight` | Gauge | `method`, `endpoint` | Number of requests currently being processed. |
| `http_response_size_bytes` | Histogram | `method`, `endpoint` | Size of HTTP response body in bytes. |
| `http_errors_total` | Counter | `method`, `endpoint`, `error_type` | Total number of HTTP 5xx errors. |

### Throttle

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `throttle_requests_total` | Counter | `type`, `method`, `endpoint`, `result` | Total throttle requests. `result`: `allowed`, `rejected`, `queued`, `timeout`. |
| `throttle_queue_length` | Gauge | `type`, `method`, `endpoint` | Current number of requests in throttle queue. |
| `throttle_request_duration_seconds` | Histogram | `type`, `method`, `endpoint`, `result` | Duration of throttle requests (allowed & queued). |
| `throttle_window_usage_ratio` | Gauge | `type`, `method`, `endpoint` | Ratio of current window usage. |
| `throttle_wait_time_seconds` | Histogram | `type`, `method`, `endpoint` | Time spent waiting in throttle queue. |

### Redis

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `throttle_redis_errors_total` | Counter | `operation` | Total Redis errors. `operation`: `eval_allow`, `eval_dequeue`. |

### Runtime

| Metric | Type | Description |
|--------|------|-------------|
| `go_*` | Various | Go runtime metrics (GC, goroutines, memory). |
| `process_*` | Various | Process metrics (CPU, memory, file descriptors). |
