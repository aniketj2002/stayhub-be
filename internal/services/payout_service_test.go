package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/config"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/db"
)

func TestPayoutService_HoldAndReleaseTiming(t *testing.T) {
	cfg := &config.Config{
		PayoutReleaseAfterCheckInHours: 24,
	}
	payRepo := newMemPaymentRepo()
	resRepo := newMemReservationRepo()
	now := time.Date(2026, 12, 1, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMockClock(now)

	payoutSvc := NewPayoutService(cfg, &db.DB{}, nil, payRepo, resRepo, nil, nil, clk)
	ctx := context.Background()
	adminID := uuid.New()
	resID := uuid.New()

	checkInAt := now.Add(-10 * time.Hour) // checked in 10 hours ago -> 24h mark is in future (+14h from now)
	res := models.Reservation{
		ID:          resID,
		Status:      enums.ReservationStatusCheckedIn,
		CheckInAt:   checkInAt,
		CheckedInAt: &checkInAt,
	}
	resRepo.rows[resID] = res

	payoutID := uuid.New()
	releaseAtInit := checkInAt.Add(24 * time.Hour)
	payout := models.Payout{
		ID:            payoutID,
		ReservationID: resID,
		HostID:        uuid.New(),
		GrossMinor:    53000,
		HostFeeMinor:  3000,
		NetMinor:      50000,
		Currency:      "EUR",
		Status:        enums.PayoutStatusScheduled,
		ReleaseAt:     &releaseAtInit,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	payRepo.payouts[payoutID] = payout

	// 1. Hold requires non-empty reason
	_, err := payoutSvc.HoldPayout(ctx, adminID, payoutID, "   ")
	if !errors.Is(err, apperrors.ErrValidation) {
		t.Fatalf("expected ErrValidation on whitespace reason, got %v", err)
	}

	// 2. Hold payout succeeds
	held, err := payoutSvc.HoldPayout(ctx, adminID, payoutID, "Suspected fraud investigation")
	if err != nil {
		t.Fatalf("unexpected error holding payout: %v", err)
	}
	if held.Status != enums.PayoutStatusHeld {
		t.Errorf("expected status %s, got %s", enums.PayoutStatusHeld, held.Status)
	}
	if held.HeldBy == nil || *held.HeldBy != adminID {
		t.Errorf("expected held_by %s, got %v", adminID, held.HeldBy)
	}
	if held.HeldReason == nil || *held.HeldReason != "Suspected fraud investigation" {
		t.Errorf("expected held_reason, got %v", held.HeldReason)
	}

	// 3. Release hold when checked_in_at + 24h is still in the future
	rel1, err := payoutSvc.ReleasePayoutHold(ctx, adminID, payoutID, "Cleared by Trust & Safety")
	if err != nil {
		t.Fatalf("unexpected error releasing payout: %v", err)
	}
	if rel1.Status != enums.PayoutStatusScheduled {
		t.Errorf("expected status %s, got %s", enums.PayoutStatusScheduled, rel1.Status)
	}
	if rel1.HeldBy != nil || rel1.HeldReason != nil {
		t.Errorf("expected held_by and held_reason to be cleared")
	}
	// release_at should still be checkInAt + 24h since that is after current now
	expectedRelease1 := checkInAt.Add(24 * time.Hour)
	if !rel1.ReleaseAt.Equal(expectedRelease1) {
		t.Errorf("expected release_at %v, got %v", expectedRelease1, *rel1.ReleaseAt)
	}

	// 4. Put on hold again
	_, err = payoutSvc.HoldPayout(ctx, adminID, payoutID, "Second check")
	if err != nil {
		t.Fatalf("unexpected error holding payout again: %v", err)
	}

	// Fast-forward clock to 48 hours after check-in (well past the 24h threshold)
	past24hNow := checkInAt.Add(48 * time.Hour)
	clk.Set(past24hNow)

	// Release hold now: release_at should be max(now, checked_in_at + 24h) = past24hNow
	rel2, err := payoutSvc.ReleasePayoutHold(ctx, adminID, payoutID, "Cleared after delay")
	if err != nil {
		t.Fatalf("unexpected error releasing payout: %v", err)
	}
	if !rel2.ReleaseAt.Equal(past24hNow) {
		t.Errorf("expected release_at %v (clamped to now), got %v", past24hNow, *rel2.ReleaseAt)
	}

	// 5. Calling release on non-held payout fails with ErrPayoutNotHeld
	_, err = payoutSvc.ReleasePayoutHold(ctx, adminID, payoutID, "Should fail")
	if !errors.Is(err, ErrPayoutNotHeld) {
		t.Errorf("expected ErrPayoutNotHeld, got %v", err)
	}
}

func TestPayoutService_AdjustPayoutOnRefund(t *testing.T) {
	cfg := &config.Config{PayoutReleaseAfterCheckInHours: 24}
	payRepo := newMemPaymentRepo()
	resRepo := newMemReservationRepo()
	now := time.Date(2026, 12, 1, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMockClock(now)

	payoutSvc := NewPayoutService(cfg, &db.DB{}, nil, payRepo, resRepo, nil, nil, clk)
	ctx := context.Background()
	resID := uuid.New()

	payoutID := uuid.New()
	payout := models.Payout{
		ID:            payoutID,
		ReservationID: resID,
		HostID:        uuid.New(),
		GrossMinor:    53000,
		HostFeeMinor:  3000,
		NetMinor:      50000,
		Currency:      "EUR",
		Status:        enums.PayoutStatusScheduled,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	payRepo.payouts[payoutID] = payout

	res := models.Reservation{ID: resID}

	// 1. Partial refund reduces payout amount
	err := payoutSvc.AdjustPayoutOnRefund(ctx, res, 20000)
	if err != nil {
		t.Fatalf("unexpected adjust error: %v", err)
	}
	p1 := payRepo.payouts[payoutID]
	if p1.NetMinor != 30000 {
		t.Errorf("expected net minor 30000, got %d", p1.NetMinor)
	}
	if p1.GrossMinor != 33000 {
		t.Errorf("expected gross minor 33000, got %d", p1.GrossMinor)
	}
	if p1.Status != enums.PayoutStatusScheduled {
		t.Errorf("expected status %s, got %s", enums.PayoutStatusScheduled, p1.Status)
	}

	// 2. Further refund exceeding or matching remainder cancels payout
	err = payoutSvc.AdjustPayoutOnRefund(ctx, res, 30000)
	if err != nil {
		t.Fatalf("unexpected adjust error: %v", err)
	}
	p2 := payRepo.payouts[payoutID]
	if p2.NetMinor != 0 {
		t.Errorf("expected net minor 0, got %d", p2.NetMinor)
	}
	if p2.Status != enums.PayoutStatusCancelled {
		t.Errorf("expected status %s, got %s", enums.PayoutStatusCancelled, p2.Status)
	}
}

type testSchedulerStub struct {
	scheduled map[uuid.UUID]time.Time
	cancelled map[uuid.UUID]bool
	fail      bool
}

func newTestSchedulerStub() *testSchedulerStub {
	return &testSchedulerStub{
		scheduled: make(map[uuid.UUID]time.Time),
		cancelled: make(map[uuid.UUID]bool),
	}
}

func (s *testSchedulerStub) SchedulePayout(_ context.Context, payoutID, reservationID uuid.UUID, releaseAt time.Time) error {
	if s.fail {
		return errors.New("eventbridge failure")
	}
	s.scheduled[payoutID] = releaseAt
	return nil
}

func (s *testSchedulerStub) CancelPayoutSchedule(_ context.Context, payoutID uuid.UUID) error {
	if s.fail {
		return errors.New("eventbridge delete failure")
	}
	s.cancelled[payoutID] = true
	return nil
}

func TestPayoutService_ProcessPayoutRelease(t *testing.T) {
	cfg := &config.Config{PayoutReleaseAfterCheckInHours: 24}
	payRepo := newMemPaymentRepo()
	resRepo := newMemReservationRepo()
	now := time.Date(2026, 12, 1, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMockClock(now)

	payoutSvc := NewPayoutService(cfg, &db.DB{}, nil, payRepo, resRepo, nil, nil, clk)
	ctx := context.Background()

	pID := uuid.New()
	rID := uuid.New()
	payout := models.Payout{
		ID:            pID,
		ReservationID: rID,
		HostID:        uuid.New(),
		GrossMinor:    53000,
		HostFeeMinor:  3000,
		NetMinor:      50000,
		Currency:      "EUR",
		Status:        enums.PayoutStatusScheduled,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	payRepo.payouts[pID] = payout

	// 1. Process release on scheduled payout
	err := payoutSvc.ProcessPayoutRelease(ctx, pID)
	if err != nil {
		t.Fatalf("unexpected process payout release error: %v", err)
	}

	released := payRepo.payouts[pID]
	if released.Status != enums.PayoutStatusReleased {
		t.Errorf("expected status %s, got %s", enums.PayoutStatusReleased, released.Status)
	}
	if released.ReleasedAt == nil || !released.ReleasedAt.Equal(now) {
		t.Errorf("expected released_at %v, got %v", now, released.ReleasedAt)
	}
	if released.SimulatedRef == nil || *released.SimulatedRef == "" {
		t.Errorf("expected non-empty simulated ref, got %v", released.SimulatedRef)
	}

	// 2. Idempotent re-run on already released payout
	err = payoutSvc.ProcessPayoutRelease(ctx, pID)
	if err != nil {
		t.Fatalf("expected nil error on idempotent release, got %v", err)
	}

	// 3. Payout marked cancelled should not be released
	cancelPID := uuid.New()
	payRepo.payouts[cancelPID] = models.Payout{
		ID:        cancelPID,
		Status:    enums.PayoutStatusCancelled,
		CreatedAt: now,
		UpdatedAt: now,
	}
	err = payoutSvc.ProcessPayoutRelease(ctx, cancelPID)
	if err != nil {
		t.Fatalf("expected nil error on cancelled payout, got %v", err)
	}
	if payRepo.payouts[cancelPID].Status != enums.PayoutStatusCancelled {
		t.Errorf("expected status to remain cancelled, got %s", payRepo.payouts[cancelPID].Status)
	}
}

func TestPayoutService_HoldAndReleaseEventBridge(t *testing.T) {
	cfg := &config.Config{PayoutReleaseAfterCheckInHours: 24}
	payRepo := newMemPaymentRepo()
	resRepo := newMemReservationRepo()
	now := time.Date(2026, 12, 1, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMockClock(now)
	stubSched := newTestSchedulerStub()

	payoutSvc := NewPayoutService(cfg, &db.DB{}, nil, payRepo, resRepo, nil, nil, clk, stubSched)
	ctx := context.Background()
	adminID := uuid.New()
	resID := uuid.New()
	pID := uuid.New()

	checkInAt := now.Add(-5 * time.Hour)
	resRepo.rows[resID] = models.Reservation{
		ID:          resID,
		Status:      enums.ReservationStatusCheckedIn,
		CheckInAt:   checkInAt,
		CheckedInAt: &checkInAt,
	}
	payRepo.payouts[pID] = models.Payout{
		ID:            pID,
		ReservationID: resID,
		Status:        enums.PayoutStatusScheduled,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// Hold payout should trigger EventBridge cancel
	_, err := payoutSvc.HoldPayout(ctx, adminID, pID, "Hold check")
	if err != nil {
		t.Fatalf("unexpected hold error: %v", err)
	}
	if !stubSched.cancelled[pID] {
		t.Errorf("expected EventBridge schedule to be cancelled on hold")
	}

	// Release hold should trigger EventBridge schedule
	_, err = payoutSvc.ReleasePayoutHold(ctx, adminID, pID, "Release check")
	if err != nil {
		t.Fatalf("unexpected release hold error: %v", err)
	}
	if _, ok := stubSched.scheduled[pID]; !ok {
		t.Errorf("expected EventBridge schedule to be created on release hold")
	}
}

func TestPayoutService_CohostSplits(t *testing.T) {
	cfg := &config.Config{
		PayoutReleaseAfterCheckInHours: 24,
	}
	payRepo := newMemPaymentRepo()
	resRepo := newMemReservationRepo()
	hostRepo := &memListingHostRepo{members: make(map[string]models.ListingHost)}
	cohostRepo := newMemCohostRepo()
	now := time.Date(2026, 12, 1, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMockClock(now)

	payoutSvc := NewPayoutService(cfg, &db.DB{}, nil, payRepo, resRepo, nil, nil, clk)
	if pSvc, ok := payoutSvc.(*PayoutService); ok {
		pSvc.SetCohostDependencies(hostRepo, cohostRepo)
	}
	ctx := context.Background()

	listingID := uuid.New()
	ownerID := uuid.New()
	cohost1ID := uuid.New()
	cohost2ID := uuid.New()

	hostRepo.members[listingID.String()+":"+ownerID.String()] = models.ListingHost{
		ListingID:      listingID,
		UserID:         ownerID,
		Role:           enums.ListingHostRoleOwner,
		PayoutShareBPS: 0,
	}
	hostRepo.members[listingID.String()+":"+cohost1ID.String()] = models.ListingHost{
		ListingID:      listingID,
		UserID:         cohost1ID,
		Role:           enums.ListingHostRoleCohost,
		PayoutShareBPS: 2000, // 20%
	}
	hostRepo.members[listingID.String()+":"+cohost2ID.String()] = models.ListingHost{
		ListingID:      listingID,
		UserID:         cohost2ID,
		Role:           enums.ListingHostRoleCohost,
		PayoutShareBPS: 1000, // 10%
	}

	resID := uuid.New()
	checkIn := now.Add(24 * time.Hour)
	checkOut := now.Add(48 * time.Hour)
	res := models.Reservation{
		ID:              resID,
		ListingID:       listingID,
		HostID:          ownerID,
		GuestID:         uuid.New(),
		CheckIn:         checkIn,
		CheckOut:        checkOut,
		CheckInAt:       checkIn,
		CheckOutAt:      checkOut,
		GuestTotalMinor: 114000,
		HostFeeMinor:    3000,
		HostNetMinor:    100000, // 1000.00 EUR
		Currency:        "EUR",
		Status:          enums.ReservationStatusConfirmed,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	resRepo.rows[resID] = res

	// Schedule payout
	po, err := payoutSvc.SchedulePayout(ctx, res, "idem-test-cohost-splits")
	if err != nil {
		t.Fatalf("failed to schedule payout: %v", err)
	}

	// Verify splits
	splits, err := cohostRepo.ListPayoutSplits(ctx, po.ID)
	if err != nil {
		t.Fatalf("failed to list payout splits: %v", err)
	}
	if len(splits) != 3 {
		t.Fatalf("expected 3 splits (2 cohosts + 1 owner), got %d", len(splits))
	}

	var sumAmount int64
	for _, s := range splits {
		sumAmount += s.AmountMinor
		if s.Status != enums.PayoutSplitScheduled {
			t.Errorf("expected split status scheduled, got %s", s.Status)
		}
		if s.UserID == cohost1ID && s.AmountMinor != 20000 {
			t.Errorf("expected cohost 1 amount 20000, got %d", s.AmountMinor)
		}
		if s.UserID == cohost2ID && s.AmountMinor != 10000 {
			t.Errorf("expected cohost 2 amount 10000, got %d", s.AmountMinor)
		}
		if s.UserID == ownerID && s.AmountMinor != 70000 {
			t.Errorf("expected owner amount 70000, got %d", s.AmountMinor)
		}
	}
	if sumAmount != 100000 {
		t.Errorf("expected total split amount 100000, got %d", sumAmount)
	}

	// Release payout
	if err := payoutSvc.ProcessPayoutRelease(ctx, po.ID); err != nil {
		t.Fatalf("failed to process payout release: %v", err)
	}

	splitsReleased, _ := cohostRepo.ListPayoutSplits(ctx, po.ID)
	for _, s := range splitsReleased {
		if s.Status != enums.PayoutSplitReleased {
			t.Errorf("expected split status released, got %s", s.Status)
		}
	}
}
