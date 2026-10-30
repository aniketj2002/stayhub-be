package services

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/config"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/db"
)

func assertBalancedLedger(t *testing.T, label string, entries []models.LedgerEntry) {
	t.Helper()
	var debit, credit int64
	for _, e := range entries {
		switch e.Direction {
		case enums.LedgerDirectionDebit:
			debit += e.AmountMinor
		case enums.LedgerDirectionCredit:
			credit += e.AmountMinor
		default:
			t.Fatalf("[%s] unexpected ledger direction: %s", label, e.Direction)
		}
	}
	if debit != credit {
		t.Fatalf("[%s] unbalanced ledger entries: debit=%d credit=%d", label, debit, credit)
	}
}

func TestLedgerBuilders_AlwaysBalance(t *testing.T) {
	resID := uuid.New()
	payID := uuid.New()
	payoutID := uuid.New()

	// Odd amount quote to stress rounding: 3 nights @ 11111 + 2222 cleaning = 35555 subtotal
	q := models.ComputeStayQuote(11111, 2222, 3, 1400, 300)

	// 1. Charge Ledger
	chargeEntries := BuildChargeLedgerEntries(
		uuid.New(),
		resID,
		payID,
		"EUR",
		q.GuestTotalMinor,
		q.GuestFeeMinor,
		q.HostFeeMinor,
		q.HostNetMinor,
	)
	if len(chargeEntries) != 4 {
		t.Fatalf("expected 4 charge ledger legs, got %d", len(chargeEntries))
	}
	assertBalancedLedger(t, "charge", chargeEntries)

	res := models.Reservation{
		ID:                 resID,
		NightlyTotalMinor:  q.NightlyTotalMinor,
		CleaningFeeMinor:   2222,
		GuestFeeMinor:      q.GuestFeeMinor,
		HostFeeMinor:       q.HostFeeMinor,
		GuestTotalMinor:    q.GuestTotalMinor,
		HostNetMinor:       q.HostNetMinor,
		GuestFeeBPS:        1400,
		HostFeeBPS:         300,
		Currency:           "EUR",
		CheckInAt:          time.Date(2026, 12, 20, 15, 0, 0, 0, time.UTC),
		CancellationPolicy: "firm",
	}

	firmRules := []models.CancellationPolicyRule{
		{PolicyCode: "firm", MinHoursBefore: 720, RefundPct: 100, RefundsServiceFee: true},
		{PolicyCode: "firm", MinHoursBefore: 168, RefundPct: 50, RefundsServiceFee: false},
		{PolicyCode: "firm", MinHoursBefore: 0, RefundPct: 0, RefundsServiceFee: false},
	}

	// 2. Full Refund Ledger (35 days before check-in)
	rbFull := models.ComputePolicyRefund(res, firmRules, res.CheckInAt.Add(-35*24*time.Hour))
	fullRefundEntries := BuildRefundLedgerEntries(uuid.New(), resID, uuid.New(), "EUR", rbFull)
	if len(fullRefundEntries) != 4 {
		t.Fatalf("expected 4 full-refund ledger legs, got %d", len(fullRefundEntries))
	}
	assertBalancedLedger(t, "full_refund", fullRefundEntries)

	// 3. Partial 50% Refund Ledger (10 days before check-in -> no guest fee refund)
	rbPartial := models.ComputePolicyRefund(res, firmRules, res.CheckInAt.Add(-10*24*time.Hour))
	partialRefundEntries := BuildRefundLedgerEntries(uuid.New(), resID, uuid.New(), "EUR", rbPartial)
	if len(partialRefundEntries) != 3 {
		t.Fatalf("expected 3 partial-refund ledger legs (no guest fee leg), got %d", len(partialRefundEntries))
	}
	assertBalancedLedger(t, "partial_refund", partialRefundEntries)

	// 4. Payout Release Ledger for retained host net
	payoutEntries := BuildPayoutReleaseLedgerEntries(uuid.New(), resID, payoutID, "EUR", rbPartial.RetainedHostNetMinor)
	if len(payoutEntries) != 2 {
		t.Fatalf("expected 2 payout release ledger legs, got %d", len(payoutEntries))
	}
	assertBalancedLedger(t, "payout_release", payoutEntries)
}

// --- In-Memory Repositories for End-to-End Reservation Lifecycle, Concurrency & Event Log Tests ---

