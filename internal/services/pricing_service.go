package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/domain/pricing"
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/db"
)

type PricingService struct {
	db          *db.DB
	pricingRepo interfaces.PricingRepository
	listingRepo interfaces.ListingRepository
	feeConfig   interfaces.FeeConfigService
	clock       clock.Clock
}

func NewPricingService(
	database *db.DB,
	pricingRepo interfaces.PricingRepository,
	listingRepo interfaces.ListingRepository,
	feeConfig interfaces.FeeConfigService,
	clk clock.Clock,
) interfaces.PricingService {
	return &PricingService{
		db:          database,
		pricingRepo: pricingRepo,
		listingRepo: listingRepo,
		feeConfig:   feeConfig,
		clock:       clk,
	}
}

func (s *PricingService) GetPricingRule(ctx context.Context, listingID uuid.UUID) (*dtos.ListingPricingRuleResponse, error) {
	rule, err := s.pricingRepo.GetPricingRule(ctx, listingID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			// Return default rules
			return &dtos.ListingPricingRuleResponse{
				ListingID:          listingID,
				WeekendDays:        []int{5, 6},
				MinNights:          1,
				WeeklyDiscountBPS:  0,
				MonthlyDiscountBPS: 0,
				UpdatedAt:          s.clock.Now(),
			}, nil
		}
		return nil, err
	}

	return &dtos.ListingPricingRuleResponse{
		ListingID:          rule.ListingID,
		WeekendPriceMinor:  rule.WeekendPriceMinor,
		WeekendDays:        rule.WeekendDays,
		MinNights:          rule.MinNights,
		WeeklyDiscountBPS:  rule.WeeklyDiscountBPS,
		MonthlyDiscountBPS: rule.MonthlyDiscountBPS,
		UpdatedAt:          rule.UpdatedAt,
	}, nil
}

func (s *PricingService) SetPricingRule(ctx context.Context, listingID uuid.UUID, in dtos.ListingPricingRuleInput) (*dtos.ListingPricingRuleResponse, error) {
	if in.MinNights < 1 {
		in.MinNights = 1
	}
	if in.WeeklyDiscountBPS < 0 || in.WeeklyDiscountBPS > 10000 {
		return nil, fmt.Errorf("%w: weekly discount bps must be between 0 and 10000", apperrors.ErrValidation)
	}
	if in.MonthlyDiscountBPS < 0 || in.MonthlyDiscountBPS > 10000 {
		return nil, fmt.Errorf("%w: monthly discount bps must be between 0 and 10000", apperrors.ErrValidation)
	}
	if in.WeekendPriceMinor != nil && *in.WeekendPriceMinor < 0 {
		return nil, fmt.Errorf("%w: weekend price must not be negative", apperrors.ErrValidation)
	}
	if len(in.WeekendDays) == 0 {
		in.WeekendDays = []int{5, 6}
	} else {
		for _, d := range in.WeekendDays {
			if d < 0 || d > 6 {
				return nil, fmt.Errorf("%w: weekend days must be integers between 0 and 6", apperrors.ErrValidation)
			}
		}
	}

	saved, err := s.pricingRepo.UpsertPricingRule(ctx, models.ListingPricingRule{
		ListingID:          listingID,
		WeekendPriceMinor:  in.WeekendPriceMinor,
		WeekendDays:        in.WeekendDays,
		MinNights:          in.MinNights,
		WeeklyDiscountBPS:  in.WeeklyDiscountBPS,
		MonthlyDiscountBPS: in.MonthlyDiscountBPS,
	})
	if err != nil {
		return nil, err
	}

	return &dtos.ListingPricingRuleResponse{
		ListingID:          saved.ListingID,
		WeekendPriceMinor:  saved.WeekendPriceMinor,
		WeekendDays:        saved.WeekendDays,
		MinNights:          saved.MinNights,
		WeeklyDiscountBPS:  saved.WeeklyDiscountBPS,
		MonthlyDiscountBPS: saved.MonthlyDiscountBPS,
		UpdatedAt:          saved.UpdatedAt,
	}, nil
}

func (s *PricingService) GetDateOverrides(ctx context.Context, listingID uuid.UUID, from, to time.Time) ([]dtos.DateOverrideResponse, error) {
	overrides, err := s.pricingRepo.GetDateOverrides(ctx, listingID, from, to)
	if err != nil {
		return nil, err
	}

	out := make([]dtos.DateOverrideResponse, 0, len(overrides))
	for _, o := range overrides {
		out = append(out, dtos.DateOverrideResponse{
			Night:      o.Night.Format("2006-01-02"),
			PriceMinor: o.PriceMinor,
			MinNights:  o.MinNights,
		})
	}
	return out, nil
}

