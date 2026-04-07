package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

type InMemoryThrottleConfig struct {
	RateLimit     int
	WindowSeconds int
	MaxQueue      int
}

func InMemoryThrottle(config InMemoryThrottleConfig) echo.MiddlewareFunc {
	var (
		mu    sync.Mutex
		times []int64
		queue []chan struct{}
	)

	cleanup := func(now int64) {
		windowStart := now - int64(config.WindowSeconds)*1000
		i := 0
		for i < len(times) {
			if times[i] > windowStart {
				break
			}
			i++
		}
		if i > 0 {
			times = times[:copy(times, times[i:])]
		}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			mu.Lock()
			now := time.Now().UnixMilli()
			cleanup(now)

			if len(times) < config.RateLimit {
				times = append(times, now)
				mu.Unlock()
				ThrottleRequestsTotal.WithLabelValues("inmem", c.Request().Method, c.Path(), "allowed").Inc()
				ThrottleRequestDuration.WithLabelValues("inmem", c.Request().Method, c.Path(), "allowed").Observe(time.Since(start).Seconds())
				ThrottleWindowUsage.WithLabelValues("inmem", c.Request().Method, c.Path()).Set(float64(len(times)) / float64(config.RateLimit))
				return next(c)
			}

			if len(queue) >= config.MaxQueue {
				mu.Unlock()
				ThrottleRequestsTotal.WithLabelValues("inmem", c.Request().Method, c.Path(), "rejected").Inc()
				return c.JSON(http.StatusServiceUnavailable, map[string]string{
					"error": "server busy",
				})
			}

			ch := make(chan struct{})
			queue = append(queue, ch)
			ThrottleRequestsTotal.WithLabelValues("inmem", c.Request().Method, c.Path(), "queued").Inc()
			ThrottleQueueLength.WithLabelValues("inmem", c.Request().Method, c.Path()).Set(float64(len(queue)))
			ThrottleWindowUsage.WithLabelValues("inmem", c.Request().Method, c.Path()).Set(float64(len(times)+len(queue)) / float64(config.RateLimit))
			mu.Unlock()

			queueStart := time.Now()

			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			for range ticker.C {
				select {
				case <-c.Request().Context().Done():
					mu.Lock()
					for i, c := range queue {
						if c == ch {
							queue = append(queue[:i], queue[i+1:]...)
							break
						}
					}
					ThrottleQueueLength.WithLabelValues("inmem", c.Request().Method, c.Path()).Set(float64(len(queue)))
					mu.Unlock()
					ThrottleRequestsTotal.WithLabelValues("inmem", c.Request().Method, c.Path(), "timeout").Inc()
					return c.JSON(http.StatusRequestTimeout, map[string]string{
						"error": "request timeout",
					})
				default:
					mu.Lock()
					cleanup(time.Now().UnixMilli())
					if len(times) < config.RateLimit {
						times = append(times, time.Now().UnixMilli())
						for i, c := range queue {
							if c == ch {
								queue = append(queue[:i], queue[i+1:]...)
								break
							}
						}
						ThrottleQueueLength.WithLabelValues("inmem", c.Request().Method, c.Path()).Set(float64(len(queue)))
						ThrottleWindowUsage.WithLabelValues("inmem", c.Request().Method, c.Path()).Set(float64(len(times)+len(queue)) / float64(config.RateLimit))
						mu.Unlock()
						ThrottleWaitTime.WithLabelValues("inmem", c.Request().Method, c.Path()).Observe(time.Since(queueStart).Seconds())
						ThrottleRequestDuration.WithLabelValues("inmem", c.Request().Method, c.Path(), "queued").Observe(time.Since(start).Seconds())
						return next(c)
					}
					mu.Unlock()
				}
			}

			return nil
		}
	}
}
