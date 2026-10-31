package dtos

import (
	"time"

	"github.com/google/uuid"
)

type CreateConversationInput struct {
	ReservationID uuid.UUID `json:"reservation_id" binding:"required"`
}

type SendMessageInput struct {
	Body     string      `json:"body"`
	MediaIDs []uuid.UUID `json:"media_ids,omitempty"`
}

type MessageResponse struct {
	ID             uuid.UUID   `json:"id"`
	ConversationID uuid.UUID   `json:"conversation_id"`
	SenderID       uuid.UUID   `json:"sender_id"`
	Body           string      `json:"body"`
	MediaIDs       []uuid.UUID `json:"media_ids"`
	MediaURLs      []string    `json:"media_urls"`
	Flags          []string    `json:"flags"`
	Warning        *string     `json:"warning,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
}

type ConversationResponse struct {
	ID                uuid.UUID        `json:"id"`
	ReservationID     uuid.UUID        `json:"reservation_id"`
	ReservationCode   string           `json:"reservation_code,omitempty"`
	ReservationStatus string           `json:"reservation_status,omitempty"`
	CheckInDate       string           `json:"check_in_date,omitempty"`
	CheckOutDate      string           `json:"check_out_date,omitempty"`
	Guests            int              `json:"guests,omitempty"`
	ListingID         uuid.UUID        `json:"listing_id"`
	ListingTitle      string           `json:"listing_title"`
	ListingCity       string           `json:"listing_city,omitempty"`
	ListingCoverURL   string           `json:"listing_cover_url,omitempty"`
	HostID            uuid.UUID        `json:"host_id"`
	HostName          string           `json:"host_name"`
	GuestID           uuid.UUID        `json:"guest_id"`
	GuestName         string           `json:"guest_name"`
	LastMessage       *MessageResponse `json:"last_message,omitempty"`
	UnreadCount       int              `json:"unread_count"`
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
}

type MessageListResponse struct {
	Messages   []MessageResponse `json:"messages"`
	HasMore    bool              `json:"has_more"`
	NextCursor *string           `json:"next_cursor,omitempty"`
}
