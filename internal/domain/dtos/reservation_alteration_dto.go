package dtos

import (
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/enums"
)

type CreateReservationAlterationInput struct {
	CheckIn  *string        `json:"check_in,omitempty"`  // "YYYY-MM-DD"
	CheckOut *string        `json:"check_out,omitempty"` // "YYYY-MM-DD"
	Guests   *int           `json:"guests,omitempty"`
	Config   map[string]any `json:"config,omitempty"`
}

type DeclineReservationAlterationInput struct {
	Reason string `json:"reason"`
}

type HostPendingAlteration struct {
	ReservationAlterationResponse
	ReservationCode string `json:"reservation_code"`
	ListingTitle    string `json:"listing_title"`
}

type ReservationAlterationResponse struct {
	ID               uuid.UUID              `json:"id"`
	ReservationID    uuid.UUID              `json:"reservation_id"`
	RequestedBy      uuid.UUID              `json:"requested_by"`
	CurrentCheckIn   string                 `json:"current_check_in"`
	CurrentCheckOut  string                 `json:"current_check_out"`
	NewCheckIn       string                 `json:"new_check_in"`
	NewCheckOut      string                 `json:"new_check_out"`
	OriginalSnapshot map[string]any         `json:"original_snapshot"`
	RequestedChanges map[string]any         `json:"requested_changes"`
	DeltaMinor       int64                  `json:"delta_minor"`
	Currency         string                 `json:"currency"`
	Status           enums.AlterationStatus `json:"status"`
	ExpiresAt        time.Time              `json:"expires_at"`
	DecidedAt        *time.Time             `json:"decided_at,omitempty"`
	DecisionReason   *string                `json:"decision_reason,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
}
