package cohost

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/http/response"
)

type Controller struct {
	cohostService interfaces.CohostService
	userRepo      interfaces.UserRepository
}

func NewController(cohostService interfaces.CohostService, userRepo interfaces.UserRepository) *Controller {
	return &Controller{
		cohostService: cohostService,
		userRepo:      userRepo,
	}
}

func (ctrl *Controller) InviteCohost(c *gin.Context) {
	listingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid listing ID format", nil)
		return
	}

	actor, ok := models.AuthActorFromContext(c.Request.Context())
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	hostAccess, ok := models.ListingHostFromContext(c.Request.Context())
	if !ok || hostAccess.Role != enums.ListingHostRoleOwner {
		response.Forbidden(c, "Only listing owner can invite co-hosts")
		return
	}

	in, valErr := ValidateCreateCohostInvite(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	res, rawToken, err := ctrl.cohostService.InviteCohost(c.Request.Context(), actor.SubjectID, listingID, in)
	if err != nil {
		if errors.Is(err, apperrors.ErrValidation) {
			response.BadRequest(c, "VALIDATION_ERROR", err.Error(), nil)
			return
		}
		response.InternalError(c, "Failed to invite cohost")
		return
	}

	response.Created(c, gin.H{
		"invitation": res,
		"token":      rawToken,
	})
}

func (ctrl *Controller) ListInvitations(c *gin.Context) {
	listingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid listing ID format", nil)
		return
	}

	res, err := ctrl.cohostService.ListInvitations(c.Request.Context(), listingID)
	if err != nil {
		response.InternalError(c, "Failed to list invitations")
		return
	}
	response.OK(c, gin.H{"invitations": res})
}

func (ctrl *Controller) CancelInvitation(c *gin.Context) {
	listingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid listing ID format", nil)
		return
	}
	inviteID, err := uuid.Parse(c.Param("inviteId"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid invite ID format", nil)
		return
	}

	hostAccess, ok := models.ListingHostFromContext(c.Request.Context())
	if !ok || hostAccess.Role != enums.ListingHostRoleOwner {
		response.Forbidden(c, "Only listing owner can cancel invitations")
		return
	}

	if err := ctrl.cohostService.CancelInvitation(c.Request.Context(), listingID, inviteID); err != nil {
		if errors.Is(err, apperrors.ErrForbidden) {
			response.Forbidden(c, "Forbidden")
			return
		}
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Invitation not found")
			return
		}
		if errors.Is(err, apperrors.ErrValidation) {
			response.BadRequest(c, "VALIDATION_ERROR", err.Error(), nil)
			return
		}
		response.InternalError(c, "Failed to cancel invitation")
		return
	}
	c.Status(http.StatusNoContent)
}

func (ctrl *Controller) GetInvitationDetails(c *gin.Context) {
	token := c.Param("token")
	if strings.TrimSpace(token) == "" {
		response.BadRequest(c, "INVALID_TOKEN", "Invitation token is required", nil)
		return
	}

	res, err := ctrl.cohostService.GetInvitationDetails(c.Request.Context(), token)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Invitation not found")
			return
		}
		response.InternalError(c, "Failed to get invitation details")
		return
	}

	response.OK(c, res)
}

func (ctrl *Controller) AcceptInvitation(c *gin.Context) {
	token := c.Param("token")
	actor, ok := models.AuthActorFromContext(c.Request.Context())
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	user, err := ctrl.userRepo.FindByID(c.Request.Context(), actor.SubjectID)
	if err != nil {
		response.Unauthorized(c, "User not found")
		return
	}

	res, err := ctrl.cohostService.AcceptInvitation(c.Request.Context(), *user, token)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Invitation not found")
			return
		}
		if errors.Is(err, apperrors.ErrForbidden) {
			response.Forbidden(c, err.Error())
			return
		}
		if errors.Is(err, apperrors.ErrValidation) {
			response.BadRequest(c, "VALIDATION_ERROR", err.Error(), nil)
			return
		}
		response.InternalError(c, "Failed to accept invitation")
		return
	}
	response.OK(c, res)
}

func (ctrl *Controller) ListCohosts(c *gin.Context) {
	listingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid listing ID format", nil)
		return
	}

	res, err := ctrl.cohostService.ListCohosts(c.Request.Context(), listingID)
	if err != nil {
		response.InternalError(c, "Failed to list cohosts")
		return
	}
	response.OK(c, gin.H{"cohosts": res})
}

func (ctrl *Controller) UpdateCohost(c *gin.Context) {
	listingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid listing ID format", nil)
		return
	}
	targetUserID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid user ID format", nil)
		return
	}

	hostAccess, ok := models.ListingHostFromContext(c.Request.Context())
	if !ok || hostAccess.Role != enums.ListingHostRoleOwner {
		response.Forbidden(c, "Only listing owner can update cohosts")
		return
	}

	in, valErr := ValidateUpdateCohost(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	res, err := ctrl.cohostService.UpdateCohost(c.Request.Context(), listingID, targetUserID, in)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Cohost member not found")
			return
		}
		if errors.Is(err, apperrors.ErrForbidden) {
			response.Forbidden(c, err.Error())
			return
		}
		if errors.Is(err, apperrors.ErrValidation) {
			response.BadRequest(c, "VALIDATION_ERROR", err.Error(), nil)
			return
		}
		response.InternalError(c, "Failed to update cohost")
		return
	}
	response.OK(c, res)
}

func (ctrl *Controller) RemoveCohost(c *gin.Context) {
	listingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid listing ID format", nil)
		return
	}
	targetUserID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid user ID format", nil)
		return
	}

	hostAccess, ok := models.ListingHostFromContext(c.Request.Context())
	if !ok || hostAccess.Role != enums.ListingHostRoleOwner {
		response.Forbidden(c, "Only listing owner can remove cohosts")
		return
	}

	if err := ctrl.cohostService.RemoveCohost(c.Request.Context(), listingID, targetUserID); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Cohost member not found")
			return
		}
		if errors.Is(err, apperrors.ErrForbidden) {
			response.Forbidden(c, err.Error())
			return
		}
		response.InternalError(c, "Failed to remove cohost")
		return
	}
	c.Status(http.StatusNoContent)
}
