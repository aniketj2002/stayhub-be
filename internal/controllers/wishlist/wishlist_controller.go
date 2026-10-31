package wishlist

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
	wishlistService interfaces.WishlistService
}

func NewController(wishlistService interfaces.WishlistService) *Controller {
	return &Controller{
		wishlistService: wishlistService,
	}
}

func (ctrl *Controller) ListWishlists(c *gin.Context) {
	actor, ok := models.AuthActorFromContext(c.Request.Context())
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	wishlists, err := ctrl.wishlistService.GetUserWishlists(c.Request.Context(), actor.SubjectID)
	if err != nil {
		response.InternalError(c, "Failed to list wishlists")
		return
	}

	response.OK(c, wishlists)
}

func (ctrl *Controller) CreateWishlist(c *gin.Context) {
	actor, ok := models.AuthActorFromContext(c.Request.Context())
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	var req dtos.CreateWishlistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_BODY", "Invalid request body", nil)
		return
	}

	w, err := ctrl.wishlistService.CreateWishlist(c.Request.Context(), actor.SubjectID, req)
	if err != nil {
		if errors.Is(err, apperrors.ErrValidation) {
			response.BadRequest(c, "VALIDATION_ERROR", err.Error(), nil)
			return
		}
		response.InternalError(c, "Failed to create wishlist")
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": w})
}

func (ctrl *Controller) GetWishlist(c *gin.Context) {
	actor, ok := models.AuthActorFromContext(c.Request.Context())
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	wishlistID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid wishlist ID", nil)
		return
	}

	w, err := ctrl.wishlistService.GetWishlist(c.Request.Context(), actor.SubjectID, wishlistID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Wishlist not found")
			return
		}
		if errors.Is(err, apperrors.ErrForbidden) {
			response.Forbidden(c, "Access denied")
			return
		}
		response.InternalError(c, "Failed to get wishlist")
		return
	}

	response.OK(c, w)
}

func (ctrl *Controller) UpdateWishlist(c *gin.Context) {
	actor, ok := models.AuthActorFromContext(c.Request.Context())
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	wishlistID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid wishlist ID", nil)
		return
	}

	var req dtos.UpdateWishlistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_BODY", "Invalid request body", nil)
		return
	}

	w, err := ctrl.wishlistService.UpdateWishlist(c.Request.Context(), actor.SubjectID, wishlistID, req)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Wishlist not found")
			return
		}
		if errors.Is(err, apperrors.ErrForbidden) {
			response.Forbidden(c, "Access denied")
			return
		}
		if errors.Is(err, apperrors.ErrValidation) {
			response.BadRequest(c, "VALIDATION_ERROR", err.Error(), nil)
			return
		}
		response.InternalError(c, "Failed to update wishlist")
		return
	}

	response.OK(c, w)
}

func (ctrl *Controller) DeleteWishlist(c *gin.Context) {
	actor, ok := models.AuthActorFromContext(c.Request.Context())
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	wishlistID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid wishlist ID", nil)
		return
	}

	err = ctrl.wishlistService.DeleteWishlist(c.Request.Context(), actor.SubjectID, wishlistID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Wishlist not found")
			return
		}
		if errors.Is(err, apperrors.ErrForbidden) {
			response.Forbidden(c, "Access denied")
			return
		}
		response.InternalError(c, "Failed to delete wishlist")
		return
	}

	c.Status(http.StatusNoContent)
}

func (ctrl *Controller) AddListing(c *gin.Context) {
	actor, ok := models.AuthActorFromContext(c.Request.Context())
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	wishlistID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid wishlist ID", nil)
		return
	}

	listingID, err := uuid.Parse(c.Param("listingId"))
	if err != nil {
		response.BadRequest(c, "INVALID_LISTING_ID", "Invalid listing ID", nil)
		return
	}

	err = ctrl.wishlistService.AddListing(c.Request.Context(), actor.SubjectID, wishlistID, listingID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Wishlist or listing not found")
			return
		}
		if errors.Is(err, apperrors.ErrForbidden) {
			response.Forbidden(c, "Access denied")
			return
		}
		response.InternalError(c, "Failed to add listing to wishlist")
		return
	}

	c.Status(http.StatusNoContent)
}

func (ctrl *Controller) RemoveListing(c *gin.Context) {
	actor, ok := models.AuthActorFromContext(c.Request.Context())
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	wishlistID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid wishlist ID", nil)
		return
	}

	listingID, err := uuid.Parse(c.Param("listingId"))
	if err != nil {
		response.BadRequest(c, "INVALID_LISTING_ID", "Invalid listing ID", nil)
		return
	}

	err = ctrl.wishlistService.RemoveListing(c.Request.Context(), actor.SubjectID, wishlistID, listingID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Wishlist not found")
			return
		}
		if errors.Is(err, apperrors.ErrForbidden) {
			response.Forbidden(c, "Access denied")
			return
		}
		response.InternalError(c, "Failed to remove listing from wishlist")
		return
	}

	c.Status(http.StatusNoContent)
}

func (ctrl *Controller) ShareWishlist(c *gin.Context) {
	actor, ok := models.AuthActorFromContext(c.Request.Context())
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	wishlistID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid wishlist ID", nil)
		return
	}

	res, err := ctrl.wishlistService.ShareWishlist(c.Request.Context(), actor.SubjectID, wishlistID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Wishlist not found")
			return
		}
		if errors.Is(err, apperrors.ErrForbidden) {
			response.Forbidden(c, "Access denied")
			return
		}
		response.InternalError(c, "Failed to share wishlist")
		return
	}

	response.OK(c, res)
}

func (ctrl *Controller) UnshareWishlist(c *gin.Context) {
	actor, ok := models.AuthActorFromContext(c.Request.Context())
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	wishlistID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid wishlist ID", nil)
		return
	}

	err = ctrl.wishlistService.UnshareWishlist(c.Request.Context(), actor.SubjectID, wishlistID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Wishlist not found")
			return
		}
		if errors.Is(err, apperrors.ErrForbidden) {
			response.Forbidden(c, "Access denied")
			return
		}
		response.InternalError(c, "Failed to unshare wishlist")
		return
	}

	c.Status(http.StatusNoContent)
}

func (ctrl *Controller) GetSharedWishlist(c *gin.Context) {
	token := c.Param("token")
	w, err := ctrl.wishlistService.GetSharedWishlist(c.Request.Context(), token)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Shared wishlist not found or link has expired")
			return
		}
		response.InternalError(c, "Failed to get shared wishlist")
		return
	}

	response.OK(c, w)
}
