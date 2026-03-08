package middleware

import (
	"strconv"
	"time"

	"go-tempo/internal/metrics"

	"github.com/gin-gonic/gin"
)

// PrometheusMiddleware returns a Gin middleware that records HTTP metrics
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		// Record request size
		if c.Request.ContentLength > 0 {
			metrics.HTTPRequestSizeBytes.WithLabelValues(
				c.Request.Method,
				path,
			).Observe(float64(c.Request.ContentLength))
		}

		// Process request
		c.Next()

		// Record metrics after request is processed
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		metrics.HTTPRequestsTotal.WithLabelValues(
			c.Request.Method,
			path,
			status,
		).Inc()

		metrics.HTTPRequestDuration.WithLabelValues(
			c.Request.Method,
			path,
		).Observe(duration)

		metrics.HTTPResponseSizeBytes.WithLabelValues(
			c.Request.Method,
			path,
		).Observe(float64(c.Writer.Size()))
	}
}
