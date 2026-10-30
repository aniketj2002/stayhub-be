package dtos

import (
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/models"
)

type PresignMediaInput struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	Width       *int   `json:"width,omitempty"`
	Height      *int   `json:"height,omitempty"`
	Purpose     string `json:"purpose,omitempty"`
}

type ConfirmMediaInput struct {
	Width  *int `json:"width,omitempty"`
	Height *int `json:"height,omitempty"`
}

type CreateMediaParams struct {
	ID          uuid.UUID
	S3Key       string
	ContentType string
	SizeBytes   int64
	Width       *int
	Height      *int
	UploadedBy  uuid.UUID
}

type MediaResponse struct {
	ID          uuid.UUID `json:"id"`
	S3Key       string    `json:"s3_key"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	Width       *int      `json:"width,omitempty"`
	Height      *int      `json:"height,omitempty"`
	Status      string    `json:"status"`
	UploadedBy  uuid.UUID `json:"uploaded_by"`
	PublicURL   string    `json:"public_url"`
	CreatedAt   time.Time `json:"created_at"`
}

func ToMediaResponse(m models.Media, publicURL string) MediaResponse {
	return MediaResponse{
		ID:          m.ID,
		S3Key:       m.S3Key,
		ContentType: m.ContentType,
		SizeBytes:   m.SizeBytes,
		Width:       m.Width,
		Height:      m.Height,
		Status:      m.Status,
		UploadedBy:  m.UploadedBy,
		PublicURL:   publicURL,
		CreatedAt:   m.CreatedAt,
	}
}

type PresignMediaResponse struct {
	Media     MediaResponse `json:"media"`
	UploadURL string        `json:"upload_url"`
	Method    string        `json:"method"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time     `json:"expires_at"`
}
