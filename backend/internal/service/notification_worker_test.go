package service

import (
	"context"
	"testing"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
	"go.uber.org/zap"
)

type notificationOutboxStub struct {
	claimed    []model.Notification
	marked     string
	deadReason string
}

func (s *notificationOutboxStub) ClaimPending(context.Context, int, time.Duration) ([]model.Notification, error) {
	return s.claimed, nil
}
func (s *notificationOutboxStub) MarkDelivered(_ context.Context, id string, _ time.Time) error {
	s.marked = "delivered:" + id
	return nil
}
func (s *notificationOutboxStub) MarkFailed(_ context.Context, id, _ string, _ *time.Time) error {
	s.marked = "failed:" + id
	return nil
}
func (s *notificationOutboxStub) MarkDeadLetter(_ context.Context, id, reason string) error {
	s.marked = "dead:" + id
	s.deadReason = reason
	return nil
}

func TestWebhookAllowedRequiresExactNonIPHost(t *testing.T) {
	if !webhookAllowed("hooks.example.com", []string{" hooks.example.com "}) {
		t.Fatal("expected exact allowlisted host")
	}
	for _, host := range []string{"sub.hooks.example.com", "127.0.0.1", "hooks.example.com."} {
		if webhookAllowed(host, []string{"hooks.example.com"}) {
			t.Fatalf("host %q should not be allowed", host)
		}
	}
}

func TestNotificationWorkerDeadLettersAfterMaxAttempts(t *testing.T) {
	repository := &notificationOutboxStub{claimed: []model.Notification{{ID: "notification-1", Channel: "unknown", AttemptCount: 2}}}
	worker := NewNotificationWorker(repository, config.NotificationConfig{BatchSize: 1, MaxAttempts: 2}, zap.NewNop())
	worker.process(context.Background())
	if repository.marked != "dead:notification-1" || repository.deadReason == "" {
		t.Fatalf("marked=%q reason=%q", repository.marked, repository.deadReason)
	}
}

func TestNotificationWorkerRetriesBeforeMaxAttempts(t *testing.T) {
	repository := &notificationOutboxStub{claimed: []model.Notification{{ID: "notification-2", Channel: "unknown", AttemptCount: 1}}}
	worker := NewNotificationWorker(repository, config.NotificationConfig{BatchSize: 1, MaxAttempts: 3}, zap.NewNop())
	worker.process(context.Background())
	if repository.marked != "failed:notification-2" {
		t.Fatalf("marked=%q", repository.marked)
	}
}
