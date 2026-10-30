package dtos

import (
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/models"
)

type CreateReservationInput struct {
	ListingID          uuid.UUID
	CheckIn            time.Time
	CheckOut           time.Time
	Guests             int
	GuestMessage       *string
	PaymentMethodToken string
}

type DeclineReservationInput struct {
	DeclineReason string `json:"decline_reason"`
}

type CancelReservationInput struct {
	CancelReason *string `json:"cancel_reason,omitempty"`
}

type ReservationListingSummary struct {
	ID            uuid.UUID `json:"id"`
	Title         string    `json:"title"`
	PropertyType  string    `json:"property_type"`
	City          string    `json:"city"`
	Country       string    `json:"country"`
	AddressLine1  string    `json:"address_line1"`
	CoverPhotoURL *string   `json:"cover_photo_url,omitempty"`
}

type ReservationUserSummary struct {
	ID        uuid.UUID `json:"id"`
	FullName  string    `json:"full_name"`
	Email     string    `json:"email"`
	AvatarURL *string   `json:"avatar_url,omitempty"`
}

type ReservationDetailResponse struct {
	Reservation        models.Reservation          `json:"reservation"`
	Nights             int                         `json:"nights"`
	Listing            ReservationListingSummary   `json:"listing"`
	Guest              ReservationUserSummary      `json:"guest"`
	Host               ReservationUserSummary      `json:"host"`
	Payments           []models.Payment            `json:"payments"`
	Payout             *models.Payout              `json:"payout,omitempty"`
	LedgerEntries      []models.LedgerEntry        `json:"ledger_entries,omitempty"`
	CancellationPolicy models.CancellationPolicy   `json:"cancellation_policy"`
}

type CancelPreviewResponse struct {
	ReservationID uuid.UUID                          `json:"reservation_id"`
	Code          string                             `json:"code"`
	Status        enums.ReservationStatus            `json:"status"`
	EvaluatedAt   time.Time                          `json:"evaluated_at"`
	CheckInAt     time.Time                          `json:"check_in_at"`
	Currency      string                             `json:"currency"`
	GuestPaidMinor int64                             `json:"guest_paid_minor"`
	Breakdown     models.CancellationRefundBreakdown `json:"breakdown"`
}

type HostTodayResponse struct {
	PendingRequests  []ReservationDetailResponse `json:"pending_requests"`
	ArrivingToday    []ReservationDetailResponse `json:"arriving_today"`
	CurrentlyHosting []ReservationDetailResponse `json:"currently_hosting"`
	CheckingOutToday []ReservationDetailResponse `json:"checking_out_today"`
	Upcoming         []ReservationDetailResponse `json:"upcoming"`
}

type EarningsPayoutItem struct {
	Payout          models.Payout `json:"payout"`
	ReservationCode string        `json:"reservation_code"`
	ListingID       uuid.UUID     `json:"listing_id"`
	ListingTitle    string        `json:"listing_title"`
	CheckIn         string        `json:"check_in"`
	CheckOut        string        `json:"check_out"`
}

type HostEarningsResponse struct {
	Currency       string               `json:"currency"`
	PaidMinor      int64                `json:"paid_minor"`
	UpcomingMinor  int64                `json:"upcoming_minor"`
	HeldMinor      int64                `json:"held_minor"`
	CancelledMinor int64                `json:"cancelled_minor"`
	Payouts        []EarningsPayoutItem `json:"payouts"`
}