type memReservationRepo struct {
	mu   sync.Mutex
	rows map[uuid.UUID]models.Reservation
}

func newMemReservationRepo() *memReservationRepo {
	return &memReservationRepo{rows: make(map[uuid.UUID]models.Reservation)}
}

func (r *memReservationRepo) Create(_ context.Context, res models.Reservation) (*models.Reservation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if res.ID == uuid.Nil {
		res.ID = uuid.New()
	}
	res.Version = 1
	res.CreatedAt = time.Now().UTC()
	res.UpdatedAt = res.CreatedAt
	r.rows[res.ID] = res
	cp := res
	return &cp, nil
}

func (r *memReservationRepo) FindByID(_ context.Context, id uuid.UUID) (*models.Reservation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	res, ok := r.rows[id]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	cp := res
	return &cp, nil
}

func (r *memReservationRepo) FindByIDForUpdate(ctx context.Context, id uuid.UUID) (*models.Reservation, error) {
	return r.FindByID(ctx, id)
}

func (r *memReservationRepo) Update(_ context.Context, res models.Reservation) (*models.Reservation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.rows[res.ID]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	if cur.Version != res.Version {
		return nil, apperrors.ErrConflict
	}
	res.Version++
	r.rows[res.ID] = res
	cp := res
	return &cp, nil
}

func (r *memReservationRepo) ListByGuest(_ context.Context, guestID uuid.UUID) ([]models.Reservation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.Reservation
	for _, v := range r.rows {
		if v.GuestID == guestID {
			out = append(out, v)
		}
	}
	return out, nil
}

func (r *memReservationRepo) ListByListingIDs(_ context.Context, listingIDs []uuid.UUID) ([]models.Reservation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	set := make(map[uuid.UUID]bool, len(listingIDs))
	for _, id := range listingIDs {
		set[id] = true
	}
	var out []models.Reservation
	for _, v := range r.rows {
		if set[v.ListingID] {
			out = append(out, v)
		}
	}
	return out, nil
}

func (r *memReservationRepo) ListDuePendingExpirations(_ context.Context, now time.Time, _ int) ([]models.Reservation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.Reservation
	for _, v := range r.rows {
		if v.Status == enums.ReservationStatusPending && v.RequestExpiresAt != nil && !v.RequestExpiresAt.After(now) {
			out = append(out, v)
		}
	}
	return out, nil
}

func (r *memReservationRepo) ListDueCheckIns(_ context.Context, now time.Time, _ int) ([]models.Reservation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.Reservation
	for _, v := range r.rows {
		if v.Status == enums.ReservationStatusConfirmed && !v.CheckInAt.After(now) {
			out = append(out, v)
		}
	}
	return out, nil
}

func (r *memReservationRepo) ListDueCheckOuts(_ context.Context, now time.Time, _ int) ([]models.Reservation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.Reservation
	for _, v := range r.rows {
		if v.Status == enums.ReservationStatusCheckedIn && !v.CheckOutAt.After(now) {
			out = append(out, v)
		}
	}
	return out, nil
}

type memCalendarRepo struct {
	mu     sync.Mutex
	nights map[string]models.ListingCalendar
}

func newMemCalendarRepo() *memCalendarRepo {
	return &memCalendarRepo{nights: make(map[string]models.ListingCalendar)}
}

func calKey(listingID uuid.UUID, night time.Time) string {
	return listingID.String() + ":" + night.Format("2006-01-02")
}

func (r *memCalendarRepo) ListRange(_ context.Context, listingID uuid.UUID, from, to time.Time) ([]models.ListingCalendar, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.ListingCalendar
	for _, n := range r.nights {
		if n.ListingID == listingID && !n.Night.Before(from) && !n.Night.After(to) {
			out = append(out, n)
		}
	}
	return out, nil
}

func (r *memCalendarRepo) BlockNights(_ context.Context, listingID uuid.UUID, nights []time.Time, reason *string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, n := range nights {
		if _, exists := r.nights[calKey(listingID, n)]; exists {
			return apperrors.ErrDatesUnavailable
		}
	}
	for _, n := range nights {
		r.nights[calKey(listingID, n)] = models.ListingCalendar{
			ListingID:   listingID,
			Night:       n,
			Status:      enums.CalendarStatusBlocked,
			BlockReason: reason,
		}
	}
	return nil
}

