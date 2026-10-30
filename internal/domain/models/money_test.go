package models

import (
	"testing"
	"time"
)

func TestComputeStayQuoteAndMulBPS(t *testing.T) {
	// 3 nights at €120.00 (12000 cents) + €35.00 cleaning (3500 cents)
	// Subtotal = 36000 + 3500 = 39500 cents
	// Guest Fee (14% = 1400 bps) = (39500 * 1400 + 5000)/10000 = 5530 cents (€55.30)
	// Guest Total = 39500 + 5530 = 45030 cents (€450.30)
	// Host Fee (3% = 300 bps) = (39500 * 300 + 5000)/10000 = 1185 cents (€11.85)
	// Host Net = 39500 - 1185 = 38315 cents (€383.15)
	q := ComputeStayQuote(12000, 3500, 3, 1400, 300)

	if q.NightlyTotalMinor != 36000 {
		t.Fatalf("expected nightly_total_minor 36000, got %d", q.NightlyTotalMinor)
	}
	if q.AccommodationSubtotalMinor != 39500 {
		t.Fatalf("expected subtotal 39500, got %d", q.AccommodationSubtotalMinor)
	}
	if q.GuestFeeMinor != 5530 {
		t.Fatalf("expected guest_fee_minor 5530, got %d", q.GuestFeeMinor)
	}
	if q.GuestTotalMinor != 45030 {
		t.Fatalf("expected guest_total_minor 45030, got %d", q.GuestTotalMinor)
	}
	if q.HostFeeMinor != 1185 {
		t.Fatalf("expected host_fee_minor 1185, got %d", q.HostFeeMinor)
	}
	if q.HostNetMinor != 38315 {
		t.Fatalf("expected host_net_minor 38315, got %d", q.HostNetMinor)
	}

	// Verify ledger identity: guest_fee + host_fee + host_net == guest_total
	if q.GuestFeeMinor+q.HostFeeMinor+q.HostNetMinor != q.GuestTotalMinor {
		t.Fatalf("ledger identity violated: %d + %d + %d != %d",
			q.GuestFeeMinor, q.HostFeeMinor, q.HostNetMinor, q.GuestTotalMinor)
	}
}

func TestEnumerateNightsAndTimezoneCombine(t *testing.T) {
	checkIn, _ := time.Parse("2006-01-02", "2026-11-10")
	checkOut, _ := time.Parse("2006-01-02", "2026-11-14")

	nights := EnumerateNights(checkIn, checkOut)
	if len(nights) != 4 {
		t.Fatalf("expected 4 nights, got %d", len(nights))
	}
	if nights[0].Format("2006-01-02") != "2026-11-10" || nights[3].Format("2006-01-02") != "2026-11-13" {
		t.Fatalf("unexpected night boundaries: %v", nights)
	}

	checkInAt, err := CombineLocalDateAndTime(checkIn, "15:00", "Europe/Lisbon")
	if err != nil {
		t.Fatalf("unexpected error combining local date and time: %v", err)
	}
	if checkInAt.Hour() != 15 || checkInAt.Minute() != 0 {
		t.Fatalf("expected 15:00 local time, got %v", checkInAt)
	}
}

