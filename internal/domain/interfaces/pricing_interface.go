package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/domain/pricing"
)

type PricingRepository interface {
	GetPricingRule(ctx context.Context, listingID uuid.UUID) (*models.ListingPricingRule, error)
	UpsertPricingRule(ctx context.Context, rule models.ListingPricingRule) (*models.ListingPricingRule, error)
	GetDateOverrides(ctx context.Context, listingID uuid.UUID, from, to time.Time) ([]models.ListingDateOverride, error)
	UpsertDateOverrides(ctx context.Context, listingID uuid.UUID, overrides []models.ListingDateOverride) error
	DeleteDateOverrides(ctx context.Context, listingID uuid.UUID, from, to time.Time) error
	CreateReservationNights(ctx context.Context, nights []models.ReservationNight) error
	ListReservationNights(ctx context.Context, reservationID uuid.UUID) ([]models.ReservationNight, error)
}

type PricingService interface {
	GetPricingRule(ctx context.Context, listingID uuid.UUID) (*dtos.ListingPricingRuleResponse, error)
	SetPricingRule(ctx context.Context, listingID uuid.UUID, in dtos.ListingPricingRuleInput) (*dtos.ListingPricingRuleResponse, error)
	GetDateOverrides(ctx context.Context, listingID uuid.UUID, from, to time.Time) ([]dtos.DateOverrideResponse, error)
	SetDateOverrides(ctx context.Context, listingID uuid.UUID, in dtos.DateOverrideRangeInput) error
	DeleteDateOverrides(ctx context.Context, listingID uuid.UUID, from, to time.Time) error
	CalculateQuote(ctx context.Context, listingID uuid.UUID, checkIn, checkOut time.Time) (pricing.Breakdown, error)
}