func (r *memCalendarRepo) UnblockNights(_ context.Context, listingID uuid.UUID, startDate, endDate time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, n := range models.EnumerateNights(startDate, endDate) {
		k := calKey(listingID, n)
		if cur, ok := r.nights[k]; ok && cur.Status == enums.CalendarStatusBlocked {
			delete(r.nights, k)
		}
	}
	return nil
}

func (r *memCalendarRepo) BookNights(_ context.Context, listingID, reservationID uuid.UUID, nights []time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, n := range nights {
		if _, exists := r.nights[calKey(listingID, n)]; exists {
			return apperrors.ErrDatesUnavailable
		}
	}
	for _, n := range nights {
		resID := reservationID
		r.nights[calKey(listingID, n)] = models.ListingCalendar{
			ListingID:     listingID,
			Night:         n,
			Status:        enums.CalendarStatusBooked,
			ReservationID: &resID,
		}
	}
	return nil
}

func (r *memCalendarRepo) ReleaseReservationNights(_ context.Context, reservationID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, v := range r.nights {
		if v.ReservationID != nil && *v.ReservationID == reservationID {
			delete(r.nights, k)
		}
	}
	return nil
}

type memPaymentRepo struct {
	mu       sync.Mutex
	payments []models.Payment
	payouts  map[uuid.UUID]models.Payout
	ledger   []models.LedgerEntry
}

func newMemPaymentRepo() *memPaymentRepo {
	return &memPaymentRepo{payouts: make(map[uuid.UUID]models.Payout)}
}

func (r *memPaymentRepo) CreatePayment(_ context.Context, p models.Payment) (*models.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p.ID = uuid.New()
	p.CreatedAt = time.Now().UTC()
	p.UpdatedAt = p.CreatedAt
	r.payments = append(r.payments, p)
	cp := p
	return &cp, nil
}

