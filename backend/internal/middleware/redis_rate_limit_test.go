package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func TestRedisRateLimitIsSharedAcrossRouters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	request := func() int {
		router := gin.New()
		router.Use(RedisRateLimit(client, "test", 1, time.Minute))
		router.GET("/login", func(c *gin.Context) { c.Status(http.StatusNoContent) })
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/login", nil))
		return response.Code
	}
	if status := request(); status != http.StatusNoContent {
		t.Fatalf("first status = %d", status)
	}
	if status := request(); status != http.StatusTooManyRequests {
		t.Fatalf("second status = %d", status)
	}
}