func TestComputePolicyRefund_AllFourPoliciesAtBoundaryHours(t *testing.T) {
	// 3 nights @ 12000 + 3500 cleaning = 39500 subtotal
	// GuestFee = 5530, GuestTotal = 45030, HostFee = 1185, HostNet = 38315
	baseRes := Reservation{
		NightlyTotalMinor: 36000,
		CleaningFeeMinor:  3500,
		GuestFeeMinor:     5530,
		HostFeeMinor:      1185,
		GuestTotalMinor:   45030,
		HostNetMinor:      38315,
		GuestFeeBPS:       1400,
		HostFeeBPS:        300,
		CheckInAt:         time.Date(2026, 12, 20, 15, 0, 0, 0, time.UTC),
	}

	rulesByPolicy := map[string][]CancellationPolicyRule{
		"flexible": {
			{PolicyCode: "flexible", MinHoursBefore: 24, RefundPct: 100, RefundsServiceFee: true},
			{PolicyCode: "flexible", MinHoursBefore: 0, RefundPct: 0, RefundsServiceFee: false},
		},
		"moderate": {
			{PolicyCode: "moderate", MinHoursBefore: 120, RefundPct: 100, RefundsServiceFee: true},
			{PolicyCode: "moderate", MinHoursBefore: 24, RefundPct: 50, RefundsServiceFee: false},
			{PolicyCode: "moderate", MinHoursBefore: 0, RefundPct: 0, RefundsServiceFee: false},
		},
		"firm": {
			{PolicyCode: "firm", MinHoursBefore: 720, RefundPct: 100, RefundsServiceFee: true},
			{PolicyCode: "firm", MinHoursBefore: 168, RefundPct: 50, RefundsServiceFee: false},
			{PolicyCode: "firm", MinHoursBefore: 0, RefundPct: 0, RefundsServiceFee: false},
		},
		"strict": {
			{PolicyCode: "strict", MinHoursBefore: 336, RefundPct: 50, RefundsServiceFee: false},
			{PolicyCode: "strict", MinHoursBefore: 0, RefundPct: 0, RefundsServiceFee: false},
		},
	}

	cases := []struct {
		name               string
		policy             string
		offsetBeforeCheck  time.Duration
		wantRefundPct      int
		wantServiceFeeRef  bool
		wantRefundMinor    int64
		wantRetainedGross  int64
		wantRetainedNet    int64
	}{
		// Flexible boundaries (24h)
		{
			name:              "flexible_exact_24h_full_refund",
			policy:            "flexible",
			offsetBeforeCheck: 24 * time.Hour,
			wantRefundPct:     100,
			wantServiceFeeRef: true,
			wantRefundMinor:   45030,
			wantRetainedGross: 0,
			wantRetainedNet:   0,
		},
		{
			name:              "flexible_23h59m_zero_refund",
			policy:            "flexible",
			offsetBeforeCheck: 23*time.Hour + 59*time.Minute,
			wantRefundPct:     0,
			wantServiceFeeRef: false,
			wantRefundMinor:   0,
			wantRetainedGross: 39500,
			wantRetainedNet:   38315,
		},
		// Moderate boundaries (120h and 24h)
		{
			name:              "moderate_exact_120h_full_refund",
			policy:            "moderate",
			offsetBeforeCheck: 120 * time.Hour,
			wantRefundPct:     100,
			wantServiceFeeRef: true,
			wantRefundMinor:   45030,
			wantRetainedGross: 0,
			wantRetainedNet:   0,
		},
		{
			name:              "moderate_119h59m_partial_50pct_no_service_fee",
			policy:            "moderate",
			offsetBeforeCheck: 119*time.Hour + 59*time.Minute,
			wantRefundPct:     50,
			wantServiceFeeRef: false,
			wantRefundMinor:   19750,
			wantRetainedGross: 19750,
			wantRetainedNet:   19157,
		},
		{
			name:              "moderate_exact_24h_partial_50pct",
			policy:            "moderate",
			offsetBeforeCheck: 24 * time.Hour,
			wantRefundPct:     50,
			wantServiceFeeRef: false,
			wantRefundMinor:   19750,
			wantRetainedGross: 19750,
			wantRetainedNet:   19157,
		},
		{
			name:              "moderate_23h59m_zero_refund",
			policy:            "moderate",
			offsetBeforeCheck: 23*time.Hour + 59*time.Minute,
			wantRefundPct:     0,
			wantServiceFeeRef: false,
			wantRefundMinor:   0,
			wantRetainedGross: 39500,
			wantRetainedNet:   38315,
		},
		// Firm boundaries (720h and 168h)
		{
			name:              "firm_exact_720h_full_refund",
			policy:            "firm",
			offsetBeforeCheck: 720 * time.Hour,
			wantRefundPct:     100,
			wantServiceFeeRef: true,
			wantRefundMinor:   45030,
			wantRetainedGross: 0,
			wantRetainedNet:   0,
		},
		{
			name:              "firm_719h59m_partial_50pct",
			policy:            "firm",
			offsetBeforeCheck: 719*time.Hour + 59*time.Minute,
			wantRefundPct:     50,
			wantServiceFeeRef: false,
			wantRefundMinor:   19750,
			wantRetainedGross: 19750,
			wantRetainedNet:   19157,
		},
		{
			name:              "firm_exact_168h_partial_50pct",
			policy:            "firm",
			offsetBeforeCheck: 168 * time.Hour,
			wantRefundPct:     50,
			wantServiceFeeRef: false,
			wantRefundMinor:   19750,
			wantRetainedGross: 19750,
			wantRetainedNet:   19157,
		},
		{
			name:              "firm_167h59m_zero_refund",
			policy:            "firm",
			offsetBeforeCheck: 167*time.Hour + 59*time.Minute,
			wantRefundPct:     0,
			wantServiceFeeRef: false,
			wantRefundMinor:   0,
			wantRetainedGross: 39500,
			wantRetainedNet:   38315,
		},
		// Strict boundaries (336h)
		{
			name:              "strict_exact_336h_partial_50pct",
			policy:            "strict",
			offsetBeforeCheck: 336 * time.Hour,
			wantRefundPct:     50,
			wantServiceFeeRef: false,
			wantRefundMinor:   19750,
			wantRetainedGross: 19750,
			wantRetainedNet:   19157,
		},
		{
			name:              "strict_335h59m_zero_refund",
			policy:            "strict",
			offsetBeforeCheck: 335*time.Hour + 59*time.Minute,
			wantRefundPct:     0,
			wantServiceFeeRef: false,
			wantRefundMinor:   0,
			wantRetainedGross: 39500,
			wantRetainedNet:   38315,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := baseRes
			res.CancellationPolicy = tc.policy
			cancelAt := res.CheckInAt.Add(-tc.offsetBeforeCheck)

			rb := ComputePolicyRefund(res, rulesByPolicy[tc.policy], cancelAt)
			if rb.RefundPct != tc.wantRefundPct {
				t.Fatalf("expected RefundPct=%d, got %d", tc.wantRefundPct, rb.RefundPct)
			}
			if rb.RefundsServiceFee != tc.wantServiceFeeRef {
				t.Fatalf("expected RefundsServiceFee=%v, got %v", tc.wantServiceFeeRef, rb.RefundsServiceFee)
			}
			if rb.RefundAmountMinor != tc.wantRefundMinor {
				t.Fatalf("expected RefundAmountMinor=%d, got %d", tc.wantRefundMinor, rb.RefundAmountMinor)
			}
			if rb.RetainedGrossMinor != tc.wantRetainedGross {
				t.Fatalf("expected RetainedGrossMinor=%d, got %d", tc.wantRetainedGross, rb.RetainedGrossMinor)
			}
			if rb.RetainedHostNetMinor != tc.wantRetainedNet {
				t.Fatalf("expected RetainedHostNetMinor=%d, got %d", tc.wantRetainedNet, rb.RetainedHostNetMinor)
			}
			// Verify ledger identity on refund legs: RefundedGuestFee + RefundedHostFee + RefundedHostNet == RefundAmount
			sumRefundLegs := rb.RefundedGuestFeeMinor + rb.RefundedHostFeeMinor + rb.RefundedHostNetMinor
			if sumRefundLegs != rb.RefundAmountMinor {
				t.Fatalf("refund ledger legs sum (%d) != RefundAmountMinor (%d): %+v", sumRefundLegs, rb.RefundAmountMinor, rb)
			}
		})
	}
}
