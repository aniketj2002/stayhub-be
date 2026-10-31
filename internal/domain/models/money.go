package models

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/enums"
)

// CancellationRefundBreakdown holds the deterministic server-side refund and host retention math.
type CancellationRefundBreakdown struct {
	PolicyCode                 string                 `json:"policy_code"`
	HoursBeforeCheckIn         int                    `json:"hours_before_check_in"`
	MatchedRule                CancellationPolicyRule `json:"matched_rule"`
	RefundPct                  int                    `json:"refund_pct"`
	RefundsServiceFee          bool                   `json:"refunds_service_fee"`
	RefundedAccommodationMinor int64                  `json:"refunded_accommodation_minor"`
	RefundedGuestFeeMinor      int64                  `json:"refunded_guest_fee_minor"`
	RefundAmountMinor          int64                  `json:"refund_amount_minor"`
	NonRefundableMinor         int64                  `json:"non_refundable_minor"`
	RetainedGrossMinor         int64                  `json:"retained_gross_minor"`
	RetainedHostFeeMinor       int64                  `json:"retained_host_fee_minor"`
	RetainedHostNetMinor       int64                  `json:"retained_host_net_minor"`
	RefundedHostFeeMinor       int64                  `json:"refunded_host_fee_minor"`
	RefundedHostNetMinor       int64                  `json:"refunded_host_net_minor"`
}

// ComputeFullRefund returns a 100% refund breakdown (used when pending booking is cancelled/declined/expired,
// or when a confirmed booking is cancelled by the host).
func ComputeFullRefund(res Reservation, cancelAt time.Time) CancellationRefundBreakdown {
	accommodationSubtotal := res.NightlyTotalMinor + res.CleaningFeeMinor
	hoursBefore := int(math.Floor(res.CheckInAt.Sub(cancelAt).Hours()))
	rule := CancellationPolicyRule{
		PolicyCode:        res.CancellationPolicy,
		MinHoursBefore:    0,
		RefundPct:         100,
		RefundsServiceFee: true,
	}
	return CancellationRefundBreakdown{
		PolicyCode:                 res.CancellationPolicy,
		HoursBeforeCheckIn:         hoursBefore,
		MatchedRule:                rule,
		RefundPct:                  100,
		RefundsServiceFee:          true,
		RefundedAccommodationMinor: accommodationSubtotal,
		RefundedGuestFeeMinor:      res.GuestFeeMinor,
		RefundAmountMinor:          res.GuestTotalMinor,
		NonRefundableMinor:         0,
		RetainedGrossMinor:         0,
		RetainedHostFeeMinor:       0,
		RetainedHostNetMinor:       0,
		RefundedHostFeeMinor:       res.HostFeeMinor,
		RefundedHostNetMinor:       res.HostNetMinor,
	}
}

