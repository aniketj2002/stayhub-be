package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type OutboxEvent struct {
	ID            int64           `db:"id" json:"id"`
	EventName     string          `db:"event_name" json:"event_name"`
	AggregateType string          `db:"aggregate_type" json:"aggregate_type"`
	AggregateID   uuid.UUID       `db:"aggregate_id" json:"aggregate_id"`
	Payload       json.RawMessage `db:"payload" json:"payload"`
	Status        string          `db:"status" json:"status"` // pending, processing, published, failed
	RetryCount    int             `db:"retry_count" json:"retry_count"`
	NextRetryAt   time.Time       `db:"next_retry_at" json:"next_retry_at"`
	PublishedAt   *time.Time      `db:"published_at" json:"published_at,omitempty"`
	ErrorDetail   *string         `db:"error_detail" json:"error_detail,omitempty"`
	CreatedAt     time.Time       `db:"created_at" json:"created_at"`
}
