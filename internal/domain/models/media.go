package models

import (
	"time"

	"github.com/google/uuid"
)

// Media maps to the `media` table.
type Media struct {
	ID          uuid.UUID `db:"id" json:"id"`
	S3Key       string    `db:"s3_key" json:"s3_key"`
	ContentType string    `db:"content_type" json:"content_type"`
	SizeBytes   int64     `db:"size_bytes" json:"size_bytes"`
	Width       *int      `db:"width" json:"width,omitempty"`
	Height      *int      `db:"height" json:"height,omitempty"`
	Status      string    `db:"status" json:"status"` // 'pending' | 'uploaded'
	UploadedBy  uuid.UUID `db:"uploaded_by" json:"uploaded_by"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
}
