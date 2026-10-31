package models

import (
	"time"

	"github.com/google/uuid"
)

// Conversation maps to the `conversations` table.
type Conversation struct {
	ID            uuid.UUID `db:"id" json:"id"`
	ReservationID uuid.UUID `db:"reservation_id" json:"reservation_id"`
	ListingID     uuid.UUID `db:"listing_id" json:"listing_id"`
	GuestID       uuid.UUID `db:"guest_id" json:"guest_id"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at" json:"updated_at"`
}

// Message maps to the `messages` table.
type Message struct {
	ID             uuid.UUID   `db:"id" json:"id"`
	ConversationID uuid.UUID   `db:"conversation_id" json:"conversation_id"`
	SenderID       uuid.UUID   `db:"sender_id" json:"sender_id"`
	Body           string      `db:"body" json:"body"`
	MediaIDs       []uuid.UUID `db:"media_ids" json:"media_ids"`
	Flags          []string    `db:"flags" json:"flags"`
	CreatedAt      time.Time   `db:"created_at" json:"created_at"`
}

// ConversationRead maps to the `conversation_reads` table.
type ConversationRead struct {
	ConversationID uuid.UUID `db:"conversation_id" json:"conversation_id"`
	UserID         uuid.UUID `db:"user_id" json:"user_id"`
	LastReadAt     time.Time `db:"last_read_at" json:"last_read_at"`
}
