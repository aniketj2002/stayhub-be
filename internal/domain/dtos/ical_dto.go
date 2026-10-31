package dtos

import (
	"time"

	"github.com/google/uuid"
)

type CreateICalFeedInput struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type ICalFeedResponse struct {
	ID            uuid.UUID  `json:"id"`
	ListingID     uuid.UUID  `json:"listing_id"`
	Name          string     `json:"name"`
	URL           string     `json:"url"`
	LastSyncedAt  *time.Time `json:"last_synced_at,omitempty"`
	LastSyncError *string    `json:"last_sync_error,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type ICalExportResponse struct {
	ExportURL string `json:"export_url"`
	Token     string `json:"token"`
}