// ComputePolicyRefund evaluates the reservation's cancellation policy rules at cancelAt.
// hours_before = floor((check_in_at - cancelAt) in hours).
// Picks the rule with the largest min_hours_before <= hours_before.
func ComputePolicyRefund(res Reservation, rules []CancellationPolicyRule, cancelAt time.Time) CancellationRefundBreakdown {
	hoursBefore := int(math.Floor(res.CheckInAt.Sub(cancelAt).Hours()))

	sorted := make([]CancellationPolicyRule, len(rules))
	copy(sorted, rules)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].MinHoursBefore > sorted[j].MinHoursBefore
	})

	matched := CancellationPolicyRule{
		PolicyCode:        res.CancellationPolicy,
		MinHoursBefore:    0,
		RefundPct:         0,
		RefundsServiceFee: false,
	}
	if hoursBefore >= 0 {
		for _, r := range sorted {
			if hoursBefore >= r.MinHoursBefore {
				matched = r
				break
			}
		}
	}

	accommodationSubtotal := res.NightlyTotalMinor + res.CleaningFeeMinor
	var refundedAccomm int64
	switch {
	case matched.RefundPct >= 100:
		refundedAccomm = accommodationSubtotal
	case matched.RefundPct <= 0:
		refundedAccomm = 0
	default:
		refundedAccomm = MulBPS(accommodationSubtotal, matched.RefundPct*100)
	}

	retainedGross := accommodationSubtotal - refundedAccomm
	var refundedGuestFee int64
	if matched.RefundsServiceFee {
		refundedGuestFee = res.GuestFeeMinor
	}

	refundAmount := refundedAccomm + refundedGuestFee
	nonRefundable := res.GuestTotalMinor - refundAmount

	retainedHostFee := MulBPS(retainedGross, res.HostFeeBPS)
	retainedHostNet := retainedGross - retainedHostFee

	refundedHostFee := res.HostFeeMinor - retainedHostFee
	refundedHostNet := res.HostNetMinor - retainedHostNet

	return CancellationRefundBreakdown{
		PolicyCode:                 res.CancellationPolicy,
		HoursBeforeCheckIn:         hoursBefore,
		MatchedRule:                matched,
		RefundPct:                  matched.RefundPct,
		RefundsServiceFee:          matched.RefundsServiceFee,
		RefundedAccommodationMinor: refundedAccomm,
		RefundedGuestFeeMinor:      refundedGuestFee,
		RefundAmountMinor:          refundAmount,
		NonRefundableMinor:         nonRefundable,
		RetainedGrossMinor:         retainedGross,
		RetainedHostFeeMinor:       retainedHostFee,
		RetainedHostNetMinor:       retainedHostNet,
		RefundedHostFeeMinor:       refundedHostFee,
		RefundedHostNetMinor:       refundedHostNet,
	}
}

// ComputeCustomRefund calculates a custom or discretionary refund breakdown that balances the ledger.
func ComputeCustomRefund(res Reservation, refundAmountMinor int64) CancellationRefundBreakdown {
	if refundAmountMinor <= 0 {
		return CancellationRefundBreakdown{}
	}
	if refundAmountMinor >= res.GuestTotalMinor {
		return ComputeFullRefund(res, time.Now())
	}

	accommodationSubtotal := res.NightlyTotalMinor + res.CleaningFeeMinor
	var refundedAccomm int64
	var refundedGuestFee int64

	if res.GuestTotalMinor > 0 {
		refundedAccomm = (accommodationSubtotal * refundAmountMinor) / res.GuestTotalMinor
		refundedGuestFee = refundAmountMinor - refundedAccomm
	}

	retainedGross := accommodationSubtotal - refundedAccomm
	retainedHostFee := MulBPS(retainedGross, res.HostFeeBPS)
	retainedHostNet := retainedGross - retainedHostFee

	refundedHostFee := res.HostFeeMinor - retainedHostFee
	refundedHostNet := refundedAccomm - refundedHostFee

	pct := int((refundAmountMinor * 100) / res.GuestTotalMinor)

	return CancellationRefundBreakdown{
		PolicyCode:                 res.CancellationPolicy,
		RefundPct:                  pct,
		RefundsServiceFee:          refundedGuestFee > 0,
		RefundedAccommodationMinor: refundedAccomm,
		RefundedGuestFeeMinor:      refundedGuestFee,
		RefundAmountMinor:          refundAmountMinor,
		NonRefundableMinor:         res.GuestTotalMinor - refundAmountMinor,
		RetainedGrossMinor:         retainedGross,
		RetainedHostFeeMinor:       retainedHostFee,
		RetainedHostNetMinor:       retainedHostNet,
		RefundedHostFeeMinor:       refundedHostFee,
		RefundedHostNetMinor:       refundedHostNet,
	}
}

