package dtos

import (
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/pricing"
)

type ListingPricingRuleInput struct {
	WeekendPriceMinor  *int64 `json:"weekend_price_minor"`
	WeekendDays        []int  `json:"weekend_days"`
	MinNights          int    `json:"min_nights"`
	WeeklyDiscountBPS  int    `json:"weekly_discount_bps"`
	MonthlyDiscountBPS int    `json:"monthly_discount_bps"`
}

type ListingPricingRuleResponse struct {
	ListingID          uuid.UUID `json:"listing_id"`
	WeekendPriceMinor  *int64    `json:"weekend_price_minor"`
	WeekendDays        []int     `json:"weekend_days"`
	MinNights          int       `json:"min_nights"`
	WeeklyDiscountBPS  int       `json:"weekly_discount_bps"`
	MonthlyDiscountBPS int       `json:"monthly_discount_bps"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type DateOverrideInput struct {
	Night      string `json:"night"` // YYYY-MM-DD
	PriceMinor int64  `json:"price_minor"`
	MinNights  *int   `json:"min_nights,omitempty"`
}

type DateOverrideRangeInput struct {
	From       string `json:"from"` // YYYY-MM-DD
	To         string `json:"to"`   // YYYY-MM-DD (inclusive or exclusive; let's treat as check-in to check-out dates)
	PriceMinor int64  `json:"price_minor"`
	MinNights  *int   `json:"min_nights,omitempty"`
}

type DateOverrideDeleteRangeInput struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type DateOverrideResponse struct {
	Night      string `json:"night"` // YYYY-MM-DD
	PriceMinor int64  `json:"price_minor"`
	MinNights  *int   `json:"min_nights,omitempty"`
}

type PriceQuoteResponse struct {
	Breakdown pricing.Breakdown `json:"breakdown"`
}
