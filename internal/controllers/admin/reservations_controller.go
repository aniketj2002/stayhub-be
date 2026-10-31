package admin

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/http/middleware"
	"github.com/stayhub/backend/internal/http/response"
)

type ReservationsController struct {
	adminResService interfaces.AdminReservationService
}

func NewReservationsController(adminResService interfaces.AdminReservationService) *ReservationsController {
	return &ReservationsController{
		adminResService: adminResService,
	}
}

func (ctrl *ReservationsController) ListReservations(c *gin.Context) {
	filter, valErr := ValidateAdminReservationFilter(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	result, err := ctrl.adminResService.ListReservations(c.Request.Context(), filter)
	if err != nil {
		response.InternalError(c, "Failed to list reservations")
		return
	}

	response.OK(c, result)
}

func (ctrl *ReservationsController) GetReservationDetail(c *gin.Context) {
	id, valErr := ParseUUIDParam(c, "id")
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	result, err := ctrl.adminResService.GetReservationDetail(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Reservation not found")
			return
		}
		response.InternalError(c, "Failed to get reservation detail")
		return
	}

	response.OK(c, result)
}

func (ctrl *ReservationsController) CancelReservation(c *gin.Context) {
	id, valErr := ParseUUIDParam(c, "id")
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	input, valErr := ValidateAdminCancelReservation(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	result, err := ctrl.adminResService.CancelReservation(c.Request.Context(), actor, id, input)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Reservation not found")
			return
		}
		if errors.Is(err, apperrors.ErrValidation) || errors.Is(err, apperrors.ErrInvalidStateTransition) {
			response.Fail(c, http.StatusBadRequest, "CANCEL_FAILED", err.Error(), nil)
			return
		}
		response.InternalError(c, "Failed to cancel reservation")
		return
	}

	response.OK(c, result)
}

func (ctrl *ReservationsController) RefundReservation(c *gin.Context) {
	id, valErr := ParseUUIDParam(c, "id")
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	input, valErr := ValidateAdminRefundReservation(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	result, err := ctrl.adminResService.RefundReservation(c.Request.Context(), actor, id, input)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Reservation not found")
			return
		}
		if errors.Is(err, apperrors.ErrValidation) {
			response.Fail(c, http.StatusBadRequest, "REFUND_FAILED", err.Error(), nil)
			return
		}
		response.InternalError(c, "Failed to process refund")
		return
	}

	response.OK(c, result)
}
