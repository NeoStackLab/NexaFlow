package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
)

func signedStripeEvent(secret string, timestamp time.Time, eventType string) ([]byte, string) {
	body := []byte(fmt.Sprintf(`{"id":"evt_1","type":%q,"data":{"object":{"id":"sub_1","customer":"cus_1","current_period_start":1700000000,"current_period_end":1702592000,"metadata":{"tenant_id":"11111111-1111-1111-1111-111111111111"},"items":{"data":[{"price":{"id":"price_pro"}}]}}}}`, eventType))
	t := fmt.Sprintf("%d", timestamp.Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(t + "." + string(body)))
	return body, "t=" + t + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}

func TestStripeWebhookVerificationAndMapping(t *testing.T) {
	provider := &stripeProvider{cfg: config.BillingConfig{StripeWebhookSecret: "whsec_test"}}
	body, signature := signedStripeEvent("whsec_test", time.Now(), "customer.subscription.updated")
	id, eventType, tenant, customer, subscription, price, _, _, err := provider.ParseWebhook(body, signature)
	if err != nil {
		t.Fatalf("ParseWebhook() error = %v", err)
	}
	if id != "evt_1" || eventType != "customer.subscription.updated" || tenant == "" || customer != "cus_1" || subscription != "sub_1" || price != "price_pro" {
		t.Fatalf("unexpected mapping: %q %q %q %q %q %q", id, eventType, tenant, customer, subscription, price)
	}
}

func TestStripeWebhookRejectsInvalidReplayWindowsAndEvents(t *testing.T) {
	provider := &stripeProvider{cfg: config.BillingConfig{StripeWebhookSecret: "whsec_test"}}
	for name, testCase := range map[string]struct {
		timestamp time.Time
		eventType string
	}{
		"expired":     {time.Now().Add(-6 * time.Minute), "customer.subscription.updated"},
		"future":      {time.Now().Add(6 * time.Minute), "customer.subscription.updated"},
		"unsupported": {time.Now(), "invoice.paid"},
	} {
		body, signature := signedStripeEvent("whsec_test", testCase.timestamp, testCase.eventType)
		if _, _, _, _, _, _, _, _, err := provider.ParseWebhook(body, signature); err == nil {
			t.Fatalf("%s webhook was accepted", name)
		}
	}
}
