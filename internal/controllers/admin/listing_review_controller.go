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

type ListingReviewController struct {
	reviewService interfaces.AdminListingReviewService
}

func NewListingReviewController(reviewService interfaces.AdminListingReviewService) *ListingReviewController {
	return &ListingReviewController{
		reviewService: reviewService,
	}
}

func (ctrl *ListingReviewController) ListQueue(c *gin.Context) {
	filter := ValidateListingReviewFilter(c)
	queue, err := ctrl.reviewService.ListQueue(c.Request.Context(), filter)
	if err != nil {
		response.InternalError(c, "Failed to load listing review queue")
		return
	}

	response.OK(c, gin.H{
		"queue": queue,
		"count": len(queue),
	})
}

func (ctrl *ListingReviewController) GetDiff(c *gin.Context) {
	versionID, valErr := ParseUUIDParam(c, "versionId")
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	diff, err := ctrl.reviewService.GetDiff(c.Request.Context(), versionID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Listing version not found")
			return
		}
		response.InternalError(c, "Failed to compute listing diff")
		return
	}

	response.OK(c, diff)
}

func (ctrl *ListingReviewController) Approve(c *gin.Context) {
	versionID, valErr := ParseUUIDParam(c, "versionId")
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	input, valErr := ValidateListingReviewDecision(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	err := ctrl.reviewService.Approve(c.Request.Context(), actor, versionID, input.Note)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Listing version not found")
			return
		}
		if errors.Is(err, apperrors.ErrInvalidStateTransition) || errors.Is(err, apperrors.ErrValidation) {
			response.Fail(c, http.StatusBadRequest, "APPROVE_FAILED", err.Error(), nil)
			return
		}
		response.InternalError(c, "Failed to approve listing version")
		return
	}

	response.OK(c, gin.H{"status": "approved", "version_id": versionID})
}

func (ctrl *ListingReviewController) RequestChanges(c *gin.Context) {
	versionID, valErr := ParseUUIDParam(c, "versionId")
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	input, valErr := ValidateListingReviewDecision(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	err := ctrl.reviewService.RequestChanges(c.Request.Context(), actor, versionID, input.Note)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Listing version not found")
			return
		}
		if errors.Is(err, apperrors.ErrInvalidStateTransition) || errors.Is(err, apperrors.ErrValidation) {
			response.Fail(c, http.StatusBadRequest, "REQUEST_CHANGES_FAILED", err.Error(), nil)
			return
		}
		response.InternalError(c, "Failed to request changes on listing version")
		return
	}

	response.OK(c, gin.H{"status": "changes_requested", "version_id": versionID})
}

func (ctrl *ListingReviewController) Reject(c *gin.Context) {
	versionID, valErr := ParseUUIDParam(c, "versionId")
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	input, valErr := ValidateListingReviewDecision(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	err := ctrl.reviewService.Reject(c.Request.Context(), actor, versionID, input.Note)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Listing version not found")
			return
		}
		if errors.Is(err, apperrors.ErrInvalidStateTransition) || errors.Is(err, apperrors.ErrValidation) {
			response.Fail(c, http.StatusBadRequest, "REJECT_FAILED", err.Error(), nil)
			return
		}
		response.InternalError(c, "Failed to reject listing version")
		return
	}

	response.OK(c, gin.H{"status": "rejected", "version_id": versionID})
}
