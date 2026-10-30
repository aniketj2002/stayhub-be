package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/enums"
)

// Reservation maps to the `reservations` table.
type Reservation struct {
	ID                 uuid.UUID               `db:"id" json:"id"`
	Code               string                  `db:"code" json:"code"`
	ListingID          uuid.UUID               `db:"listing_id" json:"listing_id"`
	ListingVersionID   uuid.UUID               `db:"listing_version_id" json:"listing_version_id"`
	GuestID            uuid.UUID               `db:"guest_id" json:"guest_id"`
	HostID             uuid.UUID               `db:"host_id" json:"host_id"`
	Status             enums.ReservationStatus `db:"status" json:"status"`
	IsInstantBook      bool                    `db:"is_instant_book" json:"is_instant_book"`
	CheckIn            time.Time               `db:"check_in" json:"check_in"`
	CheckOut           time.Time               `db:"check_out" json:"check_out"`
	CheckInTime        string                  `db:"check_in_time" json:"check_in_time"`
	CheckOutTime       string                  `db:"check_out_time" json:"check_out_time"`
	Timezone           string                  `db:"timezone" json:"timezone"`
	CheckInAt          time.Time               `db:"check_in_at" json:"check_in_at"`
	CheckOutAt         time.Time               `db:"check_out_at" json:"check_out_at"`
	Guests             int                     `db:"guests" json:"guests"`
	GuestMessage       *string                 `db:"guest_message" json:"guest_message,omitempty"`
	Currency           string                  `db:"currency" json:"currency"`
	NightlyTotalMinor  int64                   `db:"nightly_total_minor" json:"nightly_total_minor"`
	CleaningFeeMinor   int64                   `db:"cleaning_fee_minor" json:"cleaning_fee_minor"`
	GuestFeeBPS        int                     `db:"guest_fee_bps" json:"guest_fee_bps"`
	GuestFeeMinor      int64                   `db:"guest_fee_minor" json:"guest_fee_minor"`
	GuestTotalMinor    int64                   `db:"guest_total_minor" json:"guest_total_minor"`
	HostFeeBPS         int                     `db:"host_fee_bps" json:"host_fee_bps"`
	HostFeeMinor       int64                   `db:"host_fee_minor" json:"host_fee_minor"`
	HostNetMinor       int64                   `db:"host_net_minor" json:"host_net_minor"`
	CancellationPolicy string                  `db:"cancellation_policy" json:"cancellation_policy"`
	RequestExpiresAt   *time.Time              `db:"request_expires_at" json:"request_expires_at,omitempty"`
	DecidedAt          *time.Time              `db:"decided_at" json:"decided_at,omitempty"`
	DeclineReason      *string                 `db:"decline_reason" json:"decline_reason,omitempty"`
	CancelledAt        *time.Time              `db:"cancelled_at" json:"cancelled_at,omitempty"`
	CancelledBy        *enums.ActorType        `db:"cancelled_by" json:"cancelled_by,omitempty"`
	CancelReason       *string                 `db:"cancel_reason" json:"cancel_reason,omitempty"`
	CheckedInAt        *time.Time              `db:"checked_in_at" json:"checked_in_at,omitempty"`
	CompletedAt        *time.Time              `db:"completed_at" json:"completed_at,omitempty"`
	Version            int                     `db:"version" json:"version"`
	CreatedAt          time.Time               `db:"created_at" json:"created_at"`
	UpdatedAt          time.Time               `db:"updated_at" json:"updated_at"`
}
