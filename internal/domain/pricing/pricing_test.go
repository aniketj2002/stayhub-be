package pricing

import (
	"errors"
	"testing"
	"time"
)

func TestPricingEngine_BasePrice(t *testing.T) {
	rules := ListingPricing{
		BasePriceMinor:   10000, // 100 EUR
		CleaningFeeMinor: 2000,  // 20 EUR
		Currency:         "EUR",
	}
	fees := FeeConfig{
		GuestFeeBPS: 1400,
		HostFeeBPS:  300,
	}

	checkIn := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	checkOut := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC) // 3 nights

	b, err := Quote(rules, nil, checkIn, checkOut, fees)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if b.NightCount != 3 {
		t.Errorf("expected 3 nights, got %d", b.NightCount)
	}
	if b.SubtotalMinor != 30000 {
		t.Errorf("expected subtotal 30000, got %d", b.SubtotalMinor)
	}
	if b.NightlyTotalNetMinor != 30000 {
		t.Errorf("expected net nightly 30000, got %d", b.NightlyTotalNetMinor)
	}
	if b.AccommodationSubtotalMinor != 32000 {
		t.Errorf("expected accommodation subtotal 32000, got %d", b.AccommodationSubtotalMinor)
	}
	// 32000 * 1400 / 10000 = 4480
	if b.GuestFeeMinor != 4480 {
		t.Errorf("expected guest fee 4480, got %d", b.GuestFeeMinor)
	}
	// guest total = 32000 + 4480 = 36480
	if b.GuestTotalMinor != 36480 {
		t.Errorf("expected guest total 36480, got %d", b.GuestTotalMinor)
	}
	// host fee = 32000 * 300 / 10000 = 960
	if b.HostFeeMinor != 960 {
		t.Errorf("expected host fee 960, got %d", b.HostFeeMinor)
	}
	// host net = 32000 - 960 = 31040
	if b.HostNetMinor != 31040 {
		t.Errorf("expected host net 31040, got %d", b.HostNetMinor)
	}
}

func TestPricingEngine_WeekendAndOverrides(t *testing.T) {
	weekendPrice := int64(15000)
	rules := ListingPricing{
		BasePriceMinor:    10000,
		CleaningFeeMinor:  0,
		Currency:          "EUR",
		WeekendPriceMinor: &weekendPrice,
		WeekendDays:       []int{5, 6}, // Fri, Sat
	}
	fees := FeeConfig{GuestFeeBPS: 1000, HostFeeBPS: 500}

	// Friday, Saturday, Sunday nights (check-in Friday Nov 6 2026, check-out Monday Nov 9 2026)
	checkIn := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	checkOut := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)

	// Override Sunday night (Nov 8) to 20000
	overrides := []DateOverride{
		{
			Night:      time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC),
			PriceMinor: 20000,
		},
	}

	b, err := Quote(rules, overrides, checkIn, checkOut, fees)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Nov 6 (Fri): 15000 (weekend)
	// Nov 7 (Sat): 15000 (weekend)
	// Nov 8 (Sun): 20000 (override)
	// Total: 50000
	if b.SubtotalMinor != 50000 {
		t.Errorf("expected 50000 subtotal, got %d", b.SubtotalMinor)
	}
}

func TestPricingEngine_Discounts(t *testing.T) {
	rules := ListingPricing{
		BasePriceMinor:     10000,
		WeeklyDiscountBPS:  1000, // 10%
		MonthlyDiscountBPS: 2000, // 20%
	}
	fees := FeeConfig{}

	// 7 nights -> weekly discount
	checkIn := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	checkOut := time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)

	b, err := Quote(rules, nil, checkIn, checkOut, fees)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// subtotal = 70000, 10% = 7000, net = 63000
	if b.DiscountMinor != 7000 || b.DiscountKind != "weekly" {
		t.Errorf("expected 7000 weekly discount, got %d (%s)", b.DiscountMinor, b.DiscountKind)
	}
	if b.NightlyTotalNetMinor != 63000 {
		t.Errorf("expected 63000 net, got %d", b.NightlyTotalNetMinor)
	}
}

func TestPricingEngine_MinNights(t *testing.T) {
	rules := ListingPricing{
		BasePriceMinor:   10000,
		CleaningFeeMinor: 2000,
		MinNights:        3,
	}
	fees := FeeConfig{GuestFeeBPS: 1400, HostFeeBPS: 300}

	checkIn := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	// 2 nights < base min 3 nights
	checkOutShort := time.Date(2026, 12, 3, 0, 0, 0, 0, time.UTC)
	_, err := Quote(rules, nil, checkIn, checkOutShort, fees)
	if !errors.Is(err, ErrMinNightsNotMet) {
		t.Errorf("expected ErrMinNightsNotMet for 2 nights, got %v", err)
	}

	// 3 nights meets base min
	checkOutValid := time.Date(2026, 12, 4, 0, 0, 0, 0, time.UTC)
	b, err := Quote(rules, nil, checkIn, checkOutValid, fees)
	if err != nil {
		t.Fatalf("unexpected error for 3 nights: %v", err)
	}
	if b.NightCount != 3 {
		t.Errorf("expected 3 nights, got %d", b.NightCount)
	}

	// Override tightens min_nights to 4
	minNights4 := 4
	overrides := []DateOverride{
		{
			Night:      time.Date(2026, 12, 2, 0, 0, 0, 0, time.UTC),
			PriceMinor: 12000,
			MinNights:  &minNights4,
		},
	}
	_, err = Quote(rules, overrides, checkIn, checkOutValid, fees)
	if !errors.Is(err, ErrMinNightsNotMet) {
		t.Errorf("expected ErrMinNightsNotMet when override tightens min to 4, got %v", err)
	}

	// 4 nights satisfies the override
	checkOut4 := time.Date(2026, 12, 5, 0, 0, 0, 0, time.UTC)
	b4, err := Quote(rules, overrides, checkIn, checkOut4, fees)
	if err != nil {
		t.Fatalf("unexpected error for 4 nights: %v", err)
	}
	if b4.NightCount != 4 {
		t.Errorf("expected 4 nights, got %d", b4.NightCount)
	}
}
