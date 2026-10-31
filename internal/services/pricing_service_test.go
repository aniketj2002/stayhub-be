package services

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/db"
)

type memPricingRepo struct {
	mu        sync.Mutex
	rules     map[uuid.UUID]models.ListingPricingRule
	overrides map[string]models.ListingDateOverride
}

func newMemPricingRepo() *memPricingRepo {
	return &memPricingRepo{
		rules:     make(map[uuid.UUID]models.ListingPricingRule),
		overrides: make(map[string]models.ListingDateOverride),
	}
}

func overrideKey(listingID uuid.UUID, night time.Time) string {
	return listingID.String() + ":" + night.Format("2006-01-02")
}

func (m *memPricingRepo) GetPricingRule(_ context.Context, listingID uuid.UUID) (*models.ListingPricingRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rule, ok := m.rules[listingID]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	return &rule, nil
}

func (m *memPricingRepo) UpsertPricingRule(_ context.Context, rule models.ListingPricingRule) (*models.ListingPricingRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules[rule.ListingID] = rule
	return &rule, nil
}

func (m *memPricingRepo) GetDateOverrides(_ context.Context, listingID uuid.UUID, from, to time.Time) ([]models.ListingDateOverride, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.ListingDateOverride
	for _, o := range m.overrides {
		if o.ListingID == listingID && !o.Night.Before(from) && !o.Night.After(to) {
			out = append(out, o)
		}
	}
	return out, nil
}

func (m *memPricingRepo) UpsertDateOverrides(_ context.Context, listingID uuid.UUID, overrides []models.ListingDateOverride) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, o := range overrides {
		m.overrides[overrideKey(listingID, o.Night)] = o
	}
	return nil
}

func (m *memPricingRepo) DeleteDateOverrides(_ context.Context, listingID uuid.UUID, from, to time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, o := range m.overrides {
		if o.ListingID == listingID && !o.Night.Before(from) && !o.Night.After(to) {
			delete(m.overrides, k)
		}
	}
	return nil
}

func (m *memPricingRepo) CreateReservationNights(_ context.Context, _ []models.ReservationNight) error {
	return nil
}

func (m *memPricingRepo) ListReservationNights(_ context.Context, _ uuid.UUID) ([]models.ReservationNight, error) {
	return nil, nil
}

func TestPricingService_RulesAndOverrides(t *testing.T) {
	pricingRepo := newMemPricingRepo()
	listingRepo := newMemListingRepo()
	clk := clock.NewMockClock(time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC))
	svc := NewPricingService(&db.DB{}, pricingRepo, listingRepo, nil, clk)
	ctx := context.Background()

	listingID := uuid.New()

	// 1. Default pricing rules when none saved
	defRule, err := svc.GetPricingRule(ctx, listingID)
	if err != nil {
		t.Fatalf("unexpected error getting default rule: %v", err)
	}
	if defRule.MinNights != 1 {
		t.Errorf("expected default min nights 1, got %d", defRule.MinNights)
	}
	if len(defRule.WeekendDays) != 2 || defRule.WeekendDays[0] != 5 || defRule.WeekendDays[1] != 6 {
		t.Errorf("expected default weekend days [5, 6], got %v", defRule.WeekendDays)
	}

	// 2. Set custom pricing rule
	weekendPrice := int64(15000)
	ruleIn := dtos.ListingPricingRuleInput{
		WeekendPriceMinor:  &weekendPrice,
		WeekendDays:        []int{5, 6},
		MinNights:          2,
		WeeklyDiscountBPS:  1000, // 10%
		MonthlyDiscountBPS: 2000, // 20%
	}
	savedRule, err := svc.SetPricingRule(ctx, listingID, ruleIn)
	if err != nil {
		t.Fatalf("failed to set pricing rule: %v", err)
	}
	if *savedRule.WeekendPriceMinor != 15000 || savedRule.MinNights != 2 || savedRule.WeeklyDiscountBPS != 1000 {
		t.Fatalf("unexpected saved rule response: %+v", savedRule)
	}

	// 3. Validation: discount > 10000 bps rejected
	invalidRule := ruleIn
	invalidRule.WeeklyDiscountBPS = 10001
	if _, err := svc.SetPricingRule(ctx, listingID, invalidRule); err == nil {
		t.Errorf("expected error when weekly discount bps > 10000")
	}

	// 4. Set date overrides
	overrideMinNights := 3
	overrideIn := dtos.DateOverrideRangeInput{
		From:       "2026-12-24",
		To:         "2026-12-26",
		PriceMinor: 25000,
		MinNights:  &overrideMinNights,
	}
	if err := svc.SetDateOverrides(ctx, listingID, overrideIn); err != nil {
		t.Fatalf("failed to set date overrides: %v", err)
	}

	overrides, err := svc.GetDateOverrides(ctx, listingID, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("failed to get date overrides: %v", err)
	}
	// 24, 25, 26 => 3 nights
	if len(overrides) != 3 {
		t.Fatalf("expected 3 override nights, got %d", len(overrides))
	}
	for _, o := range overrides {
		if o.PriceMinor != 25000 {
			t.Errorf("expected price 25000, got %d", o.PriceMinor)
		}
		if *o.MinNights != 3 {
			t.Errorf("expected min nights 3, got %v", o.MinNights)
		}
	}

	// 5. Delete date overrides
	if err := svc.DeleteDateOverrides(ctx, listingID, time.Date(2026, 12, 25, 0, 0, 0, 0, time.UTC), time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("failed to delete overrides: %v", err)
	}
	overridesAfter, err := svc.GetDateOverrides(ctx, listingID, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("failed to get overrides after delete: %v", err)
	}
	if len(overridesAfter) != 1 { // Only 2026-12-24 remains
		t.Fatalf("expected 1 remaining override, got %d", len(overridesAfter))
	}
}

