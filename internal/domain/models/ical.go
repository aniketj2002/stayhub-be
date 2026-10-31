package models

import (
	"time"

	"github.com/google/uuid"
)

// ICalFeed maps to `ical_feeds`.
type ICalFeed struct {
	ID            uuid.UUID  `db:"id" json:"id"`
	ListingID     uuid.UUID  `db:"listing_id" json:"listing_id"`
	Name          string     `db:"name" json:"name"`
	URL           string     `db:"url" json:"url"`
	LastSyncedAt  *time.Time `db:"last_synced_at" json:"last_synced_at,omitempty"`
	LastSyncError *string    `db:"last_sync_error" json:"last_sync_error,omitempty"`
	CreatedAt     time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at" json:"updated_at"`
}
