package server

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// requestLogger replaces gin.Logger with one structured slog line per
// request. Level follows the response status (5xx → error, 4xx → warn);
// successful health-check probes are demoted to debug to keep logs quiet.
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		status := c.Writer.Status()
		attrs := []any{
			"request_id", RequestID(c),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, "errors", c.Errors.String())
		}

		switch {
		case status >= 500:
			slog.Error("http request", attrs...)
		case status >= 400:
			slog.Warn("http request", attrs...)
		case c.Request.URL.Path == "/api/health":
			slog.Debug("http request", attrs...)
		default:
			slog.Info("http request", attrs...)
		}
	}
}