func TestPricingService_CalculateQuote(t *testing.T) {
	pricingRepo := newMemPricingRepo()
	listingRepo := newMemListingRepo()
	clk := clock.NewMockClock(time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC))
	svc := NewPricingService(&db.DB{}, pricingRepo, listingRepo, nil, clk)
	ctx := context.Background()

	listingID := uuid.New()
	versionID := uuid.New()
	basePrice := int64(10000) // $100/night
	cleaningFee := int64(5000) // $50

	listingRepo.listings[listingID] = models.Listing{
		ID:            listingID,
		LiveVersionID: &versionID,
	}
	listingRepo.versions[versionID] = models.ListingVersion{
		ID:               versionID,
		ListingID:        listingID,
		BasePriceMinor:   &basePrice,
		CleaningFeeMinor: cleaningFee,
		Currency:         "USD",
	}

	// Rule: weekend price $150 (15000 minor), 10% weekly discount (1000 BPS), min nights 2
	weekendPrice := int64(15000)
	_, _ = pricingRepo.UpsertPricingRule(ctx, models.ListingPricingRule{
		ListingID:         listingID,
		WeekendPriceMinor: &weekendPrice,
		WeekendDays:       []int{5, 6},
		MinNights:         2,
		WeeklyDiscountBPS: 1000,
	})

	// Quote 1 night -> fails min nights (2 required)
	checkIn := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC) // Monday
	checkOut := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC) // Tuesday (1 night)
	_, err := svc.CalculateQuote(ctx, listingID, checkIn, checkOut)
	if err == nil {
		t.Fatalf("expected error min nights not met")
	}

	// Quote 7 nights (Nov 2 Mon -> Nov 9 Mon)
	// Monday to Thursday (4 nights @ 10000 = 40000)
	// Friday & Saturday (2 nights @ 15000 = 30000)
	// Sunday (1 night @ 10000 = 10000)
	// Base total = 80000
	// 10% weekly discount = 8000 -> Discounted accommodation = 72000
	// Cleaning fee = 5000
	// Subtotal = 77000
	// Guest fee 14% on 77000 = 10780 -> Guest Total = 87780
	// Host fee 3% on 77000 = 2310 -> Host Net = 74690
	checkIn7 := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	checkOut7 := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	quote, err := svc.CalculateQuote(ctx, listingID, checkIn7, checkOut7)
	if err != nil {
		t.Fatalf("unexpected quote error: %v", err)
	}

	if quote.SubtotalMinor != 80000 {
		t.Errorf("expected subtotal minor 80000, got %d", quote.SubtotalMinor)
	}
	if quote.DiscountMinor != 8000 {
		t.Errorf("expected discount minor 8000, got %d", quote.DiscountMinor)
	}
	if quote.AccommodationSubtotalMinor != 77000 {
		t.Errorf("expected accommodation subtotal minor 77000, got %d", quote.AccommodationSubtotalMinor)
	}
	if quote.GuestFeeMinor != 10780 {
		t.Errorf("expected guest fee 10780, got %d", quote.GuestFeeMinor)
	}
	if quote.GuestTotalMinor != 87780 {
		t.Errorf("expected guest total 87780, got %d", quote.GuestTotalMinor)
	}
	if quote.HostFeeMinor != 2310 {
		t.Errorf("expected host fee 2310, got %d", quote.HostFeeMinor)
	}
	if quote.HostNetMinor != 74690 {
		t.Errorf("expected host net 74690, got %d", quote.HostNetMinor)
	}
	if len(quote.Nights) != 7 {
		t.Fatalf("expected 7 nightly breakdowns, got %d", len(quote.Nights))
	}
}
