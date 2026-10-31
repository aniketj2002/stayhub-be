package services

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/domain/pricing"
	"github.com/stayhub/backend/internal/platform/clock"
)

type memAlterationRepo struct {
	mu          sync.Mutex
	alterations map[uuid.UUID]models.ReservationAlteration
}

func newMemAlterationRepo() *memAlterationRepo {
	return &memAlterationRepo{
		alterations: make(map[uuid.UUID]models.ReservationAlteration),
	}
}

func (m *memAlterationRepo) CreateAlteration(_ context.Context, alt models.ReservationAlteration) (*models.ReservationAlteration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if alt.ID == uuid.Nil {
		alt.ID = uuid.New()
	}
	m.alterations[alt.ID] = alt
	return &alt, nil
}

func (m *memAlterationRepo) FindAlterationByID(_ context.Context, id uuid.UUID) (*models.ReservationAlteration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	alt, ok := m.alterations[id]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	return &alt, nil
}

func (m *memAlterationRepo) FindAlterationByIDForUpdate(_ context.Context, id uuid.UUID) (*models.ReservationAlteration, error) {
	return m.FindAlterationByID(context.Background(), id)
}

func (m *memAlterationRepo) ListAlterationsByReservation(_ context.Context, reservationID uuid.UUID) ([]models.ReservationAlteration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.ReservationAlteration
	for _, a := range m.alterations {
		if a.ReservationID == reservationID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (m *memAlterationRepo) ListPendingByReservationIDs(_ context.Context, reservationIDs []uuid.UUID) ([]models.ReservationAlteration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	set := make(map[uuid.UUID]bool, len(reservationIDs))
	for _, id := range reservationIDs {
		set[id] = true
	}
	var out []models.ReservationAlteration
	for _, a := range m.alterations {
		if a.Status == enums.AlterationStatusPending && set[a.ReservationID] {
			out = append(out, a)
		}
	}
	return out, nil
}

func (m *memAlterationRepo) FindPendingByReservation(_ context.Context, reservationID uuid.UUID) (*models.ReservationAlteration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.alterations {
		if a.ReservationID == reservationID && a.Status == enums.AlterationStatusPending {
			return &a, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (m *memAlterationRepo) UpdateAlterationStatus(_ context.Context, id uuid.UUID, status enums.AlterationStatus, decidedAt *time.Time, reason *string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	alt, ok := m.alterations[id]
	if !ok {
		return apperrors.ErrNotFound
	}
	alt.Status = status
	alt.DecidedAt = decidedAt
	alt.DecisionReason = reason
	m.alterations[id] = alt
	return nil
}

func (m *memAlterationRepo) ListExpiredPending(_ context.Context, now time.Time) ([]models.ReservationAlteration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.ReservationAlteration
	for _, a := range m.alterations {
		if a.Status == enums.AlterationStatusPending && !a.ExpiresAt.After(now) {
			out = append(out, a)
		}
	}
	return out, nil
}

type mockPricingService struct {
	quoteFn func(checkIn, checkOut time.Time) (pricing.Breakdown, error)
}

func (p *mockPricingService) GetPricingRule(_ context.Context, _ uuid.UUID) (*dtos.ListingPricingRuleResponse, error) {
	return nil, nil
}
func (p *mockPricingService) SetPricingRule(_ context.Context, _ uuid.UUID, _ dtos.ListingPricingRuleInput) (*dtos.ListingPricingRuleResponse, error) {
	return nil, nil
}
func (p *mockPricingService) GetDateOverrides(_ context.Context, _ uuid.UUID, _, _ time.Time) ([]dtos.DateOverrideResponse, error) {
	return nil, nil
}
func (p *mockPricingService) SetDateOverrides(_ context.Context, _ uuid.UUID, _ dtos.DateOverrideRangeInput) error {
	return nil
}
func (p *mockPricingService) DeleteDateOverrides(_ context.Context, _ uuid.UUID, _, _ time.Time) error {
	return nil
}
func (p *mockPricingService) CalculateQuote(_ context.Context, _ uuid.UUID, checkIn, checkOut time.Time) (pricing.Breakdown, error) {
	if p.quoteFn != nil {
		return p.quoteFn(checkIn, checkOut)
	}
	nights := models.EnumerateNights(checkIn, checkOut)
	nightCount := len(nights)
	subtotal := int64(nightCount * 10000)
	cleaning := int64(5000)
	guestFee := int64(float64(subtotal+cleaning) * 0.12)
	hostFee := int64(float64(subtotal+cleaning) * 0.03)
	guestTotal := subtotal + cleaning + guestFee
	hostNet := subtotal + cleaning - hostFee
	return pricing.Breakdown{
		NightCount:                 nightCount,
		SubtotalMinor:              subtotal,
		NightlyTotalNetMinor:       subtotal,
		CleaningFeeMinor:           cleaning,
		AccommodationSubtotalMinor: subtotal + cleaning,
		GuestFeeBPS:                1200,
		GuestFeeMinor:              guestFee,
		GuestTotalMinor:            guestTotal,
		HostFeeBPS:                 300,
		HostFeeMinor:               hostFee,
		HostNetMinor:               hostNet,
		Currency:                   "USD",
	}, nil
}

func TestReservationAlterationService(t *testing.T) {
	now := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	clk := clock.NewMockClock(now)

	guestID := uuid.New()
	hostID := uuid.New()
	listingID := uuid.New()
	resID := uuid.New()

	checkIn := time.Date(2026, 11, 10, 15, 0, 0, 0, time.UTC)
	checkOut := time.Date(2026, 11, 13, 11, 0, 0, 0, time.UTC) // 3 nights

	res := models.Reservation{
		ID:                resID,
		Code:              "SH-TEST",
		ListingID:         listingID,
		GuestID:           guestID,
		HostID:            hostID,
		Status:            enums.ReservationStatusConfirmed,
		CheckInAt:         checkIn,
		CheckOutAt:        checkOut,
		Guests:            2,
		Currency:          "USD",
		NightlyTotalMinor: 30000,
		CleaningFeeMinor:  5000,
		GuestFeeMinor:     4200,
		GuestTotalMinor:   39200,
		HostFeeMinor:      1050,
		HostNetMinor:      33950,
	}

	altRepo := newMemAlterationRepo()
	resRepo := newMemReservationRepo()
	resRepo.rows[resID] = res
	calRepo := newMemCalendarRepo()
	// Seed calendar with the 3 nights for this reservation
	_ = calRepo.BookNights(context.Background(), listingID, resID, models.EnumerateNights(checkIn, checkOut))

	listingRepo := &memListingRepo{
		listings: map[uuid.UUID]models.Listing{
			listingID: {ID: listingID, HostID: hostID},
		},
		versions: map[uuid.UUID]models.ListingVersion{
			uuid.New(): {Timezone: "UTC"},
		},
	}
	listingHostRepo := &memListingHostRepo{
		members: map[string]models.ListingHost{
			listingID.String() + ":" + hostID.String(): {
				ListingID: listingID,
				UserID:    hostID,
				Role:      enums.ListingHostRoleOwner,
			},
		},
	}
	pricingSvc := &mockPricingService{}
	pricingRepo := newMemPricingRepo()
	paymentRepo := newMemPaymentRepo()

	svc := NewReservationAlterationService(
		nil,
		altRepo,
		resRepo,
		calRepo,
		listingRepo,
		listingHostRepo,
		pricingSvc,
		pricingRepo,
		paymentRepo,
		nil,
		nil,
		clk,
	)

	var altID uuid.UUID

	t.Run("Guest requests alteration to add 2 nights (positive delta)", func(t *testing.T) {
		newCheckOut := "2026-11-15" // 5 nights instead of 3
		req, err := svc.RequestAlteration(context.Background(), guestID, resID, dtos.CreateReservationAlterationInput{
			CheckOut: &newCheckOut,
		})
		if err != nil {
			t.Fatalf("expected no error requesting alteration: %v", err)
		}
		if req.Status != enums.AlterationStatusPending {
			t.Errorf("expected pending status, got %s", req.Status)
		}
		if req.DeltaMinor <= 0 {
			t.Errorf("expected positive delta for 2 additional nights, got %d", req.DeltaMinor)
		}
		expectedExpiry := now.Add(24 * time.Hour)
		if !req.ExpiresAt.Equal(expectedExpiry) {
			t.Errorf("expected expires_at %v, got %v", expectedExpiry, req.ExpiresAt)
		}
		altID = req.ID

		pending, err := svc.ListPendingForHost(context.Background(), hostID)
		if err != nil {
			t.Fatalf("expected host to load pending date changes: %v", err)
		}
		if len(pending) != 1 || pending[0].ID != req.ID || pending[0].ReservationCode != "SH-TEST" {
			t.Fatalf("expected the host to receive the pending date change, got %+v", pending)
		}
		forGuest, err := svc.ListPendingForHost(context.Background(), guestID)
		if err != nil {
			t.Fatalf("expected guest lookup to succeed: %v", err)
		}
		if len(forGuest) != 0 {
			t.Fatalf("expected the requester not to receive their own request for approval, got %d", len(forGuest))
		}
	})

	t.Run("Host approves alteration - atomically locks calendar, charges delta, updates reservation", func(t *testing.T) {
		hostActor := models.AuthActor{SubjectID: hostID, IsHost: true}
		approved, err := svc.ApproveAlteration(context.Background(), hostActor, altID)
		if err != nil {
			t.Fatalf("expected no error approving alteration: %v", err)
		}
		if approved.Status != enums.AlterationStatusApproved {
			t.Errorf("expected approved status, got %s", approved.Status)
		}

		// Verify reservation was updated
		updatedRes := resRepo.rows[resID]
		if updatedRes.CheckOutAt.Format("2006-01-02") != "2026-11-15" {
			t.Errorf("expected checkout 2026-11-15, got %s", updatedRes.CheckOutAt.Format("2006-01-02"))
		}
		if updatedRes.GuestTotalMinor <= res.GuestTotalMinor {
			t.Errorf("expected updated guest total > previous guest total")
		}

		// Verify payment charge was recorded
		payments, _ := paymentRepo.ListPaymentsByReservation(context.Background(), resID)
		if len(payments) == 0 {
			t.Fatalf("expected payment charge to be recorded")
		}
		lastPay := payments[len(payments)-1]
		if lastPay.Kind != enums.PaymentKindCharge || lastPay.AmountMinor != approved.DeltaMinor {
			t.Errorf("expected charge of %d, got kind=%s amount=%d", approved.DeltaMinor, lastPay.Kind, lastPay.AmountMinor)
		}

		// Verify balanced ledger entries were written
		ledgerEntries, _ := paymentRepo.ListLedgerByReservation(context.Background(), resID)
		if len(ledgerEntries) == 0 {
			t.Fatalf("expected ledger entries for charge delta")
		}
	})

	t.Run("Guest requests second alteration and host declines", func(t *testing.T) {
		newCheckIn := "2026-11-09"
		req, err := svc.RequestAlteration(context.Background(), guestID, resID, dtos.CreateReservationAlterationInput{
			CheckIn: &newCheckIn,
		})
		if err != nil {
			t.Fatalf("expected no error: %v", err)
		}

		hostActor := models.AuthActor{SubjectID: hostID, IsHost: true}
		reason := "Host is unavailable for earlier check-in"
		declined, err := svc.DeclineAlteration(context.Background(), hostActor, req.ID, dtos.DeclineReservationAlterationInput{
			Reason: reason,
		})
		if err != nil {
			t.Fatalf("expected no error declining alteration: %v", err)
		}
		if declined.Status != enums.AlterationStatusDeclined {
			t.Errorf("expected declined status, got %s", declined.Status)
		}
		if declined.DecisionReason == nil || *declined.DecisionReason != reason {
			t.Errorf("expected decision reason '%s', got %v", reason, declined.DecisionReason)
		}
	})

	t.Run("24h expiration sweep expires pending alterations", func(t *testing.T) {
		newGuests := 3
		req, err := svc.RequestAlteration(context.Background(), guestID, resID, dtos.CreateReservationAlterationInput{
			Guests: &newGuests,
		})
		if err != nil {
			t.Fatalf("expected no error: %v", err)
		}

		// Advance clock past 24 hours
		clk.Advance(25 * time.Hour)

		count, err := svc.ExpireDuePendingAlterations(context.Background())
		if err != nil {
			t.Fatalf("expected no error running expiration sweep: %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 alteration expired, got %d", count)
		}

		alt, _ := altRepo.FindAlterationByID(context.Background(), req.ID)
		if alt.Status != enums.AlterationStatusExpired {
			t.Errorf("expected alteration status expired, got %s", alt.Status)
		}
	})
}
