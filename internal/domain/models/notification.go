package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	ID        uuid.UUID       `db:"id" json:"id"`
	UserID    uuid.UUID       `db:"user_id" json:"user_id"`
	Title     string          `db:"title" json:"title"`
	Message   string          `db:"message" json:"message"`
	Channel   string          `db:"channel" json:"channel"` // in_app
	Status    string          `db:"status" json:"status"`   // unread, read, archived
	Metadata  json.RawMessage `db:"metadata" json:"metadata"`
	ReadAt    *time.Time      `db:"read_at" json:"read_at,omitempty"`
	CreatedAt time.Time       `db:"created_at" json:"created_at"`
}
