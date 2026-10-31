package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/enums"
)

type FeeConfig struct {
	ID              uuid.UUID             `db:"id" json:"id"`
	GuestFeeBPS     int                   `db:"guest_fee_bps" json:"guest_fee_bps"`
	HostFeeBPS      int                   `db:"host_fee_bps" json:"host_fee_bps"`
	Status          enums.FeeConfigStatus `db:"status" json:"status"`
	ProposedBy      uuid.UUID             `db:"proposed_by" json:"proposed_by"`
	ProposedReason  string                `db:"proposed_reason" json:"proposed_reason"`
	ApprovedBy      *uuid.UUID            `db:"approved_by" json:"approved_by,omitempty"`
	ApprovedAt      *time.Time            `db:"approved_at" json:"approved_at,omitempty"`
	RejectionReason *string               `db:"rejection_reason" json:"rejection_reason,omitempty"`
	CreatedAt       time.Time             `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time             `db:"updated_at" json:"updated_at"`
}
