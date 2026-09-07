package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type stripeProvider struct {
	cfg    config.BillingConfig
	client *http.Client
}

func NewBillingProvider(c config.BillingConfig) BillingProvider {
	return &stripeProvider{c, &http.Client{Timeout: 30 * time.Second}}
}
func (p *stripeProvider) Checkout(ctx context.Context, tenant string, plan model.Plan) (string, error) {
	if p.cfg.StripeSecretKey == "" || plan.StripePriceID == "" {
		return "", ErrBillingUnavailable
	}
	v := url.Values{"mode": {"subscription"}, "success_url": {p.cfg.SuccessURL}, "cancel_url": {p.cfg.CancelURL}, "client_reference_id": {tenant}, "line_items[0][price]": {plan.StripePriceID}, "line_items[0][quantity]": {"1"}, "metadata[tenant_id]": {tenant}, "subscription_data[metadata][tenant_id]": {tenant}}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.stripe.com/v1/checkout/sessions", strings.NewReader(v.Encode()))
	if e != nil {
		return "", e
	}
	req.SetBasicAuth(p.cfg.StripeSecretKey, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, e := p.client.Do(req)
	if e != nil {
		return "", e
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("billing provider returned HTTP %d", resp.StatusCode)
	}
	var out struct {
		URL string `json:"url"`
	}
	if e = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); e != nil {
		return "", e
	}
	if out.URL == "" {
		return "", errors.New("billing provider returned empty checkout URL")
	}
	return out.URL, nil
}
func (p *stripeProvider) ParseWebhook(body []byte, sig string) (string, string, string, string, string, string, time.Time, time.Time, error) {
	if p.cfg.StripeWebhookSecret == "" {
		return "", "", "", "", "", "", time.Time{}, time.Time{}, ErrBillingUnavailable
	}
	timestamp, signature := parseStripeSignature(sig)
	if timestamp == "" || signature == "" {
		return "", "", "", "", "", "", time.Time{}, time.Time{}, errors.New("invalid Stripe signature")
	}
	ts, e := strconv.ParseInt(timestamp, 10, 64)
	if e != nil {
		return "", "", "", "", "", "", time.Time{}, time.Time{}, errors.New("invalid Stripe signature timestamp")
	}
	age := time.Since(time.Unix(ts, 0))
	if age > 5*time.Minute || age < -5*time.Minute {
		return "", "", "", "", "", "", time.Time{}, time.Time{}, errors.New("expired Stripe signature")
	}
	mac := hmac.New(sha256.New, []byte(p.cfg.StripeWebhookSecret))
	mac.Write([]byte(timestamp + "." + string(body)))
	expected := mac.Sum(nil)
	actual, e := hex.DecodeString(signature)
	if e != nil || !hmac.Equal(expected, actual) {
		return "", "", "", "", "", "", time.Time{}, time.Time{}, errors.New("invalid Stripe signature")
	}
	var event struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Data struct {
			Object struct {
				ID                 string            `json:"id"`
				Customer           string            `json:"customer"`
				Subscription       string            `json:"subscription"`
				CurrentPeriodStart int64             `json:"current_period_start"`
				CurrentPeriodEnd   int64             `json:"current_period_end"`
				Metadata           map[string]string `json:"metadata"`
				Items              struct {
					Data []struct {
						Price struct {
							ID string `json:"id"`
						} `json:"price"`
					} `json:"data"`
				} `json:"items"`
			} `json:"object"`
		} `json:"data"`
	}
	if e = json.Unmarshal(body, &event); e != nil {
		return "", "", "", "", "", "", time.Time{}, time.Time{}, e
	}
	switch event.Type {
	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
	default:
		return "", "", "", "", "", "", time.Time{}, time.Time{}, errors.New("unsupported Stripe event type")
	}
	tenant := event.Data.Object.Metadata["tenant_id"]
	sub := event.Data.Object.ID
	if event.ID == "" || tenant == "" || event.Data.Object.Customer == "" || sub == "" {
		return "", "", "", "", "", "", time.Time{}, time.Time{}, errors.New("billing event missing tenant metadata")
	}
	priceID := ""
	if len(event.Data.Object.Items.Data) > 0 {
		priceID = event.Data.Object.Items.Data[0].Price.ID
	}
	if priceID == "" {
		return "", "", "", "", "", "", time.Time{}, time.Time{}, errors.New("billing event missing subscription price")
	}
	return event.ID, event.Type, tenant, event.Data.Object.Customer, sub, priceID, time.Unix(event.Data.Object.CurrentPeriodStart, 0).UTC(), time.Unix(event.Data.Object.CurrentPeriodEnd, 0).UTC(), nil
}
func parseStripeSignature(h string) (string, string) {
	var t, v string
	for _, part := range strings.Split(h, ",") {
		pair := strings.SplitN(part, "=", 2)
		if len(pair) != 2 {
			continue
		}
		if pair[0] == "t" {
			t = pair[1]
		}
		if pair[0] == "v1" && v == "" {
			v = pair[1]
		}
	}
	return t, v
}