func (r *memPaymentRepo) FindChargeByReservation(_ context.Context, reservationID uuid.UUID) (*models.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.payments {
		if p.ReservationID == reservationID && p.Kind == enums.PaymentKindCharge {
			cp := p
			return &cp, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (r *memPaymentRepo) ListPaymentsByReservation(_ context.Context, reservationID uuid.UUID) ([]models.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.Payment
	for _, p := range r.payments {
		if p.ReservationID == reservationID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (r *memPaymentRepo) CreatePayout(_ context.Context, p models.Payout) (*models.Payout, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p.ID = uuid.New()
	p.CreatedAt = time.Now().UTC()
	p.UpdatedAt = p.CreatedAt
	r.payouts[p.ID] = p
	cp := p
	return &cp, nil
}

func (r *memPaymentRepo) FindPayoutByReservation(_ context.Context, reservationID uuid.UUID) (*models.Payout, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.payouts {
		if p.ReservationID == reservationID {
			cp := p
			return &cp, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (r *memPaymentRepo) FindPayoutByIDForUpdate(_ context.Context, id uuid.UUID) (*models.Payout, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.payouts[id]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	cp := p
	return &cp, nil
}

func (r *memPaymentRepo) UpdatePayout(_ context.Context, p models.Payout) (*models.Payout, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.payouts[p.ID] = p
	cp := p
	return &cp, nil
}

func (r *memPaymentRepo) ListPayoutsByListingIDs(_ context.Context, _ []uuid.UUID) ([]dtos.EarningsPayoutItem, error) {
	return nil, nil
}

func (r *memPaymentRepo) ListDuePayouts(_ context.Context, now time.Time, _ int) ([]models.Payout, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.Payout
	for _, p := range r.payouts {
		if p.Status == enums.PayoutStatusScheduled && p.ReleaseAt != nil && !p.ReleaseAt.After(now) {
			out = append(out, p)
		}
	}
	return out, nil
}

func (r *memPaymentRepo) InsertLedgerEntries(_ context.Context, entries []models.LedgerEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var debit, credit int64
	for _, e := range entries {
		if e.Direction == enums.LedgerDirectionDebit {
			debit += e.AmountMinor
		} else {
			credit += e.AmountMinor
		}
	}
	if debit != credit {
		return apperrors.ErrLedgerUnbalanced
	}
	r.ledger = append(r.ledger, entries...)
	return nil
}

func (r *memPaymentRepo) ListLedgerByReservation(_ context.Context, reservationID uuid.UUID) ([]models.LedgerEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.LedgerEntry
	for _, e := range r.ledger {
		if e.ReservationID != nil && *e.ReservationID == reservationID {
			out = append(out, e)
		}
	}
	return out, nil
}

type memListingRepo struct {
	listings map[uuid.UUID]models.Listing
	versions map[uuid.UUID]models.ListingVersion
}

func (r *memListingRepo) CreateListing(_ context.Context, _ uuid.UUID) (*models.Listing, error) {
	return nil, nil
}
func (r *memListingRepo) FindByID(_ context.Context, id uuid.UUID) (*models.Listing, error) {
	l, ok := r.listings[id]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	return &l, nil
}
func (r *memListingRepo) UpdatePointersAndStatus(_ context.Context, _ uuid.UUID, _ enums.ListingStatus, _, _ *uuid.UUID, _ time.Time) (*models.Listing, error) {
	return nil, nil
}
func (r *memListingRepo) CreateVersion(_ context.Context, _ models.ListingVersion) (*models.ListingVersion, error) {
	return nil, nil
}
func (r *memListingRepo) FindVersionByID(_ context.Context, id uuid.UUID) (*models.ListingVersion, error) {
	v, ok := r.versions[id]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	return &v, nil
}
func (r *memListingRepo) UpdateVersion(_ context.Context, _ models.ListingVersion) (*models.ListingVersion, error) {
	return nil, nil
}
func (r *memListingRepo) ReplaceCheckResults(_ context.Context, _ uuid.UUID, _ []models.ListingCheckResult) ([]models.ListingCheckResult, error) {
	return nil, nil
}
func (r *memListingRepo) ListCheckResultsByVersion(_ context.Context, _ uuid.UUID) ([]models.ListingCheckResult, error) {
	return nil, nil
}
func (r *memListingRepo) SearchLiveListings(_ context.Context, _ dtos.SearchListingsFilter) ([]dtos.SearchListingRow, error) {
	return nil, nil
}

type memListingHostRepo struct {
	members map[string]models.ListingHost
}

func (r *memListingHostRepo) CreateOwner(_ context.Context, _, _ uuid.UUID, _ []byte) (*models.ListingHost, error) {
	return nil, nil
}
func (r *memListingHostRepo) FindByListingAndUser(_ context.Context, listingID, userID uuid.UUID) (*models.ListingHost, error) {
	lh, ok := r.members[listingID.String()+":"+userID.String()]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	return &lh, nil
}
func (r *memListingHostRepo) ListByListing(_ context.Context, listingID uuid.UUID) ([]models.ListingHost, error) {
	var out []models.ListingHost
	for _, m := range r.members {
		if m.ListingID == listingID {
			out = append(out, m)
		}
	}
	return out, nil
}
func (r *memListingHostRepo) ListByUser(_ context.Context, userID uuid.UUID) ([]models.ListingHost, error) {
	var out []models.ListingHost
	for _, m := range r.members {
		if m.UserID == userID {
			out = append(out, m)
		}
	}
	return out, nil
}
func (r *memListingHostRepo) UpdateConfig(_ context.Context, _, _ uuid.UUID, _ []byte) (*models.ListingHost, error) {
	return nil, nil
}

type memUserRepo struct {
	users map[uuid.UUID]models.User
}

func (r *memUserRepo) Create(_ context.Context, _ dtos.CreateUserParams) (*models.User, error) {
	return nil, nil
}
func (r *memUserRepo) FindByID(_ context.Context, id uuid.UUID) (*models.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	return &u, nil
}
func (r *memUserRepo) FindByEmail(_ context.Context, _ string) (*models.User, error) {
	return nil, apperrors.ErrNotFound
}
func (r *memUserRepo) EnableHostRole(_ context.Context, _ uuid.UUID, _ time.Time) (*models.User, error) {
	return nil, nil
}
func (r *memUserRepo) UpdateProfile(_ context.Context, _ uuid.UUID, _ string, _ *uuid.UUID, _ time.Time) (*models.User, error) {
	return nil, nil
}

type memCatalogService struct{}

func (s *memCatalogService) ListAmenities(_ context.Context) ([]models.Amenity, error) {
	return nil, nil
}
func (s *memCatalogService) AmenityCodeSet(_ context.Context) (map[string]struct{}, error) {
	return nil, nil
}
func (s *memCatalogService) ListCancellationPolicies(_ context.Context) ([]models.CancellationPolicy, error) {
	return nil, nil
}
func (s *memCatalogService) GetCancellationPolicy(_ context.Context, code string) (*models.CancellationPolicy, error) {
	return &models.CancellationPolicy{
		Code: code,
		Name: code,
		Rules: []models.CancellationPolicyRule{
			{PolicyCode: code, MinHoursBefore: 120, RefundPct: 100, RefundsServiceFee: true},
			{PolicyCode: code, MinHoursBefore: 24, RefundPct: 50, RefundsServiceFee: false},
			{PolicyCode: code, MinHoursBefore: 0, RefundPct: 0, RefundsServiceFee: false},
		},
	}, nil
}

type memAuditService struct {
	mu     sync.Mutex
	events []dtos.AuditEntryInput
}

func (a *memAuditService) Record(_ context.Context, in dtos.AuditEntryInput) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, in)
	return nil
}

func (a *memAuditService) ListEvents(_ context.Context, _ dtos.EventListFilter) ([]models.EventLog, error) {
	return nil, nil
}

func TestReservationLifecycle_DoubleBooking_StateTransitions_PayoutTiming_EventLog(t *testing.T) {
	now := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	mockClk := clock.NewMockClock(now)

	hostID := uuid.New()
	guest1ID := uuid.New()
	guest2ID := uuid.New()
	listingInstantID := uuid.New()
	versionInstantID := uuid.New()
	listingRequestID := uuid.New()
	versionRequestID := uuid.New()

	strPtr := func(s string) *string { return &s }
	intPtr := func(i int) *int { return &i }
	int64Ptr := func(i int64) *int64 { return &i }

	listingRepo := &memListingRepo{
		listings: map[uuid.UUID]models.Listing{
			listingInstantID: {ID: listingInstantID, HostID: hostID, Status: enums.ListingStatusLive, LiveVersionID: &versionInstantID},
			listingRequestID: {ID: listingRequestID, HostID: hostID, Status: enums.ListingStatusLive, LiveVersionID: &versionRequestID},
		},
		versions: map[uuid.UUID]models.ListingVersion{
			versionInstantID: {
				ID:                 versionInstantID,
				ListingID:          listingInstantID,
				Status:             enums.VersionStatusApproved,
				Title:              strPtr("Lisbon Loft"),
				City:               strPtr("Lisbon"),
				Country:            strPtr("PT"),
				Timezone:           "Europe/Lisbon",
				MaxGuests:          intPtr(4),
				BasePriceMinor:     int64Ptr(15000),
				CleaningFeeMinor:   3000,
				Currency:           "EUR",
				InstantBook:        true,
				CheckInTime:        "15:00",
				CheckOutTime:       "11:00",
				CancellationPolicy: strPtr("moderate"),
			},
			versionRequestID: {
				ID:                 versionRequestID,
				ListingID:          listingRequestID,
				Status:             enums.VersionStatusApproved,
				Title:              strPtr("Kyoto Machiya"),
				City:               strPtr("Kyoto"),
				Country:            strPtr("JP"),
				Timezone:           "Asia/Tokyo",
				MaxGuests:          intPtr(4),
				BasePriceMinor:     int64Ptr(20000),
				CleaningFeeMinor:   4000,
				Currency:           "EUR",
				InstantBook:        false,
				CheckInTime:        "15:00",
				CheckOutTime:       "10:00",
				CancellationPolicy: strPtr("moderate"),
			},
		},
	}

	lhRepo := &memListingHostRepo{
		members: map[string]models.ListingHost{
			listingInstantID.String() + ":" + hostID.String(): {
				ListingID:             listingInstantID,
				UserID:                hostID,
				Role:                  enums.ListingHostRoleOwner,
				CanManageListing:      true,
				CanManageCalendar:     true,
				CanManageReservations: true,
				CanViewInsights:       true,
			},
			listingRequestID.String() + ":" + hostID.String(): {
				ListingID:             listingRequestID,
				UserID:                hostID,
				Role:                  enums.ListingHostRoleOwner,
				CanManageListing:      true,
				CanManageCalendar:     true,
				CanManageReservations: true,
				CanViewInsights:       true,
			},
		},
	}

	userRepo := &memUserRepo{
		users: map[uuid.UUID]models.User{
			hostID:   {ID: hostID, Email: "host@stayhub.dev", FullName: "Host", IsGuest: true, IsHost: true},
			guest1ID: {ID: guest1ID, Email: "g1@stayhub.dev", FullName: "Guest 1", IsGuest: true},
			guest2ID: {ID: guest2ID, Email: "g2@stayhub.dev", FullName: "Guest 2", IsGuest: true},
		},
	}

	resRepo := newMemReservationRepo()
	payRepo := newMemPaymentRepo()
	calRepo := newMemCalendarRepo()
	auditSvc := &memAuditService{}

	cfg := &config.Config{
		DefaultCurrency:                "EUR",
		GuestFeeBPS:                    1400,
		HostFeeBPS:                     300,
		RequestExpiryHours:             24,
		PayoutReleaseAfterCheckInHours: 24,
	}

	svc := NewReservationService(
		cfg,
		&db.DB{}, // WithinTx runs fn directly when Pool is nil
		nil,
		resRepo,
		payRepo,
		listingRepo,
		lhRepo,
		calRepo,
		userRepo,
		nil,
		&memCatalogService{},
		nil,
		auditSvc,
		mockClk,
	)

	checkIn := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	checkOut := time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC)

	// 1. Self-Booking Guard: Host cannot book their own listing
	_, err := svc.CreateReservation(
		context.Background(),
		models.AuthActor{SubjectID: hostID, SubjectType: "user", IsGuest: true, IsHost: true},
		"idem-self",
		dtos.CreateReservationInput{
			ListingID: listingInstantID,
			CheckIn:   checkIn,
			CheckOut:  checkOut,
			Guests:    2,
		},
	)
	if !errors.Is(err, apperrors.ErrSelfBookingNotAllowed) {
		t.Fatalf("expected ErrSelfBookingNotAllowed, got %v", err)
	}

	// 1b. Past-Date Booking Guard: Cannot book dates in the past
	pastCheckIn := now.AddDate(0, 0, -2)
	pastCheckOut := now.AddDate(0, 0, 1)
	_, err = svc.CreateReservation(
		context.Background(),
		models.AuthActor{SubjectID: guest1ID, SubjectType: "user", IsGuest: true},
		"idem-past",
		dtos.CreateReservationInput{
			ListingID: listingInstantID,
			CheckIn:   pastCheckIn,
			CheckOut:  pastCheckOut,
			Guests:    2,
		},
	)
	if !errors.Is(err, apperrors.ErrPastDate) {
		t.Fatalf("expected ErrPastDate for past check-in, got %v", err)
	}

	// 2. Concurrent Double-Booking Rejection on overlapping dates
	var wg sync.WaitGroup
	errs := make([]error, 2)
	results := make([]*dtos.ReservationDetailResponse, 2)
	guests := []uuid.UUID{guest1ID, guest2ID}

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = svc.CreateReservation(
				context.Background(),
				models.AuthActor{SubjectID: guests[idx], SubjectType: "user", IsGuest: true},
				"idem-conc-"+uuid.NewString(),
				dtos.CreateReservationInput{
					ListingID: listingInstantID,
					CheckIn:   checkIn,
					CheckOut:  checkOut,
					Guests:    2,
				},
			)
		}(i)
	}
	wg.Wait()

	successCount := 0
	conflictCount := 0
	var confirmedRes *dtos.ReservationDetailResponse
	for i := 0; i < 2; i++ {
		if errs[i] == nil {
			successCount++
			confirmedRes = results[i]
		} else if errors.Is(errs[i], apperrors.ErrDatesUnavailable) {
			conflictCount++
		} else {
			t.Fatalf("unexpected error in concurrent booking %d: %v", i, errs[i])
		}
	}
	if successCount != 1 || conflictCount != 1 {
		t.Fatalf("expected exactly 1 success and 1 ErrDatesUnavailable conflict, got success=%d conflict=%d", successCount, conflictCount)
	}

	// Verify Instant Book created `confirmed` status + scheduled payout + balanced 4-leg charge ledger
	if confirmedRes.Reservation.Status != enums.ReservationStatusConfirmed {
		t.Fatalf("expected confirmed status for instant book, got %s", confirmedRes.Reservation.Status)
	}

	// 3. Invalid State Transition Guard: Accepting an already confirmed reservation must fail
	_, err = svc.AcceptByHost(context.Background(), hostID, confirmedRes.Reservation.ID, "idem-invalid-accept")
	if !errors.Is(err, apperrors.ErrInvalidStateTransition) {
		t.Fatalf("expected ErrInvalidStateTransition when accepting confirmed reservation, got %v", err)
	}

	// 4. Check-In & Payout Release Timing (checked_in_at + 24h)
	checkInInstant := confirmedRes.Reservation.CheckInAt.Add(10 * time.Minute)
	mockClk.Set(checkInInstant)

	checkedInRes, err := svc.CheckInByHost(context.Background(), hostID, confirmedRes.Reservation.ID, "idem-checkin")
	if err != nil {
		t.Fatalf("unexpected error on CheckInByHost: %v", err)
	}
	if checkedInRes.Reservation.Status != enums.ReservationStatusCheckedIn {
		t.Fatalf("expected checked_in status, got %s", checkedInRes.Reservation.Status)
	}
	if checkedInRes.Payout == nil || checkedInRes.Payout.ReleaseAt == nil {
		t.Fatalf("expected payout.release_at to be set on check-in")
	}
	expectedReleaseAt := checkInInstant.Add(24 * time.Hour)
	if !checkedInRes.Payout.ReleaseAt.Equal(expectedReleaseAt) {
		t.Fatalf("expected release_at %v, got %v", expectedReleaseAt, *checkedInRes.Payout.ReleaseAt)
	}

	// 23 hours after check-in: payout must NOT be released yet
	mockClk.Set(checkInInstant.Add(23 * time.Hour))
	releasedCount, err := svc.ReleaseDuePayouts(context.Background(), 10)
	if err != nil || releasedCount != 0 {
		t.Fatalf("expected 0 payouts released at +23h, got %d (err=%v)", releasedCount, err)
	}

	// 24 hours after check-in: payout MUST be released with balanced 2-leg ledger entry
	mockClk.Set(checkInInstant.Add(24 * time.Hour))
	releasedCount, err = svc.ReleaseDuePayouts(context.Background(), 10)
	if err != nil || releasedCount != 1 {
		t.Fatalf("expected 1 payout released at +24h, got %d (err=%v)", releasedCount, err)
	}

	// 5. Request-to-Book 24h Expiry Sweep -> Auto-Declines, Refunds 100%, and Frees Calendar Nights
	reqCheckIn := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	reqCheckOut := time.Date(2026, 12, 4, 0, 0, 0, 0, time.UTC)
	pendingRes, err := svc.CreateReservation(
		context.Background(),
		models.AuthActor{SubjectID: guest1ID, SubjectType: "user", IsGuest: true},
		"idem-req-1",
		dtos.CreateReservationInput{
			ListingID: listingRequestID,
			CheckIn:   reqCheckIn,
			CheckOut:  reqCheckOut,
			Guests:    2,
		},
	)
	if err != nil {
		t.Fatalf("unexpected error creating request reservation: %v", err)
	}
	if pendingRes.Reservation.Status != enums.ReservationStatusPending {
		t.Fatalf("expected pending status for request-to-book, got %s", pendingRes.Reservation.Status)
	}

	// Advance clock by 25 hours past request creation -> expire pending request
	mockClk.Advance(25 * time.Hour)
	expiredCount, err := svc.ExpireDuePendingReservations(context.Background(), 10)
	if err != nil || expiredCount != 1 {
		t.Fatalf("expected 1 expired pending reservation, got %d (err=%v)", expiredCount, err)
	}

	// Verify calendar nights were released so another guest can now book those exact dates
	rebookedRes, err := svc.CreateReservation(
		context.Background(),
		models.AuthActor{SubjectID: guest2ID, SubjectType: "user", IsGuest: true},
		"idem-req-2",
		dtos.CreateReservationInput{
			ListingID: listingRequestID,
			CheckIn:   reqCheckIn,
			CheckOut:  reqCheckOut,
			Guests:    2,
		},
	)
	if err != nil || rebookedRes == nil {
		t.Fatalf("expected calendar nights to be available after pending expiration, got err=%v", err)
	}

	// 6. Verify Event Log Completeness
	if len(auditSvc.events) < 6 {
		t.Fatalf("expected at least 6 audit events recorded across lifecycle, got %d", len(auditSvc.events))
	}
}
