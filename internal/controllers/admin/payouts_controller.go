package admin

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/http/middleware"
	"github.com/stayhub/backend/internal/http/response"
	"github.com/stayhub/backend/internal/services"
)

type PayoutsController struct {
	payoutService interfaces.PayoutService
}

func NewPayoutsController(payoutService interfaces.PayoutService) *PayoutsController {
	return &PayoutsController{
		payoutService: payoutService,
	}
}

func (ctrl *PayoutsController) HoldPayout(c *gin.Context) {
	id, valErr := ParseUUIDParam(c, "id")
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	input, valErr := ValidatePayoutHold(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	payout, err := ctrl.payoutService.HoldPayout(c.Request.Context(), actor.SubjectID, id, input.Reason)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Payout not found")
			return
		}
		if errors.Is(err, apperrors.ErrInvalidStateTransition) || errors.Is(err, apperrors.ErrValidation) {
			response.Fail(c, http.StatusBadRequest, "HOLD_FAILED", err.Error(), nil)
			return
		}
		response.InternalError(c, "Failed to hold payout")
		return
	}

	response.OK(c, payout)
}

func (ctrl *PayoutsController) ReleasePayout(c *gin.Context) {
	id, valErr := ParseUUIDParam(c, "id")
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	input, valErr := ValidatePayoutRelease(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	payout, err := ctrl.payoutService.ReleasePayoutHold(c.Request.Context(), actor.SubjectID, id, input.Reason)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Payout not found")
			return
		}
		if errors.Is(err, services.ErrPayoutNotHeld) || errors.Is(err, apperrors.ErrValidation) {
			response.Fail(c, http.StatusBadRequest, "RELEASE_FAILED", err.Error(), nil)
			return
		}
		response.InternalError(c, "Failed to release payout hold")
		return
	}

	response.OK(c, payout)
}
