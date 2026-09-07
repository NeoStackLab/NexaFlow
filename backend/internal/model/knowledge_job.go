package model

import "time"

type KnowledgeIngestionJob struct {
	ID            string     `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID      string     `gorm:"type:uuid;not null;index:idx_knowledge_jobs_tenant_status" json:"-"`
	CreatedBy     string     `gorm:"type:uuid;not null" json:"created_by"`
	Name          string     `gorm:"size:255;not null" json:"name"`
	ContentType   string     `gorm:"size:120;not null" json:"content_type"`
	Size          int64      `gorm:"not null" json:"size"`
	StorageKey    string     `gorm:"size:500;not null" json:"-"`
	Status        string     `gorm:"size:24;not null;index:idx_knowledge_jobs_tenant_status" json:"status"`
	AttemptCount  int        `gorm:"not null" json:"attempt_count"`
	NextAttemptAt *time.Time `json:"next_attempt_at,omitempty"`
	ClaimedAt     *time.Time `json:"-"`
	DocumentID    string     `gorm:"type:uuid;default:''" json:"document_id,omitempty"`
	Error         string     `gorm:"size:1000;not null;default:''" json:"error,omitempty"`
	CreatedAt     time.Time  `gorm:"not null" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"not null" json:"updated_at"`
}

func (KnowledgeIngestionJob) TableName() string { return "knowledge_ingestion_jobs" }
