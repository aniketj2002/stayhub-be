package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/enums"
)

// ListingPricingRule maps to `listing_pricing_rules`.
type ListingPricingRule struct {
	ListingID          uuid.UUID `db:"listing_id" json:"listing_id"`
	WeekendPriceMinor  *int64    `db:"weekend_price_minor" json:"weekend_price_minor,omitempty"`
	WeekendDays        []int     `db:"weekend_days" json:"weekend_days"`
	MinNights          int       `db:"min_nights" json:"min_nights"`
	WeeklyDiscountBPS  int       `db:"weekly_discount_bps" json:"weekly_discount_bps"`
	MonthlyDiscountBPS int       `db:"monthly_discount_bps" json:"monthly_discount_bps"`
	CreatedAt          time.Time `db:"created_at" json:"created_at"`
	UpdatedAt          time.Time `db:"updated_at" json:"updated_at"`
}

// ListingDateOverride maps to `listing_date_overrides`.
type ListingDateOverride struct {
	ListingID  uuid.UUID `db:"listing_id" json:"listing_id"`
	Night      time.Time `db:"night" json:"night"`
	PriceMinor int64     `db:"price_minor" json:"price_minor"`
	MinNights  *int      `db:"min_nights" json:"min_nights,omitempty"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
}

// ReservationNight maps to `reservation_nights`.
type ReservationNight struct {
	ID            uuid.UUID      `db:"id" json:"id"`
	ReservationID uuid.UUID      `db:"reservation_id" json:"reservation_id"`
	Night         time.Time      `db:"night" json:"night"`
	PriceMinor    int64          `db:"price_minor" json:"price_minor"`
	RateType      enums.RateType `db:"rate_type" json:"rate_type"`
	CreatedAt     time.Time      `db:"created_at" json:"created_at"`
}
