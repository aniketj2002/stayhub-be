package listing

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/http/middleware"
	"github.com/stayhub/backend/internal/http/response"
	"github.com/stayhub/backend/internal/platform/clock"
)

type HostController struct {
	listingService  interfaces.ListingService
	calendarService interfaces.CalendarService
	clock           clock.Clock
}

func NewHostController(
	listingService interfaces.ListingService,
	calendarService interfaces.CalendarService,
	clk clock.Clock,
) *HostController {
	return &HostController{
		listingService:  listingService,
		calendarService: calendarService,
		clock:           clk,
	}
}

func (ctrl *HostController) CreateListing(c *gin.Context) {
	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	in, valErr := ValidateCreateListingInput(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	res, err := ctrl.listingService.CreateListing(c.Request.Context(), actor, in)
	if err != nil {
		response.InternalError(c, "Failed to create draft listing")
		return
	}

	response.Created(c, res)
}

func (ctrl *HostController) ListHostListings(c *gin.Context) {
	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	listings, err := ctrl.listingService.ListHostListings(c.Request.Context(), actor.SubjectID)
	if err != nil {
		response.InternalError(c, "Failed to list host listings")
		return
	}

	response.OK(c, gin.H{
		"listings": listings,
		"count":    len(listings),
	})
}

func (ctrl *HostController) GetHostListing(c *gin.Context) {
	listingID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.BadRequest(c, "INVALID_LISTING_ID", "id must be a valid UUID", nil)
		return
	}
	lh, ok := middleware.ListingHostFromGin(c)
	if !ok {
		response.Forbidden(c, "Listing host access required")
		return
	}

	res, err := ctrl.listingService.GetHostListing(c.Request.Context(), listingID, *lh)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Listing not found")
			return
		}
		response.InternalError(c, "Failed to load host listing")
		return
	}

	response.OK(c, res)
}

func (ctrl *HostController) UpdateWorkingVersion(c *gin.Context) {
	listingID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.BadRequest(c, "INVALID_LISTING_ID", "id must be a valid UUID", nil)
		return
	}
	lh, ok := middleware.ListingHostFromGin(c)
	if !ok {
		response.Forbidden(c, "Listing host access required")
		return
	}

	in, valErr := ValidateUpdateWorkingVersionInput(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	res, err := ctrl.listingService.UpdateWorkingVersion(c.Request.Context(), listingID, *lh, in)
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrNotFound):
			response.NotFound(c, "Listing or referenced media not found")
		case errors.Is(err, apperrors.ErrInvalidAmenityCode):
			response.BadRequest(c, "INVALID_AMENITY_CODE", "One or more amenity codes do not exist in the catalog", nil)
		case errors.Is(err, apperrors.ErrInvalidCancellationPolicy):
			response.BadRequest(c, "INVALID_CANCELLATION_POLICY", "Cancellation policy must be one of: flexible, moderate, firm, strict", nil)
		case errors.Is(err, apperrors.ErrMediaNotUploaded):
			response.BadRequest(c, "MEDIA_NOT_CONFIRMED", "All referenced media must be confirmed as uploaded", nil)
		case errors.Is(err, apperrors.ErrMediaOwnership):
			response.Forbidden(c, "All referenced media must be uploaded by an authorized host of this listing")
		default:
			response.InternalError(c, "Failed to update working version")
		}
		return
	}

	response.OK(c, res)
}

func (ctrl *HostController) SubmitListing(c *gin.Context) {
	listingID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.BadRequest(c, "INVALID_LISTING_ID", "id must be a valid UUID", nil)
		return
	}
	lh, ok := middleware.ListingHostFromGin(c)
	if !ok {
		response.Forbidden(c, "Listing host access required")
		return
	}

	res, err := ctrl.listingService.SubmitListing(c.Request.Context(), listingID, *lh)
	if err != nil {
		if errors.Is(err, apperrors.ErrListingChecksFailed) && res != nil {
			response.Fail(
				c,
				http.StatusUnprocessableEntity,
				"LISTING_CHECKS_FAILED",
				"One or more automated listing submission checks failed",
				res,
			)
			return
		}
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Listing not found")
			return
		}
		response.InternalError(c, "Failed to submit listing")
		return
	}

	response.OK(c, res)
}

func (ctrl *HostController) UpdateHostConfig(c *gin.Context) {
	listingID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.BadRequest(c, "INVALID_LISTING_ID", "id must be a valid UUID", nil)
		return
	}
	lh, ok := middleware.ListingHostFromGin(c)
	if !ok {
		response.Forbidden(c, "Listing host access required")
		return
	}

	in, valErr := ValidateUpdateHostConfigInput(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	updated, err := ctrl.listingService.UpdateHostConfig(c.Request.Context(), listingID, *lh, in)
	if err != nil {
		response.InternalError(c, "Failed to update host listing configuration")
		return
	}

	response.OK(c, gin.H{"host_access": updated})
}

func (ctrl *HostController) GetCalendar(c *gin.Context) {
	listingID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.BadRequest(c, "INVALID_LISTING_ID", "id must be a valid UUID", nil)
		return
	}

	in, valErr := ValidateCalendarQueryInput(c, ctrl.clock)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	res, err := ctrl.calendarService.GetCalendar(c.Request.Context(), listingID, in)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Listing not found")
			return
		}
		response.InternalError(c, "Failed to load listing calendar")
		return
	}

	response.OK(c, res)
}

func (ctrl *HostController) BlockCalendar(c *gin.Context) {
	listingID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.BadRequest(c, "INVALID_LISTING_ID", "id must be a valid UUID", nil)
		return
	}
	lh, ok := middleware.ListingHostFromGin(c)
	if !ok {
		response.Forbidden(c, "Listing host access required")
		return
	}

	in, valErr := ValidateCalendarBlockInput(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	res, err := ctrl.calendarService.BlockDates(c.Request.Context(), listingID, *lh, in)
	if err != nil {
		if errors.Is(err, apperrors.ErrCalendarNightBooked) {
			response.Conflict(c, "CALENDAR_NIGHT_BOOKED", "Cannot block dates that already have an active reservation", nil)
			return
		}
		response.InternalError(c, "Failed to block calendar dates")
		return
	}

	response.OK(c, res)
}

func (ctrl *HostController) UnblockCalendar(c *gin.Context) {
	listingID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.BadRequest(c, "INVALID_LISTING_ID", "id must be a valid UUID", nil)
		return
	}
	lh, ok := middleware.ListingHostFromGin(c)
	if !ok {
		response.Forbidden(c, "Listing host access required")
		return
	}

	in, valErr := ValidateCalendarUnblockInput(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	res, err := ctrl.calendarService.UnblockDates(c.Request.Context(), listingID, *lh, in)
	if err != nil {
		if errors.Is(err, apperrors.ErrCalendarNightBooked) {
			response.Conflict(c, "CALENDAR_NIGHT_BOOKED", "Cannot unblock nights held by an active reservation", nil)
			return
		}
		response.InternalError(c, "Failed to unblock calendar dates")
		return
	}

	response.OK(c, res)
}