// StayQuoteBreakdown holds exact minor-unit pricing and fee calculations.
type StayQuoteBreakdown struct {
	Nights                     int   `json:"nights"`
	NightlyRateMinor           int64 `json:"nightly_rate_minor"`
	NightlyTotalMinor          int64 `json:"nightly_total_minor"`
	CleaningFeeMinor           int64 `json:"cleaning_fee_minor"`
	AccommodationSubtotalMinor int64 `json:"accommodation_subtotal_minor"`
	GuestFeeBPS                int   `json:"guest_fee_bps"`
	GuestFeeMinor              int64 `json:"guest_fee_minor"`
	GuestTotalMinor            int64 `json:"guest_total_minor"`
	HostFeeBPS                 int   `json:"host_fee_bps"`
	HostFeeMinor               int64 `json:"host_fee_minor"`
	HostNetMinor               int64 `json:"host_net_minor"`
}

// MulBPS multiplies a non-negative minor-unit amount by basis points using half-up integer rounding:
// floor((x * bps + 5000) / 10000).
func MulBPS(amountMinor int64, bps int) int64 {
	if amountMinor <= 0 || bps <= 0 {
		return 0
	}
	return (amountMinor*int64(bps) + 5000) / 10000
}

// ComputeStayQuote calculates exact guest total and host net payout in integer minor units.
func ComputeStayQuote(basePriceMinor, cleaningFeeMinor int64, nights, guestFeeBPS, hostFeeBPS int) StayQuoteBreakdown {
	nightlyTotal := basePriceMinor * int64(nights)
	subtotal := nightlyTotal + cleaningFeeMinor
	guestFee := MulBPS(subtotal, guestFeeBPS)
	guestTotal := subtotal + guestFee
	hostFee := MulBPS(subtotal, hostFeeBPS)
	hostNet := subtotal - hostFee

	return StayQuoteBreakdown{
		Nights:                     nights,
		NightlyRateMinor:           basePriceMinor,
		NightlyTotalMinor:          nightlyTotal,
		CleaningFeeMinor:           cleaningFeeMinor,
		AccommodationSubtotalMinor: subtotal,
		GuestFeeBPS:                guestFeeBPS,
		GuestFeeMinor:              guestFee,
		GuestTotalMinor:            guestTotal,
		HostFeeBPS:                 hostFeeBPS,
		HostFeeMinor:               hostFee,
		HostNetMinor:               hostNet,
	}
}

// EnumerateNights returns every date in [checkIn, checkOut) normalized to UTC midnight.
func EnumerateNights(checkIn, checkOut time.Time) []time.Time {
	start := time.Date(checkIn.Year(), checkIn.Month(), checkIn.Day(), 0, 0, 0, 0, time.UTC)
	end := time.Date(checkOut.Year(), checkOut.Month(), checkOut.Day(), 0, 0, 0, 0, time.UTC)
	if !end.After(start) {
		return nil
	}
	var nights []time.Time
	for cur := start; cur.Before(end); cur = cur.AddDate(0, 0, 1) {
		nights = append(nights, cur)
	}
	return nights
}

// CombineLocalDateAndTime combines a calendar date and "HH:MM" (or "HH:MM:SS") time in an IANA timezone.
func CombineLocalDateAndTime(dateVal time.Time, hhmm string, ianaTimezone string) (time.Time, error) {
	loc, err := time.LoadLocation(ianaTimezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timezone %q: %w", ianaTimezone, err)
	}
	parts := strings.Split(strings.TrimSpace(hhmm), ":")
	if len(parts) < 2 || len(parts) > 3 {
		return time.Time{}, fmt.Errorf("invalid time format %q (expected HH:MM)", hhmm)
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return time.Time{}, fmt.Errorf("invalid hour in %q", hhmm)
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return time.Time{}, fmt.Errorf("invalid minute in %q", hhmm)
	}
	sec := 0
	if len(parts) == 3 {
		sec, err = strconv.Atoi(parts[2])
		if err != nil || sec < 0 || sec > 59 {
			return time.Time{}, fmt.Errorf("invalid second in %q", hhmm)
		}
	}
	return time.Date(dateVal.Year(), dateVal.Month(), dateVal.Day(), hour, minute, sec, 0, loc), nil
}

