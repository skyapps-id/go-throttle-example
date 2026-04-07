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

			RequestsInFlight.WithLabelValues(c.Request().Method, c.Path()).Inc()
			defer RequestsInFlight.WithLabelValues(c.Request().Method, c.Path()).Dec()

			err := next(c)
			status := c.Response().Status
			statusStr := strconv.Itoa(status)

			RequestsTotal.WithLabelValues(c.Request().Method, c.Path(), statusStr).Inc()
			RequestsDuration.WithLabelValues(c.Request().Method, c.Path(), statusStr).Observe(time.Since(start).Seconds())

			ResponseSizeBytes.WithLabelValues(c.Request().Method, c.Path()).Observe(float64(c.Response().Size))

			if status >= 500 {
				HTTPErrorsTotal.WithLabelValues(c.Request().Method, c.Path(), statusStr).Inc()
			}

			return err
		}
	}
}
