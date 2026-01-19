// Package types contains shared types used across the RSS feed backend
package types

import (
	"time"
)

// AsyncJobStatus represents the status of an async job
type AsyncJobStatus struct {
	JobID       string     `json:"job_id"`
	URL         string     `json:"url"`
	Status      string     `json:"status"` // pending, processing, completed, failed
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Error       string     `json:"error,omitempty"`
	ItemsCount  int        `json:"items_count,omitempty"`
	DurationMs  int64      `json:"duration_ms,omitempty"`
}
