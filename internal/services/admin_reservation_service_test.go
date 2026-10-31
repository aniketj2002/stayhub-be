package services

import (
	"context"
	"errors"
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

func setupAdminReservationTest(now time.Time) (
	*AdminReservationService,
	*memReservationRepo,
	*memPaymentRepo,
	*memListingRepo,
	*memUserRepo,
	*memAuditService,
	clock.Clock,
) {
	cfg := &config.Config{PayoutReleaseAfterCheckInHours: 24}
	resRepo := newMemReservationRepo()
	payRepo := newMemPaymentRepo()
	listingRepo := newMemListingRepo()
	userRepo := newMemUserRepo()
	calRepo := newMemCalendarRepo()
	catalogSvc := &memCatalogService{}
	auditSvc := &memAuditService{}
	clk := clock.NewMockClock(now)

	ledgerSvc := NewLedgerService(payRepo, auditSvc)
	payoutSvc := NewPayoutService(cfg, &db.DB{}, nil, payRepo, resRepo, ledgerSvc, auditSvc, clk)

	svc := NewAdminReservationService(
		&db.DB{},
		resRepo,
		payRepo,
		listingRepo,
		calRepo,
		userRepo,
		nil,
		catalogSvc,
		ledgerSvc,
		payoutSvc,
		auditSvc,
		nil,
		clk,
	).(*AdminReservationService)

	return svc, resRepo, payRepo, listingRepo, userRepo, auditSvc, clk
}

func TestAdminReservationService_ListAndGetDetail(t *testing.T) {
	now := time.Date(2026, 12, 1, 12, 0, 0, 0, time.UTC)
	svc, resRepo, _, listingRepo, userRepo, _, _ := setupAdminReservationTest(now)
	ctx := context.Background()

	hostID := uuid.New()
	guest1ID := uuid.New()
	guest2ID := uuid.New()
	listingID := uuid.New()
	versionID := uuid.New()

	userRepo.users[hostID] = models.User{ID: hostID, FullName: "Alice Host", Email: "host@stayhub.dev"}
	userRepo.users[guest1ID] = models.User{ID: guest1ID, FullName: "Bob Guest", Email: "bob@stayhub.dev"}
	userRepo.users[guest2ID] = models.User{ID: guest2ID, FullName: "Charlie Guest", Email: "charlie@stayhub.dev"}

	listingTitle := "Mountain Hideaway"
	listingCity := "Innsbruck"
	listingCountry := "AT"
	listingRepo.versions[versionID] = models.ListingVersion{
		ID:        versionID,
		ListingID: listingID,
		Title:     &listingTitle,
		City:      &listingCity,
		Country:   &listingCountry,
	}

	res1 := models.Reservation{
		ID:                 uuid.New(),
		ListingID:          listingID,
		ListingVersionID:   versionID,
		HostID:             hostID,
		GuestID:            guest1ID,
		Status:             enums.ReservationStatusConfirmed,
		GuestTotalMinor:    30000,
		HostNetMinor:       26000,
		Currency:           "EUR",
		CancellationPolicy: "firm",
	}
	res2 := models.Reservation{
		ID:                 uuid.New(),
		ListingID:          listingID,
		ListingVersionID:   versionID,
		HostID:             hostID,
		GuestID:            guest2ID,
		Status:             enums.ReservationStatusCancelledByGuest,
		GuestTotalMinor:    15000,
		HostNetMinor:       13000,
		Currency:           "EUR",
		CancellationPolicy: "firm",
	}
	resRepo.rows[res1.ID] = res1
	resRepo.rows[res2.ID] = res2

	// List without filters
	listAll, err := svc.ListReservations(ctx, dtos.AdminReservationFilter{})
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if listAll.Total != 2 {
		t.Errorf("expected 2 reservations, got %d", listAll.Total)
	}

	// Filter by status
	statusFilter := enums.ReservationStatusConfirmed
	listConfirmed, err := svc.ListReservations(ctx, dtos.AdminReservationFilter{Status: &statusFilter})
	if err != nil {
		t.Fatalf("unexpected filter error: %v", err)
	}
	if listConfirmed.Total != 1 || listConfirmed.Reservations[0].Reservation.ID != res1.ID {
		t.Errorf("expected 1 confirmed reservation res1, got %d", listConfirmed.Total)
	}

	// Filter by guest
	listGuest2, err := svc.ListReservations(ctx, dtos.AdminReservationFilter{GuestID: &guest2ID})
	if err != nil {
		t.Fatalf("unexpected filter error: %v", err)
	}
	if listGuest2.Total != 1 || listGuest2.Reservations[0].Reservation.ID != res2.ID {
		t.Errorf("expected 1 Charlie reservation, got %d", listGuest2.Total)
	}

	// Detail check
	detail, err := svc.GetReservationDetail(ctx, res1.ID)
	if err != nil {
		t.Fatalf("unexpected detail error: %v", err)
	}
	if detail.Reservation.ID != res1.ID {
		t.Errorf("expected id %s, got %s", res1.ID, detail.Reservation.ID)
	}
	if detail.Listing.Title != "Mountain Hideaway" {
		t.Errorf("expected listing title Mountain Hideaway, got %s", detail.Listing.Title)
	}
	if detail.Guest.FullName != "Bob Guest" {
		t.Errorf("expected guest name Bob Guest, got %s", detail.Guest.FullName)
	}
	if detail.Refundable != 30000 {
		t.Errorf("expected refundable minor 30000, got %d", detail.Refundable)
	}
}

func TestAdminReservationService_CancelWithRefundModes(t *testing.T) {
	now := time.Date(2026, 12, 1, 12, 0, 0, 0, time.UTC)
	svc, resRepo, payRepo, _, _, auditSvc, _ := setupAdminReservationTest(now)
	ctx := context.Background()

	adminActor := models.AuthActor{
		SubjectID:   uuid.New(),
		SubjectType: "admin",
	}

	resID := uuid.New()
	checkIn := now.Add(48 * time.Hour)
	res := models.Reservation{
		ID:                 resID,
		Status:             enums.ReservationStatusConfirmed,
		CheckInAt:          checkIn,
		GuestTotalMinor:    50000,
		NightlyTotalMinor:  40000,
		CleaningFeeMinor:   5000,
		GuestFeeMinor:      5000,
		HostFeeMinor:       1500,
		HostNetMinor:       43500,
		Currency:           "EUR",
		CancellationPolicy: "moderate",
	}
	resRepo.rows[resID] = res

	// Add original charge payment
	chgID := uuid.New()
	payRepo.payments = append(payRepo.payments, models.Payment{
		ID:            chgID,
		ReservationID: resID,
		Kind:          enums.PaymentKindCharge,
		Status:        enums.PaymentStatusSucceeded,
		AmountMinor:   50000,
		Currency:      "EUR",
	})

	// 1. Empty reason rejected
	_, err := svc.CancelReservation(ctx, adminActor, resID, dtos.AdminCancelReservationInput{
		Reason:     "   ",
		RefundMode: "full",
	})
	if !errors.Is(err, apperrors.ErrValidation) {
		t.Fatalf("expected ErrValidation on whitespace reason, got %v", err)
	}

	// 2. Custom refund exceeding guest total rejected
	excessCustom := int64(60000)
	_, err = svc.CancelReservation(ctx, adminActor, resID, dtos.AdminCancelReservationInput{
		Reason:      "Customer requested full refund plus voucher",
		RefundMode:  "custom",
		AmountMinor: &excessCustom,
	})
	if !errors.Is(err, apperrors.ErrValidation) {
		t.Fatalf("expected ErrValidation on refund exceeding total, got %v", err)
	}

	// 3. Admin cancel with full refund
	detail, err := svc.CancelReservation(ctx, adminActor, resID, dtos.AdminCancelReservationInput{
		Reason:     "Emergency property water pipe burst",
		RefundMode: "full",
	})
	if err != nil {
		t.Fatalf("unexpected cancel error: %v", err)
	}

	if detail.Reservation.Status != enums.ReservationStatusCancelledByHost {
		t.Errorf("expected status %s, got %s", enums.ReservationStatusCancelledByHost, detail.Reservation.Status)
	}
	if detail.Refundable != 0 {
		t.Errorf("expected remaining refundable minor 0, got %d", detail.Refundable)
	}

	// Verify refund payment was created
	foundRefund := false
	for _, p := range detail.Payments {
		if p.Kind == enums.PaymentKindRefund && p.AmountMinor == 50000 {
			foundRefund = true
			break
		}
	}
	if !foundRefund {
		t.Errorf("expected 50000 refund payment in reservation payments, got %+v", detail.Payments)
	}

	// Verify audit log recorded
	if len(auditSvc.events) == 0 {
		t.Errorf("expected audit event to be recorded")
	} else {
		ev := auditSvc.events[len(auditSvc.events)-1]
		if ev.Action != "admin.reservation.cancel" {
			t.Errorf("expected action admin.reservation.cancel, got %s", ev.Action)
		}
	}
}

func TestAdminReservationService_DiscretionaryRefund(t *testing.T) {
	now := time.Date(2026, 12, 1, 12, 0, 0, 0, time.UTC)
	svc, resRepo, payRepo, _, _, auditSvc, _ := setupAdminReservationTest(now)
	ctx := context.Background()

	adminActor := models.AuthActor{
		SubjectID:   uuid.New(),
		SubjectType: "admin",
	}

	resID := uuid.New()
	res := models.Reservation{
		ID:                resID,
		Status:            enums.ReservationStatusCompleted,
		GuestTotalMinor:   40000,
		NightlyTotalMinor: 35000,
		CleaningFeeMinor:  2000,
		GuestFeeMinor:     3000,
		HostFeeMinor:      1000,
		HostNetMinor:      36000,
		Currency:          "EUR",
	}
	resRepo.rows[resID] = res

	chgID := uuid.New()
	payRepo.payments = append(payRepo.payments, models.Payment{
		ID:            chgID,
		ReservationID: resID,
		Kind:          enums.PaymentKindCharge,
		Status:        enums.PaymentStatusSucceeded,
		AmountMinor:   40000,
		Currency:      "EUR",
	})

	// 1. Validation: empty reason
	_, err := svc.RefundReservation(ctx, adminActor, resID, dtos.AdminRefundReservationInput{
		AmountMinor: 10000,
		Reason:      "",
	})
	if !errors.Is(err, apperrors.ErrValidation) {
		t.Fatalf("expected ErrValidation on empty reason, got %v", err)
	}

	// 2. Validation: zero or negative amount
	_, err = svc.RefundReservation(ctx, adminActor, resID, dtos.AdminRefundReservationInput{
		AmountMinor: 0,
		Reason:      "valid reason",
	})
	if !errors.Is(err, apperrors.ErrValidation) {
		t.Fatalf("expected ErrValidation on zero amount, got %v", err)
	}

	// 3. Issue partial discretionary refund of 15000
	det1, err := svc.RefundReservation(ctx, adminActor, resID, dtos.AdminRefundReservationInput{
		AmountMinor: 15000,
		Reason:      "Air conditioning broke on night 2",
	})
	if err != nil {
		t.Fatalf("unexpected refund error: %v", err)
	}
	if det1.Refundable != 25000 {
		t.Errorf("expected remaining refundable 25000, got %d", det1.Refundable)
	}

	// 4. Refund exceeding remaining refundable (25000 left, trying to refund 26000)
	_, err = svc.RefundReservation(ctx, adminActor, resID, dtos.AdminRefundReservationInput{
		AmountMinor: 26000,
		Reason:      "Further compensation",
	})
	if !errors.Is(err, apperrors.ErrValidation) {
		t.Fatalf("expected ErrValidation on amount exceeding remaining balance, got %v", err)
	}

	// 5. Refund exact remaining balance
	det2, err := svc.RefundReservation(ctx, adminActor, resID, dtos.AdminRefundReservationInput{
		AmountMinor: 25000,
		Reason:      "Settled remaining balance",
	})
	if err != nil {
		t.Fatalf("unexpected refund error: %v", err)
	}
	if det2.Refundable != 0 {
		t.Errorf("expected remaining refundable 0, got %d", det2.Refundable)
	}

	// Verify audit log
	var refundEvents int
	for _, ev := range auditSvc.events {
		if ev.Action == "admin.reservation.refund" {
			refundEvents++
		}
	}
	if refundEvents != 2 {
		t.Errorf("expected 2 admin.reservation.refund audit events, got %d", refundEvents)
	}
}
