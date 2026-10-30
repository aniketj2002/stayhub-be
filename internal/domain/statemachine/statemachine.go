package statemachine

import (
	"fmt"

	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/enums"
)

// ============================================================================
// 1. Reservation Status State Machine
// ============================================================================

var reservationTransitions = map[enums.ReservationStatus][]enums.ReservationStatus{
	enums.ReservationStatusInquiry: {
		enums.ReservationStatusPending,
		enums.ReservationStatusConfirmed,
		enums.ReservationStatusDeclined,
		enums.ReservationStatusCancelledByGuest,
	},
	enums.ReservationStatusPending: {
		enums.ReservationStatusConfirmed,        // Host accepts request within 24h
		enums.ReservationStatusDeclined,         // Host declines or 24h system auto-expiry
		enums.ReservationStatusCancelledByGuest, // Guest withdraws pending request (100% refund)
	},
	enums.ReservationStatusConfirmed: {
		enums.ReservationStatusCheckedIn,        // Host manual check-in or worker auto check-in at check_in_at
		enums.ReservationStatusCancelledByGuest, // Guest cancels before check-in (policy-based refund)
		enums.ReservationStatusCancelledByHost,  // Host cancels before check-in (100% refund)
		enums.ReservationStatusDisputed,         // V2 Dispute opened
	},
	enums.ReservationStatusCheckedIn: {
		enums.ReservationStatusCompleted, // Worker auto-completes at check_out_at
		enums.ReservationStatusDisputed,  // V2 Dispute opened during stay
	},
	enums.ReservationStatusCompleted: {
		enums.ReservationStatusReviewed, // V2 Review completed
		enums.ReservationStatusDisputed, // V2 Post-stay dispute
	},
	enums.ReservationStatusDisputed: {
		enums.ReservationStatusCompleted,        // Admin resolves in favor of completion
		enums.ReservationStatusCancelledByGuest, // Admin resolves as guest cancellation
		enums.ReservationStatusCancelledByHost,  // Admin resolves as host cancellation
	},
	// Terminal states: no outgoing transitions
	enums.ReservationStatusReviewed:         {},
	enums.ReservationStatusDeclined:         {},
	enums.ReservationStatusCancelledByGuest: {},
	enums.ReservationStatusCancelledByHost:  {},
}

// CanTransitionReservation returns true if a reservation is allowed to move from `from` to `to`.
func CanTransitionReservation(from, to enums.ReservationStatus) bool {
	allowed, ok := reservationTransitions[from]
	if !ok {
		return false
	}
	for _, candidate := range allowed {
		if candidate == to {
			return true
		}
	}
	return false
}

// ValidateReservationTransition validates `from -> to` and returns apperrors.ErrInvalidStateTransition if disallowed.
func ValidateReservationTransition(from, to enums.ReservationStatus) error {
	if !CanTransitionReservation(from, to) {
		return fmt.Errorf(
			"%w: reservation cannot transition from %q to %q",
			apperrors.ErrInvalidStateTransition,
			from,
			to,
		)
	}
	return nil
}

// AllowedNextReservationStates returns a copy of all valid target states from `from`.
func AllowedNextReservationStates(from enums.ReservationStatus) []enums.ReservationStatus {
	allowed := reservationTransitions[from]
	out := make([]enums.ReservationStatus, len(allowed))
	copy(out, allowed)
	return out
}

// IsTerminalReservationStatus returns true if the reservation is in a final state with no outgoing transitions.
func IsTerminalReservationStatus(status enums.ReservationStatus) bool {
	allowed, ok := reservationTransitions[status]
	return ok && len(allowed) == 0
}

// ============================================================================
// 2. Listing Status State Machine
// ============================================================================

var listingTransitions = map[enums.ListingStatus][]enums.ListingStatus{
	enums.ListingStatusDraft: {
		enums.ListingStatusDraft,
		enums.ListingStatusPendingReview,
		enums.ListingStatusLive, // V1 auto-approve when automated checks pass
		enums.ListingStatusRejected,
		enums.ListingStatusArchived,
	},
	enums.ListingStatusPendingReview: {
		enums.ListingStatusLive,
		enums.ListingStatusRejected,
		enums.ListingStatusDraft,
		enums.ListingStatusArchived,
	},
	enums.ListingStatusRejected: {
		enums.ListingStatusDraft,
		enums.ListingStatusPendingReview,
		enums.ListingStatusLive,
		enums.ListingStatusArchived,
	},
	enums.ListingStatusLive: {
		enums.ListingStatusLive,          // Subsequent version approved while live
		enums.ListingStatusPendingReview, // V2 edit submitted while live
		enums.ListingStatusPaused,        // Host snoozes/pauses listing
		enums.ListingStatusArchived,      // Host permanently archives listing
	},
	enums.ListingStatusPaused: {
		enums.ListingStatusLive,
		enums.ListingStatusArchived,
	},
	// Terminal state
	enums.ListingStatusArchived: {},
}

func CanTransitionListing(from, to enums.ListingStatus) bool {
	allowed, ok := listingTransitions[from]
	if !ok {
		return false
	}
	for _, candidate := range allowed {
		if candidate == to {
			return true
		}
	}
	return false
}

