# Middleware

Package middleware menyediakan throttle middleware untuk Echo framework dengan dua implementasi: Redis (global limit) dan In-Memory (per-instance limit).

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

Global limit menggunakan Redis Lua script (atomic operation).

### Komponen Redis

| Data Structure | Key | Fungsi |
|---|---|---|
| Sorted Set | `throttle:global` | Sliding window, score = timestamp (ms), member = unique ID |
| List | `throttle:global:queue` | Antrian request yang menunggu slot |

### Lua Script

**allowScript** — Dieksekusi saat request masuk:

1. `ZREMRANGEBYSCORE` — Hapus entry yang sudah expired (di luar window)
2. `ZCARD` — Hitung request aktif dalam window
3. Jika `< rate_limit` → `ZADD` + return `0` (lanjutkan)
4. Jika queue penuh → return `2` (reject)
5. Jika tidak → `RPUSH` ke queue + return `1` (tunggu)

**dequeueScript** — Dieksekusi setiap 100ms oleh request di queue:

1. `ZCARD` — Cek apakah ada slot kosong
2. Jika ada → `LREM` keluar dari queue + `ZADD` + return `1` (lanjutkan)
3. Jika belum → return `0` (tetap tunggu)

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

| Kode | Kondisi |
|---|---|
| 200 | Request diproses |
| 503 | Queue penuh (`{"error": "server busy"}`) |
| 408 | Timeout saat menunggu queue (`{"error": "request timeout"}`) |
| 500 | Redis error |

## In-Memory Throttle (`InMemoryThrottle`)

Per-instance limit menggunakan `sync.Mutex` + slice.

### Komponen

| Variable | Tipe | Fungsi |
|---|---|---|
| `mu` | `sync.Mutex` | Lock untuk race condition |
| `times` | `[]int64` | Sliding window (timestamp ms) |
| `queue` | `[]chan struct{}` | Antrian request yang menunggu |

### Config

```go
middleware.InMemoryThrottle(middleware.InMemoryThrottleConfig{
    RateLimit:     10,
    WindowSeconds: 5,
    MaxQueue:      50,
})
```

## Perbandingan

| | Redis | In-Memory |
|---|---|---|
| Scope | Global (multi-instance) | Per-instance |
| Eksternal Dependency | Redis | Tidak ada |
| Latency | +network ke Redis | Lebih cepat |
| Use Case | API Key limit, IP limit | Server protection |
| Data Persistence | Ya | Hilang saat restart |
