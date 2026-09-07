package service

import (
	"context"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/repository"
)

type AuditService interface {
	Record(context.Context, model.AuditEvent) error
	List(context.Context, string, int) ([]model.AuditEvent, error)
}

type auditService struct{ repository repository.AuditRepository }

func NewAuditService(repo repository.AuditRepository) AuditService {
	return &auditService{repository: repo}
}

func (s *auditService) Record(ctx context.Context, event model.AuditEvent) error {
	return s.repository.Create(ctx, event)
}

func (s *auditService) List(ctx context.Context, tenantID string, limit int) ([]model.AuditEvent, error) {
	return s.repository.List(ctx, tenantID, limit)
}
