package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRateLimitAbortsAfterLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RateLimit(1, time.Minute))
	router.GET("/login", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/login", nil))
	second := httptest.NewRecorder()
	router.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/login", nil))

	if first.Code != http.StatusNoContent || second.Code != http.StatusTooManyRequests {
		t.Fatalf("status codes: first=%d second=%d", first.Code, second.Code)
	}
	if second.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header")
	}
}

func TestSecurityHeadersAddsHSTSOnlyInProduction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		production   bool
		expectedHSTS bool
	}{
		{production: false, expectedHSTS: false},
		{production: true, expectedHSTS: true},
	} {
		router := gin.New()
		router.Use(SecurityHeaders(test.production))
		router.GET("/health", func(c *gin.Context) { c.Status(http.StatusNoContent) })
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
		if (recorder.Header().Get("Strict-Transport-Security") != "") != test.expectedHSTS {
			t.Fatalf("production=%v HSTS=%q", test.production, recorder.Header().Get("Strict-Transport-Security"))
		}
		if recorder.Header().Get("X-Content-Type-Options") != "nosniff" || recorder.Header().Get("X-Frame-Options") != "DENY" {
			t.Fatalf("security headers=%v", recorder.Header())
		}
	}
}
