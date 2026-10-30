package statemachine

import (
	"errors"
	"testing"

	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/enums"
)

func TestReservationStateMachine(t *testing.T) {
	validCases := []struct {
		from enums.ReservationStatus
		to   enums.ReservationStatus
	}{
		{enums.ReservationStatusPending, enums.ReservationStatusConfirmed},
		{enums.ReservationStatusPending, enums.ReservationStatusDeclined},
		{enums.ReservationStatusPending, enums.ReservationStatusCancelledByGuest},
		{enums.ReservationStatusConfirmed, enums.ReservationStatusCheckedIn},
		{enums.ReservationStatusConfirmed, enums.ReservationStatusCancelledByGuest},
		{enums.ReservationStatusConfirmed, enums.ReservationStatusCancelledByHost},
		{enums.ReservationStatusCheckedIn, enums.ReservationStatusCompleted},
		{enums.ReservationStatusCompleted, enums.ReservationStatusReviewed},
	}

	for _, tc := range validCases {
		if err := ValidateReservationTransition(tc.from, tc.to); err != nil {
			t.Fatalf("expected transition %s -> %s to be valid, got err: %v", tc.from, tc.to, err)
		}
	}

	invalidCases := []struct {
		from enums.ReservationStatus
		to   enums.ReservationStatus
	}{
		{enums.ReservationStatusPending, enums.ReservationStatusCheckedIn},
		{enums.ReservationStatusPending, enums.ReservationStatusCompleted},
		{enums.ReservationStatusPending, enums.ReservationStatusCancelledByHost},
		{enums.ReservationStatusConfirmed, enums.ReservationStatusDeclined},
		{enums.ReservationStatusConfirmed, enums.ReservationStatusPending},
		{enums.ReservationStatusCheckedIn, enums.ReservationStatusCancelledByGuest},
		{enums.ReservationStatusCheckedIn, enums.ReservationStatusCancelledByHost},
		{enums.ReservationStatusCompleted, enums.ReservationStatusCancelledByGuest},
		{enums.ReservationStatusDeclined, enums.ReservationStatusConfirmed},
		{enums.ReservationStatusCancelledByGuest, enums.ReservationStatusConfirmed},
		{enums.ReservationStatusCancelledByHost, enums.ReservationStatusCheckedIn},
	}

	for _, tc := range invalidCases {
		err := ValidateReservationTransition(tc.from, tc.to)
		if !errors.Is(err, apperrors.ErrInvalidStateTransition) {
			t.Fatalf("expected ErrInvalidStateTransition for %s -> %s, got %v", tc.from, tc.to, err)
		}
	}

	if !IsTerminalReservationStatus(enums.ReservationStatusDeclined) ||
		!IsTerminalReservationStatus(enums.ReservationStatusCancelledByGuest) ||
		!IsTerminalReservationStatus(enums.ReservationStatusCancelledByHost) ||
		!IsTerminalReservationStatus(enums.ReservationStatusReviewed) {
		t.Fatalf("expected terminal reservation statuses to be recognized")
	}
	if IsTerminalReservationStatus(enums.ReservationStatusPending) ||
		IsTerminalReservationStatus(enums.ReservationStatusConfirmed) {
		t.Fatalf("expected non-terminal reservation statuses not to be marked terminal")
	}
}

func TestListingAndVersionAndPayoutStateMachines(t *testing.T) {
	// Listing transitions
	if err := ValidateListingTransition(enums.ListingStatusDraft, enums.ListingStatusLive); err != nil {
		t.Fatalf("expected draft -> live to be valid: %v", err)
	}
	if err := ValidateListingTransition(enums.ListingStatusArchived, enums.ListingStatusLive); !errors.Is(err, apperrors.ErrInvalidStateTransition) {
		t.Fatalf("expected archived -> live to fail with ErrInvalidStateTransition, got %v", err)
	}

	// Listing version transitions
	if err := ValidateVersionTransition(enums.VersionStatusDraft, enums.VersionStatusApproved); err != nil {
		t.Fatalf("expected draft -> approved to be valid: %v", err)
	}
	if err := ValidateVersionTransition(enums.VersionStatusApproved, enums.VersionStatusSuperseded); err != nil {
		t.Fatalf("expected approved -> superseded to be valid: %v", err)
	}
	if err := ValidateVersionTransition(enums.VersionStatusSuperseded, enums.VersionStatusDraft); !errors.Is(err, apperrors.ErrInvalidStateTransition) {
		t.Fatalf("expected superseded -> draft to fail with ErrInvalidStateTransition, got %v", err)
	}

	// Payout transitions
	if err := ValidatePayoutTransition(enums.PayoutStatusScheduled, enums.PayoutStatusReleased); err != nil {
		t.Fatalf("expected scheduled -> released to be valid: %v", err)
	}
	if err := ValidatePayoutTransition(enums.PayoutStatusScheduled, enums.PayoutStatusCancelled); err != nil {
		t.Fatalf("expected scheduled -> cancelled to be valid: %v", err)
	}
	if err := ValidatePayoutTransition(enums.PayoutStatusReleased, enums.PayoutStatusCancelled); !errors.Is(err, apperrors.ErrInvalidStateTransition) {
		t.Fatalf("expected released -> cancelled to fail with ErrInvalidStateTransition, got %v", err)
	}

	// Payment & Media transitions
	if err := ValidatePaymentTransition(enums.PaymentStatusPending, enums.PaymentStatusSucceeded); err != nil {
		t.Fatalf("expected pending -> succeeded payment transition to be valid: %v", err)
	}
	if err := ValidatePaymentTransition(enums.PaymentStatusSucceeded, enums.PaymentStatusPending); !errors.Is(err, apperrors.ErrInvalidStateTransition) {
		t.Fatalf("expected succeeded -> pending payment transition to fail, got %v", err)
	}
	if err := ValidateMediaTransition(MediaStatusPending, MediaStatusUploaded); err != nil {
		t.Fatalf("expected pending -> uploaded media transition to be valid: %v", err)
	}
}
