package model

import "time"

type Plan struct {
	ID                string    `gorm:"type:uuid;primaryKey" json:"id"`
	Code              string    `gorm:"size:40;not null;uniqueIndex" json:"code"`
	Name              string    `gorm:"size:80;not null" json:"name"`
	PriceCents        int64     `gorm:"not null" json:"price_cents"`
	Currency          string    `gorm:"size:8;not null" json:"currency"`
	StripePriceID     string    `gorm:"size:120;not null" json:"-"`
	MaxUsers          int64     `gorm:"not null" json:"max_users"`
	MaxRecords        int64     `gorm:"not null" json:"max_records"`
	MaxKnowledgeBytes int64     `gorm:"not null" json:"max_knowledge_bytes"`
	MaxAITokens       int64     `gorm:"not null" json:"max_ai_tokens"`
	Status            string    `gorm:"size:24;not null" json:"status"`
	CreatedAt         time.Time `gorm:"not null" json:"-"`
	UpdatedAt         time.Time `gorm:"not null" json:"-"`
}

func (Plan) TableName() string { return "plans" }

type Subscription struct {
	ID                     string    `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID               string    `gorm:"type:uuid;not null;uniqueIndex" json:"tenant_id"`
	PlanID                 string    `gorm:"type:uuid;not null" json:"plan_id"`
	Provider               string    `gorm:"size:24;not null" json:"provider"`
	ProviderCustomerID     string    `gorm:"size:120;not null" json:"-"`
	ProviderSubscriptionID string    `gorm:"size:120;not null;index" json:"-"`
	Status                 string    `gorm:"size:24;not null" json:"status"`
	PeriodStart            time.Time `gorm:"not null" json:"period_start"`
	PeriodEnd              time.Time `gorm:"not null" json:"period_end"`
	CreatedAt              time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt              time.Time `gorm:"not null" json:"updated_at"`
}

func (Subscription) TableName() string { return "subscriptions" }

type UsageCounter struct {
	TenantID    string    `gorm:"type:uuid;primaryKey"`
	Metric      string    `gorm:"size:40;primaryKey"`
	PeriodStart time.Time `gorm:"primaryKey"`
	Quantity    int64     `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

func (UsageCounter) TableName() string { return "usage_counters" }

type BillingEvent struct {
	ID          string    `gorm:"size:160;primaryKey"`
	Type        string    `gorm:"size:80;not null"`
	ProcessedAt time.Time `gorm:"not null"`
}

func (BillingEvent) TableName() string { return "billing_events" }

type BillingOverview struct {
	Plan         Plan             `json:"plan"`
	Subscription Subscription     `json:"subscription"`
	Usage        map[string]int64 `json:"usage"`
}
