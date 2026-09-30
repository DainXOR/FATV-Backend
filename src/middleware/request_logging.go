package middleware

import (
	"strings"
	"time"

	"dainxor/atv/logger"

	"github.com/gin-gonic/gin"
)

// RequestLogger intentionally excludes query strings, headers, bodies, and invitation tokens.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		path := c.Request.URL.Path
		if strings.Contains(path, "/public/forms/") {
			parts := strings.Split(path, "/")
			for i := range parts {
				if parts[i] == "forms" && i+1 < len(parts) {
					parts[i+1] = "[token]"
					break
				}
			}
			path = strings.Join(parts, "/")
		}
		logger.Infof("HTTP %s %s status=%d latency=%s",
			c.Request.Method, path, c.Writer.Status(), time.Since(start).Round(time.Millisecond))
	}
}
