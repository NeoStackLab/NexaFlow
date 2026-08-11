package service

import (
	"context"
	"errors"
	"testing"
)

type healthRepositoryStub struct {
	postgresError error
	redisError    error
}

func (s healthRepositoryStub) CheckPostgres(context.Context) error { return s.postgresError }
func (s healthRepositoryStub) CheckRedis(context.Context) error    { return s.redisError }

func TestHealthServiceReadiness(t *testing.T) {
	tests := []struct {
		name         string
		repository   healthRepositoryStub
		wantStatus   string
		wantPostgres string
		wantRedis    string
		wantError    bool
	}{
		{
			name:         "all dependencies available",
			repository:   healthRepositoryStub{},
			wantStatus:   "ok",
			wantPostgres: "ok",
			wantRedis:    "ok",
		},
		{
			name:         "postgres unavailable",
			repository:   healthRepositoryStub{postgresError: errors.New("postgres unavailable")},
			wantStatus:   "degraded",
			wantPostgres: "unavailable",
			wantRedis:    "ok",
			wantError:    true,
		},
		{
			name:         "redis unavailable",
			repository:   healthRepositoryStub{redisError: errors.New("redis unavailable")},
			wantStatus:   "degraded",
			wantPostgres: "ok",
			wantRedis:    "unavailable",
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewHealthService(tt.repository, "NexaFlow API", "test")
			status, err := service.Readiness(context.Background())

			if (err != nil) != tt.wantError {
				t.Fatalf("Readiness() error = %v, wantError %v", err, tt.wantError)
			}
			if status.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", status.Status, tt.wantStatus)
			}
			if status.Dependencies["postgres"] != tt.wantPostgres {
				t.Errorf("postgres = %q, want %q", status.Dependencies["postgres"], tt.wantPostgres)
			}
			if status.Dependencies["redis"] != tt.wantRedis {
				t.Errorf("redis = %q, want %q", status.Dependencies["redis"], tt.wantRedis)
			}
		})
	}
}
