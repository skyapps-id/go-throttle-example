# Go-Throttle Load Test Summary

## Konfigurasi

| Setting | Value |
|---|---|
| Rate Limit | 10 req / 5s |
| Max Queue | 20 |
| Memory per Request | 10MB |
| Container Memory Limit | 64MB |
| Total Requests | 40 (shared-iterations) |
| VUs | 40 |
| Response Delay | 500ms |

## Endpoint

| Endpoint | Description |
|---|---|
| `/no-throttle` | Tanpa throttle |
| `/throttle` | In-memory throttle |
| `/redis` | Redis throttle (global) |

## Hasil

### `/throttle`

```
Total requests:  40
200 OK:          30 (75%)
503 Server Busy: 10 (25%)
OOM Crash:       Tidak
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
OOM Crash:       Ya (34 request hilang)
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
| OOM | Tidak | Ya |
| Avg Response Time | 5200ms | 143ms |
| Max Response Time | 10277ms | 170ms |

## Kesimpulan

- **Tanpa throttle**: Container OOM, hanya 6 request berhasil. 34 request hilang (silent failure), client tidak mendapat response apapun.
- **Pakai throttle**: Semua request mendapat response. 30 berhasil + 10 ditolak dengan `503 Server Busy`. Server tetap hidup dan stabil.

**Trade-off**: Throttle memiliki response time lebih tinggi karena antrian, namun menjamin server tidak crash akibat kelebihan beban memori.

## Cara Menjalankan

```bash
# Build & run
docker compose up --build

# Load test
k6 run loadtest.js                                              # /no-throttle (default)
k6 run loadtest.js -e URL=http://localhost:8080/throttle         # in-memory throttle
k6 run loadtest.js -e URL=http://localhost:8080/redis           # redis throttle

# Monitor memory
docker stats
```
