package dtos

import (
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/enums"
)

type TimelineDay struct {
	Date        string `json:"date"` // "2026-11-01"
	Title       string `json:"title"`
	Description string `json:"description"`
	IsCheckIn   bool   `json:"is_check_in"`
	IsCheckOut  bool   `json:"is_check_out"`
}

type ReceiptPayment struct {
	ID        uuid.UUID           `json:"id"`
	Type      string              `json:"type"` // "charge" or "refund"
	AmountEUR float64             `json:"amount_eur"`
	AmountMinor int64             `json:"amount_minor"`
	Status    enums.PaymentStatus `json:"status"`
	CreatedAt time.Time           `json:"created_at"`
}

type TripItineraryResponse struct {
	ReservationID   uuid.UUID               `json:"reservation_id"`
	Code            string                  `json:"code"`
	Status          enums.ReservationStatus `json:"status"`
	ListingID       uuid.UUID               `json:"listing_id"`
	ListingTitle    string                  `json:"listing_title"`
	CoverPhotoURL   *string                 `json:"cover_photo_url,omitempty"`
	Address         *string                 `json:"address,omitempty"` // populated only if confirmed/checked_in/completed/reviewed
	City            string                  `json:"city"`
	Country         string                  `json:"country"`
	CheckInDate     string                  `json:"check_in_date"`
	CheckOutDate    string                  `json:"check_out_date"`
	CheckInTime     string                  `json:"check_in_time"`
	CheckOutTime    string                  `json:"check_out_time"`
	Timezone        string                  `json:"timezone"`
	CheckInInstant  time.Time               `json:"check_in_instant"`
	CheckOutInstant time.Time               `json:"check_out_instant"`
	HostName        string                  `json:"host_name"`
	HostEmail       string                  `json:"host_email"`
	Guests          int                     `json:"guests"`
	NightlyTotalMinor int64                 `json:"nightly_total_minor"`
	CleaningFeeMinor int64                  `json:"cleaning_fee_minor"`
	DiscountMinor   int64                   `json:"discount_minor"`
	GuestFeeMinor   int64                   `json:"guest_fee_minor"`
	GuestTotalMinor int64                   `json:"guest_total_minor"`
	Currency        string                  `json:"currency"`
	Timeline        []TimelineDay           `json:"timeline"`
	Receipts        []ReceiptPayment        `json:"receipts"`
}
