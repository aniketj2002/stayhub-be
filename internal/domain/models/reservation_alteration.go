package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/enums"
)

// ReservationAlteration maps to the `reservation_alterations` table.
type ReservationAlteration struct {
	ID               uuid.UUID              `db:"id" json:"id"`
	ReservationID    uuid.UUID              `db:"reservation_id" json:"reservation_id"`
	RequestedBy      uuid.UUID              `db:"requested_by" json:"requested_by"`
	CurrentCheckIn   time.Time              `db:"current_check_in" json:"current_check_in"`
	CurrentCheckOut  time.Time              `db:"current_check_out" json:"current_check_out"`
	NewCheckIn       time.Time              `db:"new_check_in" json:"new_check_in"`
	NewCheckOut      time.Time              `db:"new_check_out" json:"new_check_out"`
	OriginalSnapshot json.RawMessage        `db:"original_snapshot" json:"original_snapshot"`
	RequestedChanges json.RawMessage        `db:"requested_changes" json:"requested_changes"`
	DeltaMinor       int64                  `db:"delta_minor" json:"delta_minor"`
	Currency         string                 `db:"currency" json:"currency"`
	Status           enums.AlterationStatus `db:"status" json:"status"`
	ExpiresAt        time.Time              `db:"expires_at" json:"expires_at"`
	DecidedAt        *time.Time             `db:"decided_at" json:"decided_at,omitempty"`
	DecisionReason   *string                `db:"decision_reason" json:"decision_reason,omitempty"`
	CreatedAt        time.Time              `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time              `db:"updated_at" json:"updated_at"`
}
