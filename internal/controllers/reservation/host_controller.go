package reservation

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/http/middleware"
	"github.com/stayhub/backend/internal/http/response"
)

type HostController struct {
	reservationService interfaces.ReservationService
}

func NewHostController(reservationService interfaces.ReservationService) *HostController {
	return &HostController{
		reservationService: reservationService,
	}
}

func (ctrl *HostController) GetHostToday(c *gin.Context) {
	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	todayView, err := ctrl.reservationService.GetHostToday(c.Request.Context(), actor.SubjectID)
	if err != nil {
		response.InternalError(c, "Failed to load host Today dashboard")
		return
	}

	response.OK(c, todayView)
}

func (ctrl *HostController) AcceptReservation(c *gin.Context) {
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

	idemKey := c.GetHeader(middleware.IdempotencyHeader)
	res, err := ctrl.reservationService.AcceptByHost(c.Request.Context(), actor.SubjectID, resID, idemKey)
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrNotFound):
			response.NotFound(c, "Reservation not found")
		case errors.Is(err, apperrors.ErrForbidden):
			response.Forbidden(c, "Missing can_manage_reservations permission for this listing")
		case errors.Is(err, apperrors.ErrRequestExpired):
			response.Conflict(c, "REQUEST_EXPIRED", "This booking request has expired (>24h)", nil)
		case errors.Is(err, apperrors.ErrInvalidStateTransition):
			response.Conflict(c, "INVALID_STATE_TRANSITION", "Only pending reservations can be accepted", nil)
		default:
			response.InternalError(c, "Failed to accept reservation")
		}
		return
	}

	response.OK(c, res)
}

func (ctrl *HostController) DeclineReservation(c *gin.Context) {
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

	in, valErr := ValidateDeclineReservationInput(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	idemKey := c.GetHeader(middleware.IdempotencyHeader)
	res, err := ctrl.reservationService.DeclineByHost(c.Request.Context(), actor.SubjectID, resID, idemKey, in)
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrNotFound):
			response.NotFound(c, "Reservation not found")
		case errors.Is(err, apperrors.ErrForbidden):
			response.Forbidden(c, "Missing can_manage_reservations permission for this listing")
		case errors.Is(err, apperrors.ErrInvalidStateTransition):
			response.Conflict(c, "INVALID_STATE_TRANSITION", "Only pending reservations can be declined", nil)
		default:
			response.InternalError(c, "Failed to decline reservation")
		}
		return
	}

	response.OK(c, res)
}

func (ctrl *HostController) CancelReservation(c *gin.Context) {
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
	res, err := ctrl.reservationService.CancelByHost(c.Request.Context(), actor.SubjectID, resID, idemKey, in)
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrNotFound):
			response.NotFound(c, "Reservation not found")
		case errors.Is(err, apperrors.ErrForbidden):
			response.Forbidden(c, "Missing can_manage_reservations permission for this listing")
		case errors.Is(err, apperrors.ErrInvalidStateTransition):
			response.Conflict(c, "INVALID_STATE_TRANSITION", "Only confirmed reservations can be cancelled by host", nil)
		default:
			response.InternalError(c, "Failed to cancel reservation")
		}
		return
	}

	response.OK(c, res)
}

func (ctrl *HostController) CheckInReservation(c *gin.Context) {
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

	idemKey := c.GetHeader(middleware.IdempotencyHeader)
	res, err := ctrl.reservationService.CheckInByHost(c.Request.Context(), actor.SubjectID, resID, idemKey)
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrNotFound):
			response.NotFound(c, "Reservation not found")
		case errors.Is(err, apperrors.ErrForbidden):
			response.Forbidden(c, "Missing can_manage_reservations permission for this listing")
		case errors.Is(err, apperrors.ErrInvalidStateTransition):
			response.Conflict(c, "INVALID_STATE_TRANSITION", "Only confirmed reservations can be checked in", nil)
		default:
			response.InternalError(c, "Failed to check in reservation")
		}
		return
	}

	response.OK(c, res)
}

func (ctrl *HostController) GetHostEarnings(c *gin.Context) {
	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	earnings, err := ctrl.reservationService.GetHostEarnings(c.Request.Context(), actor.SubjectID)
	if err != nil {
		response.InternalError(c, "Failed to load host earnings")
		return
	}

	response.OK(c, earnings)
}
