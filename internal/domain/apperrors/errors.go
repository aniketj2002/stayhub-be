package apperrors

import "errors"

var (
	ErrNotFound                  = errors.New("resource not found")
	ErrUnauthorized              = errors.New("unauthorized")
	ErrForbidden                 = errors.New("forbidden")
	ErrEmailAlreadyExists        = errors.New("email is already registered")
	ErrInvalidCredentials        = errors.New("invalid email or password")
	ErrInvalidRefreshToken       = errors.New("invalid or expired refresh token")
	ErrMediaNotUploaded          = errors.New("referenced media is not in uploaded status")
	ErrMediaOwnership            = errors.New("media does not belong to an authorized listing host")
	ErrConflict                  = errors.New("resource state conflict")
	ErrValidation                = errors.New("validation failed")
	ErrPastDate                  = errors.New("booking dates cannot be in the past")
	ErrListingChecksFailed       = errors.New("automated listing checks failed")
	ErrCalendarNightBooked       = errors.New("one or more nights in range are already booked")
	ErrDatesUnavailable          = errors.New("selected dates are not available")
	ErrGuestLimitExceeded        = errors.New("guest count exceeds listing max_guests")
	ErrListingNotLive            = errors.New("listing is not currently live")
	ErrInvalidAmenityCode        = errors.New("one or more amenity codes are invalid")
	ErrInvalidCancellationPolicy = errors.New("cancellation policy code is invalid")
	ErrInvalidStateTransition    = errors.New("invalid reservation state transition")
	ErrSelfBookingNotAllowed     = errors.New("hosts cannot book their own listing")
	ErrRequestExpired            = errors.New("booking request has already expired")
	ErrLedgerUnbalanced          = errors.New("double-entry ledger transaction does not balance debit == credit")
)
