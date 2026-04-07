# Rate Limit vs Throttle

## Rate Limit

Batasi jumlah request dalam periode tertentu. Request yang kelebihan **langsung ditolak**.

```
Request 1-10  → 200 OK
Request 11    → 429 Too Many Requests
Request 12    → 429 Too Many Requests
```

**Tujuan:** Mencegah abuse (brute force, DDoS, over-usage API).

## Throttle

Batasi jumlah request yang **diproses bersamaan**. Request yang kelebihan **ditangguhkan (queue)**, bukan ditolak. Ditolak hanya kalau queue penuh.

```
Request 1-10  → diproses
Request 11    → masuk antrian, tunggu slot kosong
Request 12    → masuk antrian, tunggu slot kosong
...
Request 61    → queue penuh → 503 Server Busy
```

**Tujuan:** Melindungi resource server (memori, CPU, koneksi database).

## Perbandingan

| | Rate Limit | Throttle |
|---|---|---|
| Kelebihan request | Langsung ditolak | Di-queue dulu |
| Goal | Perlindungan abuse | Kontrol concurrency |
| Response | 429 Too Many Requests | 503 Server Busy (queue penuh) |
| Response | 408 Timeout (tidak ada) | 408 Timeout (timeout antrian) |
| Contoh | API Key limit (GitHub API) | Server resource protection |
| Timeout | Tidak ada | Ya, request bisa timeout saat menunggu |
| Antrian | Tidak ada | Ya, request ditahan sampai ada slot |

## Contoh Kasus

### Rate Limit

```bash
# GitHub API: 60 req/hour
curl https://api.github.com/user   # 200 OK
# ... 60x request
curl https://api.github.com/user   # 429 Too Many Requests
```

### Throttle

```bash
# Server hanya sanggup proses 10 request bersamaan
curl http://localhost:8080/api  # request 1-10 → diproses
curl http://localhost:8080/api  # request 11-30 → masuk antrian, response time lebih lambat
curl http://localhost:8080/api  # request 31+ → 503 Server Busy
```

## Kapan Pakai Apa

| Kebutuhan | Solusi |
|---|---|
| Batasi usage per API Key | Rate Limit |
| Batasi request per IP | Rate Limit |
| Lindungi server dari OOM/CPU overload | Throttle |
| Lindungi database connection pool | Throttle |
| SaaS billing (x request per bulan) | Rate Limit |
| Background job processing | Throttle |

## Gabungan

Dalam produksi, keduanya bisa digabung:

```
Request Masuk
     │
     ▼
┌──────────────┐
│ Rate Limit   │ → 429 (API Key limit)
│ 100 req/hari │
└──────┬───────┘
       ▼
┌──────────────┐
│ Throttle     │ → 503 / 408 (server overload)
│ 10 req/5s    │
└──────┬───────┘
       ▼
┌──────────────┐
│ Handler      │ → 200 OK
└──────────────┘
```

Rate limit di luar (gateway/load balancer) untuk billing & abuse protection, throttle di dalam (middleware) untuk resource protection.
