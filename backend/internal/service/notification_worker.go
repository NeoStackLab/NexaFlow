package service

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
	"github.com/NeoStackLab/NexaFlow/backend/internal/repository"
	"github.com/NeoStackLab/NexaFlow/backend/internal/tenantcontext"
	"go.uber.org/zap"
)

type NotificationWorker struct {
	repository repository.NotificationOutboxRepository
	config     config.NotificationConfig
	client     *http.Client
	log        *zap.Logger
}

func NewNotificationWorker(repo repository.NotificationOutboxRepository, cfg config.NotificationConfig, log *zap.Logger) *NotificationWorker {
	return &NotificationWorker{
		repository: repo,
		config:     cfg,
		client: &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		}, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }},
		log: log,
	}
}

func (w *NotificationWorker) ProcessBatch(ctx context.Context) error {
	limit := w.config.BatchSize
	if limit <= 0 {
		limit = 25
	}
	notifications, err := w.repository.ClaimPending(tenantcontext.WithSystem(ctx), limit, 5*time.Minute)
	if err != nil {
		w.log.Error("notification_outbox_claim_failed", zap.Error(err))
		return err
	}
	for _, notification := range notifications {
		notificationContext := tenantcontext.With(ctx, tenantcontext.Scope{TenantID: notification.TenantID, UserID: "notification-worker"})
		if err := w.deliver(notificationContext, notification); err != nil {
			reason := truncateError(err)
			markErr := error(nil)
			if w.config.MaxAttempts > 0 && notification.AttemptCount >= w.config.MaxAttempts {
				markErr = w.repository.MarkDeadLetter(notificationContext, notification.ID, reason)
			} else {
				markErr = w.repository.MarkFailed(notificationContext, notification.ID, reason, w.nextAttempt(notification.AttemptCount))
			}
			if markErr != nil {
				w.log.Error("notification_outbox_mark_failed", zap.String("notification_id", notification.ID), zap.Error(markErr))
			}
			w.log.Warn("notification_delivery_failed", zap.String("notification_id", notification.ID), zap.String("channel", notification.Channel), zap.Error(err))
			continue
		}
		if err := w.repository.MarkDelivered(notificationContext, notification.ID, time.Now().UTC()); err != nil {
			w.log.Error("notification_outbox_mark_delivered_failed", zap.String("notification_id", notification.ID), zap.Error(err))
		}
	}
	return nil
}

// process remains as a narrow compatibility helper for package-level tests.
func (w *NotificationWorker) process(ctx context.Context) { _ = w.ProcessBatch(ctx) }

func (w *NotificationWorker) deliver(ctx context.Context, notification model.Notification) error {
	switch notification.Channel {
	case "inapp":
		return nil
	case "webhook":
		return w.deliverWebhook(ctx, notification)
	case "email":
		return w.deliverEmail(notification)
	default:
		return fmt.Errorf("unsupported notification channel %q", notification.Channel)
	}
}

func (w *NotificationWorker) deliverWebhook(ctx context.Context, notification model.Notification) error {
	target, err := url.Parse(strings.TrimSpace(notification.Recipient))
	if err != nil || target.Scheme == "" || target.Hostname() == "" {
		return errors.New("webhook recipient must be a valid URL")
	}
	if target.Scheme != "https" {
		return errors.New("webhook recipient must use HTTPS")
	}
	if !webhookAllowed(target.Hostname(), w.config.WebhookAllowlist) {
		return errors.New("webhook host is not allowlisted")
	}
	if err := publicWebhookHost(ctx, target.Hostname()); err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{
		"id": notification.ID, "tenant_id": notification.TenantID, "instance_id": notification.InstanceID,
		"subject": notification.Subject, "body": notification.Body, "created_at": notification.CreatedAt,
	})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "NexaFlow-Notification-Worker/1")
	response, err := w.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("webhook returned HTTP %d", response.StatusCode)
	}
	return nil
}

func (w *NotificationWorker) deliverEmail(notification model.Notification) error {
	if strings.TrimSpace(w.config.SMTPHost) == "" || w.config.SMTPPort <= 0 || strings.TrimSpace(w.config.SMTPFrom) == "" {
		return errors.New("SMTP is not configured")
	}
	recipients := strings.FieldsFunc(notification.Recipient, func(r rune) bool { return r == ',' || r == ';' || r == ' ' })
	if len(recipients) == 0 {
		return errors.New("email notification has no recipients")
	}
	message := "From: " + w.config.SMTPFrom + "\r\n" + "To: " + strings.Join(recipients, ", ") + "\r\n" + "Subject: " + notification.Subject + "\r\n\r\n" + notification.Body
	address := net.JoinHostPort(w.config.SMTPHost, strconv.Itoa(w.config.SMTPPort))
	var auth smtp.Auth
	if w.config.SMTPUsername != "" {
		auth = smtp.PlainAuth("", w.config.SMTPUsername, w.config.SMTPPassword, w.config.SMTPHost)
	}
	return smtp.SendMail(address, auth, w.config.SMTPFrom, recipients, []byte(message))
}

func (w *NotificationWorker) nextAttempt(attempt int) *time.Time {
	if attempt < 1 {
		attempt = 1
	}
	backoff := time.Duration(1<<min(attempt-1, 6)) * time.Minute
	next := time.Now().UTC().Add(backoff)
	return &next
}

func webhookAllowed(host string, allowlist []string) bool {
	if len(allowlist) == 0 {
		return false
	}
	host = strings.ToLower(strings.TrimSpace(host))
	if net.ParseIP(host) != nil {
		return false
	}
	for _, allowed := range allowlist {
		allowed = strings.ToLower(strings.TrimSpace(allowed))
		if allowed != "" && host == allowed {
			return true
		}
	}
	return false
}

func publicWebhookHost(ctx context.Context, host string) error {
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return errors.New("webhook host cannot resolve to localhost")
	}
	addresses, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return errors.New("webhook host DNS lookup failed")
	}
	if len(addresses) == 0 {
		return errors.New("webhook host has no address")
	}
	for _, address := range addresses {
		if address.IsLoopback() || address.IsPrivate() || address.IsLinkLocalUnicast() || address.IsUnspecified() {
			return errors.New("webhook host resolves to a private address")
		}
	}
	return nil
}

func truncateError(err error) string {
	message := err.Error()
	if len(message) > 1000 {
		return message[:1000]
	}
	return message
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
