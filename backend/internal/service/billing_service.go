package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"time"
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
}

func NewBillingService(r BillingRepository, p BillingProvider) BillingService {
	return &billingService{r, p}
}
func (s *billingService) Plans(c context.Context) ([]model.Plan, error) { return s.repository.Plans(c) }
func (s *billingService) Overview(c context.Context, t string) (model.BillingOverview, error) {
	return s.repository.Overview(c, t)
}
func (s *billingService) Checkout(c context.Context, t, code string) (string, error) {
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
	return s.repository.ApplyEvent(c, id, typ, t, cus, sub, priceID, start, end)
}
func (s *billingService) Consume(c context.Context, t, m string, q int64) error {
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
		return ErrQuotaExceeded
	}
	return nil
}
