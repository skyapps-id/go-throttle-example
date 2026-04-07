package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
)

const allowScript = `
local key = KEYS[1]
local queue_key = KEYS[2]
local window_start = tonumber(ARGV[1])
local now = tonumber(ARGV[2])
local member = ARGV[3]
local rate_limit = tonumber(ARGV[4])
local max_queue = tonumber(ARGV[5])
local ttl = tonumber(ARGV[6])

redis.call('ZREMRANGEBYSCORE', key, 0, window_start)
local count = redis.call('ZCARD', key)

if count < rate_limit then
	redis.call('ZADD', key, now, member)
	redis.call('EXPIRE', key, ttl)
	return 0
end

local queue_len = redis.call('LLEN', queue_key)
if queue_len >= max_queue then
	return 2
end

redis.call('RPUSH', queue_key, member)
redis.call('EXPIRE', queue_key, ttl)
return 1
`

const dequeueScript = `
local key = KEYS[1]
local queue_key = KEYS[2]
local member = ARGV[1]
local now = tonumber(ARGV[2])
local new_member = ARGV[3]
local rate_limit = tonumber(ARGV[4])
local ttl = tonumber(ARGV[5])

local count = redis.call('ZCARD', key)
if count < rate_limit then
	redis.call('LREM', queue_key, 0, member)
	redis.call('ZADD', key, now, new_member)
	redis.call('EXPIRE', key, ttl)
	return 1
end
return 0
`

type ThrottleConfig struct {
	RedisClient   *redis.Client
	RateLimit     int
	WindowSeconds int
	MaxQueue      int
	KeyPrefix     string
}

func Throttle(config ThrottleConfig) echo.MiddlewareFunc {
	allowSha := config.loadScript(allowScript)
	dequeueSha := config.loadScript(dequeueScript)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			ctx := c.Request().Context()
			key := fmt.Sprintf("%s:%s", config.KeyPrefix, "global")
			queueKey := key + ":queue"
			member := strconv.FormatInt(time.Now().UnixNano(), 10)

			now := time.Now().UnixMilli()
			windowStart := now - int64(config.WindowSeconds)*1000
			ttl := config.WindowSeconds + 1

			result, err := config.RedisClient.EvalSha(ctx, allowSha, []string{key, queueKey},
				windowStart, now, member, config.RateLimit, config.MaxQueue, ttl,
			).Result()

			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{
					"error": "internal server error",
				})
			}

			code, _ := result.(int64)

			switch code {
			case 0:
				ThrottleRequestsTotal.WithLabelValues("redis", c.Request().Method, c.Path(), "allowed").Inc()
				ThrottleRequestDuration.WithLabelValues("redis", c.Request().Method, c.Path(), "allowed").Observe(time.Since(start).Seconds())
				return next(c)
			case 2:
				ThrottleRequestsTotal.WithLabelValues("redis", c.Request().Method, c.Path(), "rejected").Inc()
				return c.JSON(http.StatusServiceUnavailable, map[string]string{
					"error": "server busy",
				})
			}

			ThrottleRequestsTotal.WithLabelValues("redis", c.Request().Method, c.Path(), "queued").Inc()

			queueLen, _ := config.RedisClient.LLen(ctx, queueKey).Result()
			ThrottleQueueLength.WithLabelValues("redis", c.Request().Method, c.Path()).Set(float64(queueLen))

			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			for range ticker.C {
				select {
				case <-ctx.Done():
					config.RedisClient.LRem(ctx, queueKey, 0, member)
					ThrottleRequestsTotal.WithLabelValues("redis", c.Request().Method, c.Path(), "timeout").Inc()

					queueLen, _ := config.RedisClient.LLen(ctx, queueKey).Result()
					ThrottleQueueLength.WithLabelValues("redis", c.Request().Method, c.Path()).Set(float64(queueLen))

					return c.JSON(http.StatusRequestTimeout, map[string]string{
						"error": "request timeout",
					})
				default:
					newMember := strconv.FormatInt(time.Now().UnixNano(), 10)
					got, err := config.RedisClient.EvalSha(ctx, dequeueSha, []string{key, queueKey},
						member, time.Now().UnixMilli(), newMember, config.RateLimit, ttl,
					).Result()
					if err == nil {
						if v, _ := got.(int64); v == 1 {
							ThrottleRequestDuration.WithLabelValues("redis", c.Request().Method, c.Path(), "queued").Observe(time.Since(start).Seconds())

							queueLen, _ := config.RedisClient.LLen(ctx, queueKey).Result()
							ThrottleQueueLength.WithLabelValues("redis", c.Request().Method, c.Path()).Set(float64(queueLen))

							return next(c)
						}
					}
				}
			}

			return nil
		}
	}
}

func (config ThrottleConfig) loadScript(script string) string {
	sha, err := config.RedisClient.ScriptLoad(context.Background(), script).Result()
	if err != nil {
		panic("failed to load script: " + err.Error())
	}
	return sha
}
