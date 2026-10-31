package review

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
	service  interfaces.ReviewService
	userRepo interfaces.UserRepository
}

func NewController(service interfaces.ReviewService, userRepo interfaces.UserRepository) *Controller {
	return &Controller{
		service:  service,
		userRepo: userRepo,
	}
}

// SubmitReview handles POST /reservations/:id/reviews
func (ctrl *Controller) SubmitReview(c *gin.Context) {
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

	var in dtos.CreateReviewInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "INVALID_JSON", "Invalid review payload", nil)
		return
	}

	user, err := ctrl.userRepo.FindByID(c.Request.Context(), actor.SubjectID)
	if err != nil || user == nil {
		response.NotFound(c, "User not found")
		return
	}

	res, err := ctrl.service.SubmitReview(c.Request.Context(), *user, resID, in)
	if err != nil {
		if errors.Is(err, apperrors.ErrValidation) {
			response.BadRequest(c, "VALIDATION_FAILED", err.Error(), nil)
			return
		}
		if errors.Is(err, apperrors.ErrForbidden) {
			response.Forbidden(c, err.Error())
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

// GetReservationReviews handles GET /reservations/:id/reviews
func (ctrl *Controller) GetReservationReviews(c *gin.Context) {
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

	res, err := ctrl.service.GetReservationReviews(c.Request.Context(), actor.SubjectID, resID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"reviews": res})
}

// ListListingReviews handles GET /listings/:id/reviews (Public)
func (ctrl *Controller) ListListingReviews(c *gin.Context) {
	listingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid listing ID", nil)
		return
	}

	res, err := ctrl.service.ListListingReviews(c.Request.Context(), listingID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Listing not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, res)
}
