package middleware

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/venkat/livepolls/backend/internal/realtime"
)

// CORS is written by hand rather than pulled from a library so the allowed
// origins are an explicit whitelist, never a reflected "*".
func CORS(allowed []string) gin.HandlerFunc {
	set := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		set[strings.TrimRight(o, "/")] = struct{}{}
	}
	return func(c *gin.Context) {
		origin := strings.TrimRight(c.GetHeader("Origin"), "/")
		if _, ok := set[origin]; ok {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Max-Age", "600")
			c.Header("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Next()
	}
}

// RateLimit buckets by route name plus client IP, backed by Redis so the limit
// holds across every instance rather than per-process.
func RateLimit(rt *realtime.Service, name string, limit int64, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ok, err := rt.Allow(c.Request.Context(), name+":"+c.ClientIP(), limit, window)
		if err != nil {
			log.Printf("ratelimit: %v", err)
		}
		if !ok {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "too many requests, wait a moment and try again",
			})
			return
		}
		c.Next()
	}
}

// BodyLimit caps request bodies before Gin ever tries to decode them.
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}
