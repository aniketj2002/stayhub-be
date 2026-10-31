package pricing

import (
	"errors"
	"time"

	"github.com/stayhub/backend/internal/domain/models"
)

var (
	ErrInvalidDates     = errors.New("check-out date must be after check-in date")
	ErrNegativePrice    = errors.New("base price must not be negative")
	ErrNegativeCleaning = errors.New("cleaning fee must not be negative")
	ErrMinNightsNotMet  = errors.New("stay duration does not meet minimum nights requirement")
)

type ListingPricing struct {
	BasePriceMinor     int64
	CleaningFeeMinor   int64
	Currency           string
	WeekendPriceMinor  *int64
	WeekendDays        []int
	MinNights          int
	WeeklyDiscountBPS  int
	MonthlyDiscountBPS int
}

type DateOverride struct {
	Night      time.Time
	PriceMinor int64
	MinNights  *int
}

type FeeConfig struct {
	GuestFeeBPS int
	HostFeeBPS  int
}

type NightPrice struct {
	Night      time.Time `json:"night"`
	PriceMinor int64     `json:"price_minor"`
	Source     string    `json:"source"` // "base", "weekend", "override"
}

type Breakdown struct {
	Nights                     []NightPrice `json:"nights"`
	NightCount                 int          `json:"night_count"`
	SubtotalMinor              int64        `json:"subtotal_minor"`
	DiscountMinor              int64        `json:"discount_minor"`
	DiscountKind               string       `json:"discount_kind,omitempty"`
	NightlyTotalNetMinor       int64        `json:"nightly_total_net_minor"`
	CleaningFeeMinor           int64        `json:"cleaning_fee_minor"`
	AccommodationSubtotalMinor int64        `json:"accommodation_subtotal_minor"`
	GuestFeeBPS                int          `json:"guest_fee_bps"`
	GuestFeeMinor              int64        `json:"guest_fee_minor"`
	GuestTotalMinor            int64        `json:"guest_total_minor"`
	HostFeeBPS                 int          `json:"host_fee_bps"`
	HostFeeMinor               int64        `json:"host_fee_minor"`
	HostNetMinor               int64        `json:"host_net_minor"`
	Currency                   string       `json:"currency"`
}

// Quote calculates the stay pricing breakdown using the pure pricing engine.
func Quote(
	rules ListingPricing,
	overrides []DateOverride,
	checkIn, checkOut time.Time,
	fees FeeConfig,
) (Breakdown, error) {
	nightsList := models.EnumerateNights(checkIn, checkOut)
	if len(nightsList) == 0 {
		return Breakdown{}, ErrInvalidDates
	}
	if rules.BasePriceMinor < 0 {
		return Breakdown{}, ErrNegativePrice
	}
	if rules.CleaningFeeMinor < 0 {
		return Breakdown{}, ErrNegativeCleaning
	}

	overrideMap := make(map[string]DateOverride, len(overrides))
	for _, o := range overrides {
		key := o.Night.UTC().Format("2006-01-02")
		overrideMap[key] = o
	}

	weekendDaysMap := make(map[time.Weekday]bool)
	if len(rules.WeekendDays) > 0 {
		for _, wd := range rules.WeekendDays {
			weekendDaysMap[time.Weekday(wd)] = true
		}
	} else {
		// default Friday (5) & Saturday (6)
		weekendDaysMap[time.Friday] = true
		weekendDaysMap[time.Saturday] = true
	}

	var nightPrices []NightPrice
	var subtotalMinor int64

	requiredMinNights := rules.MinNights
	if requiredMinNights < 1 {
		requiredMinNights = 1
	}

	for _, n := range nightsList {
		key := n.UTC().Format("2006-01-02")
		if ovr, exists := overrideMap[key]; exists {
			if ovr.MinNights != nil && *ovr.MinNights > requiredMinNights {
				requiredMinNights = *ovr.MinNights
			}
			nightPrices = append(nightPrices, NightPrice{
				Night:      n,
				PriceMinor: ovr.PriceMinor,
				Source:     "override",
			})
			subtotalMinor += ovr.PriceMinor
		} else if rules.WeekendPriceMinor != nil && weekendDaysMap[n.Weekday()] {
			nightPrices = append(nightPrices, NightPrice{
				Night:      n,
				PriceMinor: *rules.WeekendPriceMinor,
				Source:     "weekend",
			})
			subtotalMinor += *rules.WeekendPriceMinor
		} else {
			nightPrices = append(nightPrices, NightPrice{
				Night:      n,
				PriceMinor: rules.BasePriceMinor,
				Source:     "base",
			})
			subtotalMinor += rules.BasePriceMinor
		}
	}

	nightCount := len(nightsList)
	if nightCount < requiredMinNights {
		return Breakdown{}, ErrMinNightsNotMet
	}
	var discountMinor int64
	var discountKind string

	// Length-of-stay discounts: monthly (>= 28 nights) beats weekly (>= 7 nights)
	if nightCount >= 28 && rules.MonthlyDiscountBPS > 0 {
		discountMinor = models.MulBPS(subtotalMinor, rules.MonthlyDiscountBPS)
		discountKind = "monthly"
	} else if nightCount >= 7 && rules.WeeklyDiscountBPS > 0 {
		discountMinor = models.MulBPS(subtotalMinor, rules.WeeklyDiscountBPS)
		discountKind = "weekly"
	}

	nightlyTotalNet := subtotalMinor - discountMinor
	if nightlyTotalNet < 0 {
		nightlyTotalNet = 0
	}

	accommodationSubtotal := nightlyTotalNet + rules.CleaningFeeMinor
	guestFee := models.MulBPS(accommodationSubtotal, fees.GuestFeeBPS)
	guestTotal := accommodationSubtotal + guestFee
	hostFee := models.MulBPS(accommodationSubtotal, fees.HostFeeBPS)
	hostNet := accommodationSubtotal - hostFee

	currency := rules.Currency
	if currency == "" {
		currency = "EUR"
	}

	return Breakdown{
		Nights:                     nightPrices,
		NightCount:                 nightCount,
		SubtotalMinor:              subtotalMinor,
		DiscountMinor:              discountMinor,
		DiscountKind:               discountKind,
		NightlyTotalNetMinor:       nightlyTotalNet,
		CleaningFeeMinor:           rules.CleaningFeeMinor,
		AccommodationSubtotalMinor: accommodationSubtotal,
		GuestFeeBPS:                fees.GuestFeeBPS,
		GuestFeeMinor:              guestFee,
		GuestTotalMinor:            guestTotal,
		HostFeeBPS:                 fees.HostFeeBPS,
		HostFeeMinor:               hostFee,
		HostNetMinor:               hostNet,
		Currency:                   currency,
	}, nil
}
