package itinerary

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/http/response"
)

type Controller struct {
	service interfaces.ItineraryService
}

func NewController(service interfaces.ItineraryService) *Controller {
	return &Controller{service: service}
}

// GetItinerary handles GET /guest/trips/:id/itinerary or GET /reservations/:id/itinerary
func (ctrl *Controller) GetItinerary(c *gin.Context) {
	actor, ok := models.AuthActorFromContext(c.Request.Context())
	if !ok {
		response.Forbidden(c, "Unauthorized")
		return
	}

	reservationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid reservation ID", nil)
		return
	}

	res, err := ctrl.service.GetItinerary(c.Request.Context(), actor.SubjectID, reservationID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Reservation not found")
			return
		}
		if errors.Is(err, apperrors.ErrForbidden) {
			response.Forbidden(c, "You do not have access to this reservation itinerary")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, res)
}
