package reservation

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/http/middleware"
	"github.com/stayhub/backend/internal/http/response"
)

type GuestController struct {
	reservationService interfaces.ReservationService
}

func NewGuestController(reservationService interfaces.ReservationService) *GuestController {
	return &GuestController{
		reservationService: reservationService,
	}
}

func (ctrl *GuestController) CreateReservation(c *gin.Context) {
	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	in, valErr := ValidateCreateReservationInput(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	idemKey := c.GetHeader(middleware.IdempotencyHeader)
	res, err := ctrl.reservationService.CreateReservation(c.Request.Context(), actor, idemKey, in)
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrNotFound), errors.Is(err, apperrors.ErrListingNotLive):
			response.NotFound(c, "Listing is not available for booking")
		case errors.Is(err, apperrors.ErrPastDate):
			response.BadRequest(
				c,
				"PAST_DATE_NOT_ALLOWED",
				"Check-in date cannot be in the past",
				[]map[string]string{{"field": "check_in", "message": "check_in date cannot be in the past"}},
			)
		case errors.Is(err, apperrors.ErrSelfBookingNotAllowed):
			response.Forbidden(c, "Hosts and co-hosts cannot book their own listing")
		case errors.Is(err, apperrors.ErrGuestLimitExceeded):
			response.Fail(
				c,
				http.StatusUnprocessableEntity,
				"GUEST_LIMIT_EXCEEDED",
				"Requested guest count exceeds listing max_guests capacity",
				nil,
			)
		case errors.Is(err, apperrors.ErrDatesUnavailable):
			response.Conflict(
				c,
				"DATES_UNAVAILABLE",
				"One or more nights in the selected date range are already booked or blocked",
				nil,
			)
		default:
			response.InternalError(c, "Failed to create reservation")
		}
		return
	}

	response.Created(c, res)
}

func (ctrl *GuestController) ListGuestTrips(c *gin.Context) {
	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	tab := c.Query("tab")
	trips, err := ctrl.reservationService.ListGuestTrips(c.Request.Context(), actor.SubjectID, tab)
	if err != nil {
		response.InternalError(c, "Failed to load guest trips")
		return
	}

	response.OK(c, gin.H{
		"trips": trips,
		"count": len(trips),
	})
}

func (ctrl *GuestController) GetReservationDetail(c *gin.Context) {
	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	resID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.BadRequest(c, "INVALID_RESERVATION_ID", "id must be a valid UUID", nil)
		return
	}

	res, err := ctrl.reservationService.GetReservationDetail(c.Request.Context(), actor, resID)
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrNotFound):
			response.NotFound(c, "Reservation not found")
		case errors.Is(err, apperrors.ErrForbidden):
			response.Forbidden(c, "You do not have access to this reservation")
		default:
			response.InternalError(c, "Failed to load reservation detail")
		}
		return
	}

	response.OK(c, res)
}

func (ctrl *GuestController) GetCancelPreview(c *gin.Context) {
	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	resID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.BadRequest(c, "INVALID_RESERVATION_ID", "id must be a valid UUID", nil)
		return
	}

	preview, err := ctrl.reservationService.GetCancelPreview(c.Request.Context(), actor.SubjectID, resID)
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrNotFound):
			response.NotFound(c, "Reservation not found")
		case errors.Is(err, apperrors.ErrForbidden):
			response.Forbidden(c, "Only the booking guest can preview cancellation")
		case errors.Is(err, apperrors.ErrInvalidStateTransition):
			response.Conflict(c, "INVALID_STATE_TRANSITION", "Reservation cannot be cancelled in its current status", nil)
		default:
			response.InternalError(c, "Failed to compute cancellation preview")
		}
		return
	}

	response.OK(c, gin.H{"cancel_preview": preview})
}

func (ctrl *GuestController) CancelByGuest(c *gin.Context) {
	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	resID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.BadRequest(c, "INVALID_RESERVATION_ID", "id must be a valid UUID", nil)
		return
	}

	in, valErr := ValidateCancelReservationInput(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	idemKey := c.GetHeader(middleware.IdempotencyHeader)
	res, err := ctrl.reservationService.CancelByGuest(c.Request.Context(), actor.SubjectID, resID, idemKey, in)
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrNotFound):
			response.NotFound(c, "Reservation not found")
		case errors.Is(err, apperrors.ErrForbidden):
			response.Forbidden(c, "Only the booking guest can cancel this reservation")
		case errors.Is(err, apperrors.ErrInvalidStateTransition):
			response.Conflict(c, "INVALID_STATE_TRANSITION", "Reservation cannot be cancelled from its current status", nil)
		default:
			response.InternalError(c, "Failed to cancel reservation")
		}
		return
	}

	response.OK(c, res)
}
