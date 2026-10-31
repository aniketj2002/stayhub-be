package reservation_alteration

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/http/response"
)

type Controller struct {
	service interfaces.ReservationAlterationService
}

func NewController(service interfaces.ReservationAlterationService) *Controller {
	return &Controller{service: service}
}

// RequestAlteration handles POST /reservations/:id/alterations
func (ctrl *Controller) RequestAlteration(c *gin.Context) {
	actor, ok := models.AuthActorFromContext(c.Request.Context())
	if !ok {
		response.Forbidden(c, "Unauthorized")
		return
	}

	resID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid reservation ID", nil)
		return
	}

	var in dtos.CreateReservationAlterationInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "INVALID_JSON", "Invalid alteration request payload", nil)
		return
	}

	res, err := ctrl.service.RequestAlteration(c.Request.Context(), actor.SubjectID, resID, in)
	if err != nil {
		if errors.Is(err, apperrors.ErrValidation) {
			response.BadRequest(c, "VALIDATION_FAILED", err.Error(), nil)
			return
		}
		if errors.Is(err, apperrors.ErrDatesUnavailable) {
			response.BadRequest(c, "DATES_UNAVAILABLE", err.Error(), nil)
			return
		}
		if errors.Is(err, apperrors.ErrForbidden) {
			response.Forbidden(c, "Forbidden")
			return
		}
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Reservation not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusCreated, res)
}

// ListAlterations handles GET /reservations/:id/alterations
func (ctrl *Controller) ListAlterations(c *gin.Context) {
	actor, ok := models.AuthActorFromContext(c.Request.Context())
	if !ok {
		response.Forbidden(c, "Unauthorized")
		return
	}

	resID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid reservation ID", nil)
		return
	}

	res, err := ctrl.service.ListAlterations(c.Request.Context(), actor.SubjectID, resID)
	if err != nil {
		if errors.Is(err, apperrors.ErrForbidden) {
			response.Forbidden(c, "Forbidden")
			return
		}
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Reservation not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"alterations": res})
}

// ListPendingForHost handles GET /host/date-change-requests
func (ctrl *Controller) ListPendingForHost(c *gin.Context) {
	actor, ok := models.AuthActorFromContext(c.Request.Context())
	if !ok {
		response.Forbidden(c, "Unauthorized")
		return
	}

	res, err := ctrl.service.ListPendingForHost(c.Request.Context(), actor.SubjectID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"alterations": res})
}

// ApproveAlteration handles POST /reservations/:id/alterations/:alterationId/approve
func (ctrl *Controller) ApproveAlteration(c *gin.Context) {
	actor, ok := models.AuthActorFromContext(c.Request.Context())
	if !ok {
		response.Forbidden(c, "Unauthorized")
		return
	}

	altID, err := uuid.Parse(c.Param("alterationId"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid alteration ID", nil)
		return
	}

	res, err := ctrl.service.ApproveAlteration(c.Request.Context(), actor, altID)
	if err != nil {
		if errors.Is(err, apperrors.ErrValidation) {
			response.BadRequest(c, "VALIDATION_FAILED", err.Error(), nil)
			return
		}
		if errors.Is(err, apperrors.ErrDatesUnavailable) {
			response.BadRequest(c, "DATES_UNAVAILABLE", err.Error(), nil)
			return
		}
		if errors.Is(err, apperrors.ErrForbidden) {
			response.Forbidden(c, "Forbidden: you do not have permission to manage reservations for this listing")
			return
		}
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Alteration request not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, res)
}

// DeclineAlteration handles POST /reservations/:id/alterations/:alterationId/decline
func (ctrl *Controller) DeclineAlteration(c *gin.Context) {
	actor, ok := models.AuthActorFromContext(c.Request.Context())
	if !ok {
		response.Forbidden(c, "Unauthorized")
		return
	}

	altID, err := uuid.Parse(c.Param("alterationId"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid alteration ID", nil)
		return
	}

	var in dtos.DeclineReservationAlterationInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "INVALID_JSON", "Invalid decline payload", nil)
		return
	}

	res, err := ctrl.service.DeclineAlteration(c.Request.Context(), actor, altID, in)
	if err != nil {
		if errors.Is(err, apperrors.ErrValidation) {
			response.BadRequest(c, "VALIDATION_FAILED", err.Error(), nil)
			return
		}
		if errors.Is(err, apperrors.ErrForbidden) {
			response.Forbidden(c, "Forbidden")
			return
		}
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Alteration request not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, res)
}
