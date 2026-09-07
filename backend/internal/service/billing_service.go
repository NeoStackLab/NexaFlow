package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/tenantcontext"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var ErrBillingUnavailable = errors.New("billing provider is not configured")
var ErrQuotaExceeded = errors.New("tenant quota exceeded")

type BillingRepository interface {
	Plans(context.Context) ([]model.Plan, error)
	Overview(context.Context, string) (model.BillingOverview, error)
	RecordUsage(context.Context, string, string, int64, int64) (bool, error)
	ApplyEvent(context.Context, string, string, string, string, string, string, time.Time, time.Time) error
}
type BillingProvider interface {
	Checkout(context.Context, string, model.Plan) (string, error)
	ParseWebhook([]byte, string) (string, string, string, string, string, string, time.Time, time.Time, error)
}
type BillingService interface {
	Plans(context.Context) ([]model.Plan, error)
	Overview(context.Context, string) (model.BillingOverview, error)
	Checkout(context.Context, string, string) (string, error)
	Webhook(context.Context, []byte, string) error
	Consume(context.Context, string, string, int64) error
}
type UsageMeter interface {
	Consume(context.Context, string, string, int64) error
}
type billingService struct {
	repository BillingRepository
	provider   BillingProvider
	denials    metric.Int64Counter
}

func NewBillingService(r BillingRepository, p BillingProvider) BillingService {
	denials, _ := otel.Meter("nexaflow/billing").Int64Counter("nexaflow.quota.denials")
	return &billingService{repository: r, provider: p, denials: denials}
}
func (s *billingService) Plans(c context.Context) ([]model.Plan, error) { return s.repository.Plans(c) }
func (s *billingService) Overview(c context.Context, t string) (model.BillingOverview, error) {
	var err error
	if t, err = billingTenant(c, t); err != nil {
		return model.BillingOverview{}, err
	}
	return s.repository.Overview(c, t)
}
func (s *billingService) Checkout(c context.Context, t, code string) (string, error) {
	var err error
	if t, err = billingTenant(c, t); err != nil {
		return "", err
	}
	plans, e := s.repository.Plans(c)
	if e != nil {
		return "", e
	}
	for _, p := range plans {
		if p.Code == code && p.PriceCents > 0 {
			return s.provider.Checkout(c, t, p)
		}
	}
	return "", fmt.Errorf("invalid paid plan")
}
func (s *billingService) Webhook(c context.Context, b []byte, sig string) error {
	id, typ, t, cus, sub, priceID, start, end, e := s.provider.ParseWebhook(b, sig)
	if e != nil {
		return e
	}
	webhookContext := tenantcontext.With(c, tenantcontext.Scope{TenantID: t, UserID: "billing-webhook"})
	return s.repository.ApplyEvent(webhookContext, id, typ, t, cus, sub, priceID, start, end)
}
func (s *billingService) Consume(c context.Context, t, m string, q int64) error {
	var err error
	if t, err = billingTenant(c, t); err != nil {
		return err
	}
	o, e := s.repository.Overview(c, t)
	if e != nil {
		return e
	}
	limits := map[string]int64{"users": o.Plan.MaxUsers, "records": o.Plan.MaxRecords, "knowledge_bytes": o.Plan.MaxKnowledgeBytes, "ai_tokens": o.Plan.MaxAITokens}
	limit, ok := limits[m]
	if !ok {
		return fmt.Errorf("unknown metric")
	}
	allowed, err := s.repository.RecordUsage(c, t, m, q, limit)
	if err != nil {
		return err
	}
	if !allowed {
		s.denials.Add(c, 1, metric.WithAttributes(attribute.String("quota.metric", m)))
		return ErrQuotaExceeded
	}
	return nil
}

func billingTenant(ctx context.Context, requested string) (string, error) {
	scope, ok := tenantcontext.From(ctx)
	if !ok {
		return "", tenantcontext.ErrRequired
	}
	if scope.System {
		if requested == "" {
			return "", tenantcontext.ErrRequired
		}
		return requested, nil
	}
	verified, err := tenantcontext.RequireTenant(ctx, requested)
	if err != nil {
		return "", err
	}
	return verified.TenantID, nil
}
