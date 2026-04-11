# Middleware

Package middleware provides throttle middleware for Echo framework with two implementations: Redis (global limit) and In-Memory (per-instance limit).

## Flow

```
Request Masuk
     │
     ▼
┌─────────────────┐
│ Hitung request  │
│ dalam window    │
└────────┬────────┘
         │
    ┌────▼────┐
    │ < Limit │
    │  ?      │
    └────┬────┘
         │
    ┌────┴────────────────┐
    │ Ya                  │ Tidak
    ▼                     ▼
┌──────────┐    ┌─────────────────┐
│ Proses   │    │ Queue penuh?   │
│ Request  │    │ MaxQueue >= ?   │
└──────────┘    └───────┬─────────┘
                        │
                   ┌────┴────────────────┐
                   │ Tidak               │ Ya
                   ▼                     ▼
            ┌─────────────┐      ┌──────────────┐
            │ Masuk Queue │      │ 503 Response │
            │ Poll 100ms  │      │ Server Busy  │
            └──────┬──────┘      └──────────────┘
                   │
              ┌────▼────┐
              │ Slot    │
              │ Kosong? │
              └────┬────┘
                   │
              ┌────┴────────────────┐
              │ Ya                  │ Timeout
              ▼                     ▼
       ┌──────────┐         ┌──────────────┐
       │ Proses   │         │ 408 Response │
       │ Request  │         │ Auto keluar  │
       └──────────┘         │ dari queue   │
                            └──────────────┘
```

## Redis Throttle (`Throttle`)

Global limit using Redis Lua script (atomic operation).

### Redis Components

| Data Structure | Key | Function |
|---|---|---|
| Sorted Set | `throttle:global` | Sliding window, score = timestamp (ms), member = unique ID |
| List | `throttle:global:queue` | Queue of requests waiting for slot |

### Lua Script

**allowScript** — Executed when request arrives:

1. `ZREMRANGEBYSCORE` — Remove expired entries (outside window)
2. `ZCARD` — Count active requests in window
3. If `< rate_limit` → `ZADD` + return `0` (proceed)
4. If queue full → return `2` (reject)
5. Else → `RPUSH` to queue + return `1` (wait)

**dequeueScript** — Executed every 100ms by queued requests:

1. `ZCARD` — Check if slot available
2. If yes → `LREM` from queue + `ZADD` + return `1` (proceed)
3. If not → return `0` (keep waiting)

### Config

```go
middleware.Throttle(middleware.ThrottleConfig{
    RedisClient:   rdb,
    RateLimit:     10,   // max request dalam window
    WindowSeconds: 5,    // ukuran sliding window (detik)
    MaxQueue:      50,   // max antrian
    KeyPrefix:     "throttle:",
})
```

### Response

| Code | Condition |
|---|---|
| 200 | Request processed |
| 503 | Queue full (`{"error": "server busy"}`) |
| 408 | Timeout while waiting in queue (`{"error": "request timeout"}`) |
| 500 | Redis error |

## In-Memory Throttle (`InMemoryThrottle`)

Per-instance limit using `sync.Mutex` + slice.

### Components

| Variable | Type | Function |
|---|---|---|
| `mu` | `sync.Mutex` | Lock for race condition |
| `times` | `[]int64` | Sliding window (timestamp ms) |
| `queue` | `[]chan struct{}` | Queue of waiting requests |

### Config

```go
middleware.InMemoryThrottle(middleware.InMemoryThrottleConfig{
    RateLimit:     10,
    WindowSeconds: 5,
    MaxQueue:      50,
})
```

## Comparison

| | Redis | In-Memory |
|---|---|---|
| Scope | Global (multi-instance) | Per-instance |
| External Dependency | Redis | None |
| Latency | +network to Redis | Faster |
| Use Case | API Key limit, IP limit | Server protection |
| Data Persistence | Yes | Lost on restart |