// Payment maps to the `payments` table.
type Payment struct {
	ID               uuid.UUID           `db:"id" json:"id"`
	ReservationID    uuid.UUID           `db:"reservation_id" json:"reservation_id"`
	Kind             enums.PaymentKind   `db:"kind" json:"kind"`
	Status           enums.PaymentStatus `db:"status" json:"status"`
	AmountMinor      int64               `db:"amount_minor" json:"amount_minor"`
	Currency         string              `db:"currency" json:"currency"`
	IdempotencyKey   string              `db:"idempotency_key" json:"idempotency_key"`
	SimulatedRef     *string             `db:"simulated_ref" json:"simulated_ref,omitempty"`
	RelatedPaymentID *uuid.UUID          `db:"related_payment_id" json:"related_payment_id,omitempty"`
	RefundPct        *int                `db:"refund_pct" json:"refund_pct,omitempty"`
	RefundBasis      json.RawMessage     `db:"refund_basis" json:"refund_basis,omitempty"`
	FailureReason    *string             `db:"failure_reason" json:"failure_reason,omitempty"`
	CreatedAt        time.Time           `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time           `db:"updated_at" json:"updated_at"`
}

// Payout maps to the `payouts` table.
type Payout struct {
	ID             uuid.UUID          `db:"id" json:"id"`
	ReservationID  uuid.UUID          `db:"reservation_id" json:"reservation_id"`
	HostID         uuid.UUID          `db:"host_id" json:"host_id"`
	Status         enums.PayoutStatus `db:"status" json:"status"`
	GrossMinor     int64              `db:"gross_minor" json:"gross_minor"`
	HostFeeMinor   int64              `db:"host_fee_minor" json:"host_fee_minor"`
	NetMinor       int64              `db:"net_minor" json:"net_minor"`
	Currency       string             `db:"currency" json:"currency"`
	ReleaseAt      *time.Time         `db:"release_at" json:"release_at,omitempty"`
	ReleasedAt     *time.Time         `db:"released_at" json:"released_at,omitempty"`
	HeldReason     *string            `db:"held_reason" json:"held_reason,omitempty"`
	HeldBy         *uuid.UUID         `db:"held_by" json:"held_by,omitempty"`
	IdempotencyKey string             `db:"idempotency_key" json:"idempotency_key"`
	SimulatedRef   *string            `db:"simulated_ref" json:"simulated_ref,omitempty"`
	CreatedAt      time.Time          `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time          `db:"updated_at" json:"updated_at"`
}

// LedgerEntry maps to the `ledger_entries` table.
type LedgerEntry struct {
	ID            int64                 `db:"id" json:"id"`
	TxnID         uuid.UUID             `db:"txn_id" json:"txn_id"`
	ReservationID *uuid.UUID            `db:"reservation_id" json:"reservation_id,omitempty"`
	PaymentID     *uuid.UUID            `db:"payment_id" json:"payment_id,omitempty"`
	PayoutID      *uuid.UUID            `db:"payout_id" json:"payout_id,omitempty"`
	Account       string                `db:"account" json:"account"`
	Direction     enums.LedgerDirection `db:"direction" json:"direction"`
	AmountMinor   int64                 `db:"amount_minor" json:"amount_minor"`
	Currency      string                `db:"currency" json:"currency"`
	CreatedAt     time.Time             `db:"created_at" json:"created_at"`
}

// IdempotencyKey maps to the `idempotency_keys` table.
type IdempotencyKey struct {
	Scope        string          `db:"scope" json:"scope"`
	Key          string          `db:"key" json:"key"`
	SubjectID    *uuid.UUID      `db:"subject_id" json:"subject_id,omitempty"`
	RequestHash  string          `db:"request_hash" json:"request_hash"`
	ResponseCode *int            `db:"response_code" json:"response_code,omitempty"`
	ResponseBody json.RawMessage `db:"response_body" json:"response_body,omitempty"`
	CreatedAt    time.Time       `db:"created_at" json:"created_at"`
}
