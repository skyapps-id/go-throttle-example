# Rate Limit vs Throttle

## Rate Limit

Limit the number of requests within a certain period. Excess requests are **immediately rejected**.

```
Request 1-10  → 200 OK
Request 11    → 429 Too Many Requests
Request 12    → 429 Too Many Requests
```

**Purpose:** Prevent abuse (brute force, DDoS, API over-usage).

## Throttle

Limit the number of requests **processed simultaneously**. Excess requests are **queued (delayed)**, not rejected. Rejection only occurs when the queue is full.

```
Request 1-10  → processed
Request 11    → enters queue, waits for available slot
Request 12    → enters queue, waits for available slot
...
Request 61    → queue full → 503 Server Busy
```

**Purpose:** Protect server resources (memory, CPU, database connections).

## Comparison

| | Rate Limit | Throttle |
|---|---|---|
| Excess requests | Immediately rejected | Queued first |
| Goal | Abuse protection | Concurrency control |
| Response | 429 Too Many Requests | 503 Server Busy (queue full) |
| Response | 408 Timeout (none) | 408 Timeout (queue timeout) |
| Example | API Key limit (GitHub API) | Server resource protection |
| Timeout | None | Yes, requests can timeout while waiting |
| Queue | None | Yes, requests held until slot available |

## Use Cases

### Rate Limit

```bash
# GitHub API: 60 req/hour
curl https://api.github.com/user   # 200 OK
# ... 60x request
curl https://api.github.com/user   # 429 Too Many Requests
```

### Throttle

```bash
# Server can only process 10 requests simultaneously
curl http://localhost:8080/api  # request 1-10 → processed
curl http://localhost:8080/api  # request 11-30 → enters queue, slower response time
curl http://localhost:8080/api  # request 31+ → 503 Server Busy
```

## When to Use What

| Requirement | Solution |
|---|---|
| Limit usage per API Key | Rate Limit |
| Limit requests per IP | Rate Limit |
| Protect server from OOM/CPU overload | Throttle |
| Protect database connection pool | Throttle |
| SaaS billing (x requests per month) | Rate Limit |
| Background job processing | Throttle |

## Combination

In production, both can be combined:

```
Incoming Request
     │
     ▼
┌──────────────┐
│ Rate Limit   │ → 429 (API Key limit)
│ 100 req/day  │
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

Rate limit at the edge (gateway/load balancer) for billing & abuse protection, throttle internally (middleware) for resource protection.
