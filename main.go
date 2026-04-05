package main

import (
	"fmt"
	"log"
	"time"

	"os"

	"go-throttle/middleware"

	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
)

func main() {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	e := echo.New()

	throttleInMem := middleware.InMemoryThrottle(middleware.InMemoryThrottleConfig{
		RateLimit:     10,
		WindowSeconds: 5,
		MaxQueue:      20,
	})

	throttleRedis := middleware.Throttle(middleware.ThrottleConfig{
		RedisClient:   rdb,
		RateLimit:     10,
		WindowSeconds: 5,
		MaxQueue:      20,
		KeyPrefix:     "throttle",
	})

	e.GET("/no-throttle", func(c echo.Context) error {
		data := make([]byte, 10*1024*1024)
		time.Sleep(100 * time.Millisecond)
		_ = data[0]
		return c.JSON(200, map[string]string{
			"message": "hello without throttle",
		})
	})

	e.GET("/throttle", func(c echo.Context) error {
		data := make([]byte, 10*1024*1024)
		time.Sleep(100 * time.Millisecond)
		_ = data[0]
		return c.JSON(200, map[string]string{
			"message": "hello from in-memory throttle",
		})
	}, throttleInMem)

	e.GET("/throttle-with-redis", func(c echo.Context) error {
		data := make([]byte, 10*1024*1024)
		time.Sleep(100 * time.Millisecond)
		_ = data[0]
		return c.JSON(200, map[string]string{
			"message": "hello from redis throttle",
		})
	}, throttleRedis)

	log.Fatal(e.Start(fmt.Sprintf(":%d", 8080)))
}
