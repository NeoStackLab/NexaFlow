package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
)

type billingRepositoryStub struct {
	overview model.BillingOverview
	allowed  bool
	metric   string
	quantity int64
}

func (*billingRepositoryStub) Plans(context.Context) ([]model.Plan, error) { return nil, nil }
func (s *billingRepositoryStub) Overview(context.Context, string) (model.BillingOverview, error) {
	return s.overview, nil
}
func (s *billingRepositoryStub) RecordUsage(_ context.Context, _, metric string, quantity, _ int64) (bool, error) {
	s.metric, s.quantity = metric, quantity
	return s.allowed, nil
}
func (*billingRepositoryStub) ApplyEvent(context.Context, string, string, string, string, string, string, time.Time, time.Time) error {
	return nil
}

type billingProviderStub struct{}

func (*billingProviderStub) Checkout(context.Context, string, model.Plan) (string, error) { return "", nil }
func (*billingProviderStub) ParseWebhook([]byte, string) (string, string, string, string, string, string, time.Time, time.Time, error) {
	return "", "", "", "", "", "", time.Time{}, time.Time{}, nil
}

func TestBillingConsumeEnforcesPlanQuota(t *testing.T) {
	repository := &billingRepositoryStub{overview: model.BillingOverview{Plan: model.Plan{MaxRecords: 10}}, allowed: false}
	service := NewBillingService(repository, &billingProviderStub{})
	err := service.Consume(context.Background(), "tenant-a", "records", 1)
	if !errors.Is(err, ErrQuotaExceeded) || repository.metric != "records" || repository.quantity != 1 {
		t.Fatalf("error=%v metric=%q quantity=%d", err, repository.metric, repository.quantity)
	}
}
