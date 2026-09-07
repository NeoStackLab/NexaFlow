package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/httpx"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type rateLimitBucket struct {
	started time.Time
	count   int
}

var redisRateLimitScript = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[1]) end
local ttl = redis.call('PTTL', KEYS[1])
return {count, ttl}
`)

// RedisRateLimit provides one atomic fixed-window budget shared by every API
// replica. Redis failures fail closed for protected authentication endpoints.
func RedisRateLimit(client redis.UniversalClient, namespace string, limit int, window time.Duration) gin.HandlerFunc {
	if limit < 1 {
		limit = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	return func(c *gin.Context) {
		key := namespace + ":rate-limit:" + c.ClientIP() + ":" + c.Request.URL.Path
		result, err := redisRateLimitScript.Run(c.Request.Context(), client, []string{key}, window.Milliseconds()).Int64Slice()
		if err != nil || len(result) != 2 {
			httpx.Error(c, http.StatusServiceUnavailable, 3008, "rate limiter unavailable", nil)
			c.Abort()
			return
		}
		remaining := max(int64(limit)-result[0], 0)
		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
		if result[0] > int64(limit) {
			retryAfter := max((result[1]+999)/1000, 1)
			c.Header("Retry-After", strconv.FormatInt(retryAfter, 10))
			httpx.Error(c, http.StatusTooManyRequests, 3007, "too many requests", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}

// RateLimit provides a bounded process-local guard for authentication and
// bootstrap endpoints. A distributed limiter can replace this without
// changing route wiring when multiple API replicas are introduced.
func RateLimit(limit int, window time.Duration) gin.HandlerFunc {
	if limit < 1 {
		limit = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	var mu sync.Mutex
	buckets := map[string]rateLimitBucket{}
	return func(c *gin.Context) {
		now := time.Now()
		key := c.ClientIP() + "\x00" + c.Request.URL.Path
		mu.Lock()
		bucket := buckets[key]
		if bucket.started.IsZero() || now.Sub(bucket.started) >= window {
			bucket = rateLimitBucket{started: now}
		}
		bucket.count++
		buckets[key] = bucket
		if len(buckets) > 10000 {
			for candidate, item := range buckets {
				if now.Sub(item.started) >= window {
					delete(buckets, candidate)
				}
			}
		}
		allowed := bucket.count <= limit
		mu.Unlock()
		if !allowed {
			c.Header("Retry-After", "60")
			httpx.Error(c, http.StatusTooManyRequests, 3007, "too many requests", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}

func SecurityHeaders(production bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if production {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}
