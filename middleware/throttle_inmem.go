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
			mu.Lock()
			now := time.Now().UnixMilli()
			cleanup(now)

			if len(times) < config.RateLimit {
				times = append(times, now)
				mu.Unlock()
				return next(c)
			}

			if len(queue) >= config.MaxQueue {
				mu.Unlock()
				return c.JSON(http.StatusServiceUnavailable, map[string]string{
					"error": "server busy",
				})
			}

			ch := make(chan struct{})
			queue = append(queue, ch)
			mu.Unlock()

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
					mu.Unlock()
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
						mu.Unlock()
						return next(c)
					}
					mu.Unlock()
				}
			}

			return nil
		}
	}
}
