package middleware

import (
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

func HTTPMetrics() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			status := strconv.Itoa(c.Response().Status)
			RequestsTotal.WithLabelValues(c.Request().Method, c.Path(), status).Inc()
			RequestsDuration.WithLabelValues(c.Request().Method, c.Path(), status).Observe(time.Since(start).Seconds())
			return err
		}
	}
}