func (s *PricingService) SetDateOverrides(ctx context.Context, listingID uuid.UUID, in dtos.DateOverrideRangeInput) error {
	from, err := time.Parse("2006-01-02", in.From)
	if err != nil {
		return fmt.Errorf("%w: invalid from date format, expected YYYY-MM-DD", apperrors.ErrValidation)
	}
	to, err := time.Parse("2006-01-02", in.To)
	if err != nil {
		return fmt.Errorf("%w: invalid to date format, expected YYYY-MM-DD", apperrors.ErrValidation)
	}
	if to.Before(from) {
		return fmt.Errorf("%w: to date must be on or after from date", apperrors.ErrValidation)
	}
	if in.PriceMinor < 0 {
		return fmt.Errorf("%w: price_minor must not be negative", apperrors.ErrValidation)
	}
	if in.MinNights != nil && *in.MinNights < 1 {
		return fmt.Errorf("%w: min_nights must be at least 1", apperrors.ErrValidation)
	}

	var overrides []models.ListingDateOverride
	cur := from
	for !cur.After(to) {
		overrides = append(overrides, models.ListingDateOverride{
			ListingID:  listingID,
			Night:      cur,
			PriceMinor: in.PriceMinor,
			MinNights:  in.MinNights,
		})
		cur = cur.AddDate(0, 0, 1)
	}

	return s.pricingRepo.UpsertDateOverrides(ctx, listingID, overrides)
}

func (s *PricingService) DeleteDateOverrides(ctx context.Context, listingID uuid.UUID, from, to time.Time) error {
	if to.Before(from) {
		return fmt.Errorf("%w: to date must be on or after from date", apperrors.ErrValidation)
	}
	return s.pricingRepo.DeleteDateOverrides(ctx, listingID, from, to)
}

func (s *PricingService) CalculateQuote(ctx context.Context, listingID uuid.UUID, checkIn, checkOut time.Time) (pricing.Breakdown, error) {
	listing, err := s.listingRepo.FindByID(ctx, listingID)
	if err != nil {
		return pricing.Breakdown{}, err
	}
	if listing.LiveVersionID == nil {
		return pricing.Breakdown{}, fmt.Errorf("%w: listing is not live", apperrors.ErrNotFound)
	}

	ver, err := s.listingRepo.FindVersionByID(ctx, *listing.LiveVersionID)
	if err != nil {
		return pricing.Breakdown{}, err
	}

	basePrice := int64(0)
	if ver.BasePriceMinor != nil {
		basePrice = *ver.BasePriceMinor
	}
	cleaningFee := ver.CleaningFeeMinor
	currency := ver.Currency
	if currency == "" {
		currency = "EUR"
	}

	// Fetch rules
	rule, err := s.pricingRepo.GetPricingRule(ctx, listingID)
	listingPricing := pricing.ListingPricing{
		BasePriceMinor:   basePrice,
		CleaningFeeMinor: cleaningFee,
		Currency:         currency,
		MinNights:        1,
		WeekendDays:      []int{5, 6},
	}
	if err == nil && rule != nil {
		listingPricing.WeekendPriceMinor = rule.WeekendPriceMinor
		listingPricing.WeekendDays = rule.WeekendDays
		listingPricing.MinNights = rule.MinNights
		listingPricing.WeeklyDiscountBPS = rule.WeeklyDiscountBPS
		listingPricing.MonthlyDiscountBPS = rule.MonthlyDiscountBPS
	}

	// Fetch overrides for stay
	overrides, _ := s.pricingRepo.GetDateOverrides(ctx, listingID, checkIn, checkOut)
	pricingOverrides := make([]pricing.DateOverride, 0, len(overrides))
	for _, o := range overrides {
		pricingOverrides = append(pricingOverrides, pricing.DateOverride{
			Night:      o.Night,
			PriceMinor: o.PriceMinor,
			MinNights:  o.MinNights,
		})
	}

	// Fetch active fees
	feeCfg := pricing.FeeConfig{GuestFeeBPS: 1400, HostFeeBPS: 300}
	if s.feeConfig != nil {
		if act, err := s.feeConfig.GetActive(ctx); err == nil && act != nil {
			feeCfg.GuestFeeBPS = act.GuestFeeBPS
			feeCfg.HostFeeBPS = act.HostFeeBPS
		}
	}

	return pricing.Quote(listingPricing, pricingOverrides, checkIn, checkOut, feeCfg)
}
