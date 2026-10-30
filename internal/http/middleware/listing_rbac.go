package middleware

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/http/response"
)

type ListingPermission string

const (
	PermAnyHostMember      ListingPermission = "any_host_member"
	PermManageListing      ListingPermission = "can_manage_listing"
	PermManageCalendar     ListingPermission = "can_manage_calendar"
	PermManageReservations ListingPermission = "can_manage_reservations"
	PermMessageGuests      ListingPermission = "can_message_guests"
	PermViewInsights       ListingPermission = "can_view_insights"
	PermOwnerOnly          ListingPermission = "owner_only"
)

func ListingHostFromGin(c *gin.Context) (*models.ListingHost, bool) {
	if v, ok := c.Get("listing_host"); ok {
		if lh, ok := v.(*models.ListingHost); ok && lh != nil {
			return lh, true
		}
	}
	return models.ListingHostFromContext(c.Request.Context())
}

func HasListingPermission(lh *models.ListingHost, perm ListingPermission) bool {
	if lh == nil {
		return false
	}
	if lh.Role == enums.ListingHostRoleOwner {
		return true
	}
	switch perm {
	case PermAnyHostMember:
		return true
	case PermManageListing:
		return lh.CanManageListing
	case PermManageCalendar:
		return lh.CanManageCalendar
	case PermManageReservations:
		return lh.CanManageReservations
	case PermMessageGuests:
		return lh.CanMessageGuests
	case PermViewInsights:
		return lh.CanViewInsights
	case PermOwnerOnly:
		return false
	default:
		return false
	}
}

// RequireListingPermission queries `listing_hosts` for (listing_id, user_id) and enforces
// the requested scoped permission (can_manage_listing, can_manage_calendar, etc.).
func RequireListingPermission(
	lhRepo interfaces.ListingHostRepository,
	paramName string,
	perm ListingPermission,
) gin.HandlerFunc {
	if paramName == "" {
		paramName = "id"
	}
	return func(c *gin.Context) {
		actor, ok := ActorFromGin(c)
		if !ok {
			response.Unauthorized(c, "Authentication is required")
			return
		}

		rawListingID := c.Param(paramName)
		listingID, err := uuid.Parse(rawListingID)
		if err != nil {
			response.BadRequest(c, "INVALID_LISTING_ID", fmt.Sprintf("Parameter '%s' must be a valid UUID", paramName), nil)
			return
		}

		lh, err := lhRepo.FindByListingAndUser(c.Request.Context(), listingID, actor.SubjectID)
		if err != nil {
			if errors.Is(err, apperrors.ErrNotFound) {
				response.Fail(
					c,
					http.StatusForbidden,
					"LISTING_ACCESS_DENIED",
					"You do not have host or co-host access to this listing",
					nil,
				)
				return
			}
			response.InternalError(c, "Failed to verify listing host permissions")
			return
		}

		if !HasListingPermission(lh, perm) {
			response.Fail(
				c,
				http.StatusForbidden,
				"LISTING_PERMISSION_DENIED",
				fmt.Sprintf("Missing required listing permission: %s", perm),
				nil,
			)
			return
		}

		c.Set("listing_host", lh)
		ctx := models.WithListingHost(c.Request.Context(), lh)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
