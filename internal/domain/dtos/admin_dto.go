package dtos

import (
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/models"
)

// --- Admin Reservations DTOs ---

type AdminReservationFilter struct {
	Code      string
	GuestID   *uuid.UUID
	HostID    *uuid.UUID
	ListingID *uuid.UUID
	Status    *enums.ReservationStatus
	From      *time.Time
	To        *time.Time
	Limit     int
	Offset    int
}

type AdminReservationSummary struct {
	Reservation models.Reservation        `json:"reservation"`
	Listing     ReservationListingSummary `json:"listing"`
	Guest       ReservationUserSummary    `json:"guest"`
	Host        ReservationUserSummary    `json:"host"`
}

type AdminReservationListResponse struct {
	Total        int                       `json:"total"`
	Reservations []AdminReservationSummary `json:"reservations"`
}

type AdminReservationDetailResponse struct {
	Reservation   models.Reservation          `json:"reservation"`
	Listing       ReservationListingSummary   `json:"listing"`
	Guest         ReservationUserSummary      `json:"guest"`
	Host          ReservationUserSummary      `json:"host"`
	Payments      []models.Payment            `json:"payments"`
	Payout        *models.Payout              `json:"payout,omitempty"`
	LedgerEntries []models.LedgerEntry        `json:"ledger_entries"`
	Timeline      []models.EventLog           `json:"timeline"`
	Refundable    int64                       `json:"refundable_minor"`
}

type AdminCancelReservationInput struct {
	Reason      string `json:"reason"`
	RefundMode  string `json:"refund_mode"` // "policy", "full", "custom"
	AmountMinor *int64 `json:"amount_minor,omitempty"`
}

type AdminRefundReservationInput struct {
	Reason      string `json:"reason"`
	AmountMinor int64  `json:"amount_minor"`
}

// --- Admin Payouts DTOs ---

type AdminPayoutHoldInput struct {
	Reason string `json:"reason"`
}

type AdminPayoutReleaseInput struct {
	Reason string `json:"reason"`
}

// --- Admin Listing Review DTOs ---

type AdminListingReviewFilter struct {
	Status enums.VersionStatus `json:"status"`
}

type AdminListingReviewQueueItem struct {
	ListingID         uuid.UUID                   `json:"listing_id"`
	VersionID         uuid.UUID                   `json:"version_id"`
	VersionNo         int                         `json:"version_no"`
	ListingTitle      string                      `json:"listing_title"`
	HostID            uuid.UUID                   `json:"host_id"`
	HostName          string                      `json:"host_name"`
	HostEmail         string                      `json:"host_email"`
	Status            enums.VersionStatus         `json:"status"`
	SubmittedAt       *time.Time                  `json:"submitted_at"`
	SubmittedAgeHours float64                     `json:"submitted_age_hours"`
	CheckResults      []models.ListingCheckResult `json:"check_results"`
	City              string                      `json:"city,omitempty"`
	Country           string                      `json:"country,omitempty"`
	PropertyType      string                      `json:"property_type,omitempty"`
	PhotoURL          string                      `json:"photo_url,omitempty"`
	MediaIDs          []uuid.UUID                 `json:"media_ids,omitempty"`
	ReviewCount       int                         `json:"review_count,omitempty"`
	RatingAvg         float64                     `json:"rating_avg,omitempty"`
}

type FieldDiff struct {
	Old any `json:"old"`
	New any `json:"new"`
}

type PhotosDiff struct {
	Added     []uuid.UUID `json:"added"`
	Removed   []uuid.UUID `json:"removed"`
	Reordered bool        `json:"reordered"`
}

type AdminListingDiffResponse struct {
	VersionID     uuid.UUID                   `json:"version_id"`
	ListingID     uuid.UUID                   `json:"listing_id"`
	VersionNo     int                         `json:"version_no"`
	Status        enums.VersionStatus         `json:"status"`
	SubmittedAt   *time.Time                  `json:"submitted_at,omitempty"`
	LiveVersionID *uuid.UUID                  `json:"live_version_id,omitempty"`
	LiveVersionNo *int                        `json:"live_version_no,omitempty"`
	FieldDiffs    map[string]FieldDiff        `json:"field_diffs"`
	PhotosDiff    PhotosDiff                  `json:"photos_diff"`
	City          string                      `json:"city,omitempty"`
	Country       string                      `json:"country,omitempty"`
	HostName      string                      `json:"host_name,omitempty"`
	HostEmail     string                      `json:"host_email,omitempty"`
	TargetVersion *models.ListingVersion      `json:"target_version,omitempty"`
	LiveVersion   *models.ListingVersion      `json:"live_version,omitempty"`
	CheckResults  []models.ListingCheckResult `json:"check_results,omitempty"`
}

type AdminListingDecisionInput struct {
	Note string `json:"note"`
}

// --- Admin Fee Config DTOs ---

type ProposeFeeConfigInput struct {
	GuestFeeBPS int    `json:"guest_fee_bps"`
	HostFeeBPS  int    `json:"host_fee_bps"`
	Reason      string `json:"reason"`
}

type RejectFeeConfigInput struct {
	Reason string `json:"reason"`
}

// --- Admin User / RBAC DTOs ---

type CreateAdminInput struct {
	Email    string          `json:"email"`
	Password string          `json:"password"`
	FullName string          `json:"full_name"`
	Role     enums.AdminRole `json:"role"`
}

type UpdateAdminInput struct {
	Role        *enums.AdminRole `json:"role,omitempty"`
	Permissions map[string]bool  `json:"permissions,omitempty"`
}

type AdminDetailResponse struct {
	ID          uuid.UUID                `json:"id"`
	Email       string                   `json:"email"`
	FullName    string                   `json:"full_name"`
	Role        enums.AdminRole          `json:"role"`
	Permissions []string                 `json:"permissions"`
	Overrides   []models.AdminPermission `json:"overrides"`
	CreatedAt   time.Time                `json:"created_at"`
	UpdatedAt   time.Time                `json:"updated_at"`
}
