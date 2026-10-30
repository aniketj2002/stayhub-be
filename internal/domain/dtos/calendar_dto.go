package dtos

import (
	"time"

	"github.com/google/uuid"
)

type CalendarQueryInput struct {
	StartDate time.Time
	EndDate   time.Time
}

type CalendarBlockInput struct {
	StartDate   time.Time
	EndDate     time.Time
	BlockReason *string
}

type CalendarUnblockInput struct {
	StartDate time.Time
	EndDate   time.Time
}

type CalendarNightEntry struct {
	Night         string     `json:"night"`
	Status        string     `json:"status"` // "available" | "blocked" | "booked"
	ReservationID *uuid.UUID `json:"reservation_id,omitempty"`
	BlockReason   *string    `json:"block_reason,omitempty"`
}

type CalendarResponse struct {
	ListingID string               `json:"listing_id"`
	StartDate string               `json:"start_date"`
	EndDate   string               `json:"end_date"`
	Nights    []CalendarNightEntry `json:"nights"`
}