func ValidateListingTransition(from, to enums.ListingStatus) error {
	if !CanTransitionListing(from, to) {
		return fmt.Errorf(
			"%w: listing cannot transition from %q to %q",
			apperrors.ErrInvalidStateTransition,
			from,
			to,
		)
	}
	return nil
}

// ============================================================================
// 3. Listing Version Status State Machine
// ============================================================================

var versionTransitions = map[enums.VersionStatus][]enums.VersionStatus{
	enums.VersionStatusDraft: {
		enums.VersionStatusDraft,
		enums.VersionStatusSubmitted,
		enums.VersionStatusApproved, // V1 auto-approve on submit when checks pass
		enums.VersionStatusRejected,
	},
	enums.VersionStatusSubmitted: {
		enums.VersionStatusApproved,
		enums.VersionStatusChangesRequested,
		enums.VersionStatusRejected,
	},
	enums.VersionStatusChangesRequested: {
		enums.VersionStatusDraft,
		enums.VersionStatusSubmitted,
		enums.VersionStatusApproved,
		enums.VersionStatusRejected,
	},
	enums.VersionStatusRejected: {
		enums.VersionStatusDraft,
		enums.VersionStatusSubmitted,
		enums.VersionStatusApproved,
	},
	enums.VersionStatusApproved: {
		enums.VersionStatusSuperseded, // Superseded when a newer version is approved
	},
	// Terminal state
	enums.VersionStatusSuperseded: {},
}

func CanTransitionVersion(from, to enums.VersionStatus) bool {
	allowed, ok := versionTransitions[from]
	if !ok {
		return false
	}
	for _, candidate := range allowed {
		if candidate == to {
			return true
		}
	}
	return false
}

func ValidateVersionTransition(from, to enums.VersionStatus) error {
	if !CanTransitionVersion(from, to) {
		return fmt.Errorf(
			"%w: listing version cannot transition from %q to %q",
			apperrors.ErrInvalidStateTransition,
			from,
			to,
		)
	}
	return nil
}

// ============================================================================
// 4. Payout Status State Machine
// ============================================================================

var payoutTransitions = map[enums.PayoutStatus][]enums.PayoutStatus{
	enums.PayoutStatusScheduled: {
		enums.PayoutStatusScheduled, // Partial refund adjustment keeps status scheduled
		enums.PayoutStatusReleased,  // Released at checked_in_at + 24h
		enums.PayoutStatusHeld,      // Held for admin/dispute review
		enums.PayoutStatusCancelled, // Cancelled on 100% refund
		enums.PayoutStatusFailed,
	},
	enums.PayoutStatusHeld: {
		enums.PayoutStatusScheduled,
		enums.PayoutStatusReleased,
		enums.PayoutStatusCancelled,
	},
	enums.PayoutStatusFailed: {
		enums.PayoutStatusScheduled,
		enums.PayoutStatusReleased,
		enums.PayoutStatusCancelled,
	},
	// Terminal states
	enums.PayoutStatusReleased:  {},
	enums.PayoutStatusCancelled: {},
}

func CanTransitionPayout(from, to enums.PayoutStatus) bool {
	allowed, ok := payoutTransitions[from]
	if !ok {
		return false
	}
	for _, candidate := range allowed {
		if candidate == to {
			return true
		}
	}
	return false
}

func ValidatePayoutTransition(from, to enums.PayoutStatus) error {
	if !CanTransitionPayout(from, to) {
		return fmt.Errorf(
			"%w: payout cannot transition from %q to %q",
			apperrors.ErrInvalidStateTransition,
			from,
			to,
		)
	}
	return nil
}

// ============================================================================
// 5. Payment Status State Machine
// ============================================================================

var paymentTransitions = map[enums.PaymentStatus][]enums.PaymentStatus{
	enums.PaymentStatusPending: {
		enums.PaymentStatusSucceeded,
		enums.PaymentStatusFailed,
	},
	// Terminal states
	enums.PaymentStatusSucceeded: {},
	enums.PaymentStatusFailed:    {},
}

func CanTransitionPayment(from, to enums.PaymentStatus) bool {
	allowed, ok := paymentTransitions[from]
	if !ok {
		return false
	}
	for _, candidate := range allowed {
		if candidate == to {
			return true
		}
	}
	return false
}

func ValidatePaymentTransition(from, to enums.PaymentStatus) error {
	if !CanTransitionPayment(from, to) {
		return fmt.Errorf(
			"%w: payment cannot transition from %q to %q",
			apperrors.ErrInvalidStateTransition,
			from,
			to,
		)
	}
	return nil
}

// ============================================================================
// 6. Media Upload Status State Machine
// ============================================================================

const (
	MediaStatusPending  = "pending"
	MediaStatusUploaded = "uploaded"
)

var mediaTransitions = map[string][]string{
	MediaStatusPending:  {MediaStatusUploaded},
	MediaStatusUploaded: {MediaStatusUploaded}, // Idempotent confirm
}

func CanTransitionMedia(from, to string) bool {
	allowed, ok := mediaTransitions[from]
	if !ok {
		return false
	}
	for _, candidate := range allowed {
		if candidate == to {
			return true
		}
	}
	return false
}

func ValidateMediaTransition(from, to string) error {
	if !CanTransitionMedia(from, to) {
		return fmt.Errorf(
			"%w: media cannot transition from %q to %q",
			apperrors.ErrInvalidStateTransition,
			from,
			to,
		)
	}
	return nil
}
