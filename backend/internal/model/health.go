package model

import "time"

type HealthStatus struct {
	Status       string            `json:"status"`
	Service      string            `json:"service"`
	Version      string            `json:"version"`
	CheckedAt    time.Time         `json:"checked_at"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
}
