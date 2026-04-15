package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"os"

	"go-throttle/middleware"

	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	pgDSN := os.Getenv("POSTGRES_DSN")
	if pgDSN == "" {
		pgDSN = "host=host.docker.internal user=root password=root dbname=database port=5432 sslmode=disable"
	}

	sqlDB, err := sql.Open("postgres", pgDSN)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	registry := prometheus.NewRegistry()
	middleware.InitMetrics(registry)

	middleware.StartDBPoolMetricsCollector(sqlDB, 10*time.Second)

	e := echo.New()

	e.Use(middleware.HTTPMetrics())

	e.GET("/metrics", echo.WrapHandler(promhttp.HandlerFor(registry, promhttp.HandlerOpts{})))

	throttleInMem := middleware.InMemoryThrottle(middleware.InMemoryThrottleConfig{
		RateLimit:     40,
		WindowSeconds: 1,
		MaxQueue:      80,
	})

	throttleRedis := middleware.Throttle(middleware.ThrottleConfig{
		RedisClient:   rdb,
		RateLimit:     10,
		WindowSeconds: 5,
		MaxQueue:      20,
		KeyPrefix:     "throttle",
	})

	e.GET("/no-throttle", func(c echo.Context) error {
		data := make([]byte, 1024*1024)
		for i := range data {
			data[i] = 1
		}

		time.Sleep(500 * time.Millisecond)
		val := data[500000]

		return c.JSON(200, map[string]interface{}{
			"message":    "hello without throttle",
			"bytes_held": val,
		})
	})

	e.GET("/throttle", func(c echo.Context) error {
		data := make([]byte, 1024*1024)
		for i := range data {
			data[i] = 1
		}
		time.Sleep(500 * time.Millisecond)
		val := data[500000]

		return c.JSON(200, map[string]interface{}{
			"message":    "hello from in-memory throttle",
			"bytes_held": val,
		})
	}, throttleInMem)

	e.GET("/throttle-with-redis", func(c echo.Context) error {
		data := make([]byte, 1*1024*1024)
		time.Sleep(100 * time.Millisecond)
		_ = data[0]
		return c.JSON(200, map[string]string{
			"message": "hello from redis throttle",
		})
	}, throttleRedis)

	e.GET("/db-test", func(c echo.Context) error {
		start := time.Now()
		var version string
		err := sqlDB.QueryRow("SELECT version()").Scan(&version)
		middleware.TrackQuery("select_version", start, err)

		if err != nil {
			return c.JSON(500, map[string]string{
				"error": "Database query failed",
			})
		}

		return c.JSON(200, map[string]interface{}{
			"message": "Database connection successful",
			"version": version,
		})
	})

	log.Fatal(e.Start(fmt.Sprintf(":%d", 8000)))
}
