package service

import (
	"context"
	"errors"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/repository"
)

type HealthService interface {
	Liveness() model.HealthStatus
	Readiness(ctx context.Context) (model.HealthStatus, error)
}

type healthService struct {
	repository repository.HealthRepository
	service    string
	version    string
}

func NewHealthService(repository repository.HealthRepository, serviceName, version string) HealthService {
	return &healthService{repository: repository, service: serviceName, version: version}
}

func (s *healthService) Liveness() model.HealthStatus {
	return model.HealthStatus{
		Status:    "ok",
		Service:   s.service,
		Version:   s.version,
		CheckedAt: time.Now().UTC(),
	}
}

func (s *healthService) Readiness(ctx context.Context) (model.HealthStatus, error) {
	status := model.HealthStatus{
		Status:    "ok",
		Service:   s.service,
		Version:   s.version,
		CheckedAt: time.Now().UTC(),
		Dependencies: map[string]string{
			"postgres": "ok",
			"redis":    "ok",
		},
	}

	var checks []error
	if err := s.repository.CheckPostgres(ctx); err != nil {
		status.Status = "degraded"
		status.Dependencies["postgres"] = "unavailable"
		checks = append(checks, err)
	}
	if err := s.repository.CheckRedis(ctx); err != nil {
		status.Status = "degraded"
		status.Dependencies["redis"] = "unavailable"
		checks = append(checks, err)
	}

	return status, errors.Join(checks...)
}
