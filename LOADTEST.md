# Go-Throttle Load Test Summary

## Configuration

| Setting | Value |
|---|---|
| Rate Limit | 10 req / 5s |
| Max Queue | 20 |
| Memory per Request | 10MB |
| Container Memory Limit | 64MB |
| Total Requests | 40 (shared-iterations) |
| VUs | 40 |
| Response Delay | 500ms |

## Endpoints

| Endpoint | Description |
|---|---|
| `/no-throttle` | Without throttle |
| `/throttle` | In-memory throttle |
| `/redis` | Redis throttle (global) |

## Results

### `/throttle`

```
Total requests:  40
200 OK:          30 (75%)
503 Server Busy: 10 (25%)
OOM Crash:       No
```

| Response Time | Value |
|---|---|
| min | 118ms |
| avg | 5200ms |
| med | 5208ms |
| p90 | 10269ms |
| p95 | 10271ms |
| max | 10277ms |

### `/no-throttle`

```
Total requests:  40
200 OK:          6 (15%)
503 Server Busy: 0
OOM Crash:       Yes (34 requests lost)
```

| Response Time | Value |
|---|---|
| min | 137ms |
| avg | 143ms |
| med | 143ms |
| p90 | 146ms |
| p95 | 147ms |
| max | 170ms |

## Comparison

| Metric | `/throttle` | `/no-throttle` |
|---|---|---|
| Success Rate | 75% | 15% |
| OOM | No | Yes |
| Avg Response Time | 5200ms | 143ms |
| Max Response Time | 10277ms | 170ms |

## Conclusion

- **Without throttle**: Container OOM, only 6 requests succeeded. 34 requests lost (silent failure), clients receive no response.
- **With throttle**: All requests receive response. 30 succeeded + 10 rejected with `503 Server Busy`. Server remains alive and stable.

**Trade-off**: Throttle has higher response time due to queuing, but guarantees server won't crash due to memory overload.

## How to Run

```bash
# Build & run
docker compose up --build

# Load test
k6 run loadtest.js                                              # /no-throttle (default)
k6 run loadtest.js -e URL=http://localhost:8000/throttle         # in-memory throttle
k6 run loadtest.js -e URL=http://localhost:8000/redis           # redis throttle

# Monitor memory
docker stats
```
