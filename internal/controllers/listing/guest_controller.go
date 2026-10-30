package listing

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/controllers"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/http/response"
)

type GuestController struct {
	searchService interfaces.SearchService
}

func NewGuestController(searchService interfaces.SearchService) *GuestController {
	return &GuestController{
		searchService: searchService,
	}
}

func (ctrl *GuestController) SearchListings(c *gin.Context) {
	filter, valErr := ValidateSearchListingsFilter(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	cards, err := ctrl.searchService.SearchListings(c.Request.Context(), filter)
	if err != nil {
		response.InternalError(c, "Failed to search listings")
		return
	}

	response.OK(c, gin.H{
		"listings": cards,
		"count":    len(cards),
	})
}

func (ctrl *GuestController) GetListingDetail(c *gin.Context) {
	listingID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.BadRequest(c, "INVALID_LISTING_ID", "id must be a valid UUID", nil)
		return
	}

	detail, err := ctrl.searchService.GetPublicListingDetail(c.Request.Context(), listingID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Listing not found")
			return
		}
		response.InternalError(c, "Failed to load listing detail")
		return
	}

	response.OK(c, gin.H{"listing": detail})
}

func (ctrl *GuestController) QuoteStay(c *gin.Context) {
	listingID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.BadRequest(c, "INVALID_LISTING_ID", "id must be a valid UUID", nil)
		return
	}

	in, valErr := ValidateQuoteRequestInput(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	quote, err := ctrl.searchService.ComputeQuote(c.Request.Context(), listingID, in)
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrNotFound), errors.Is(err, apperrors.ErrListingNotLive):
			response.NotFound(c, "Listing is not available for booking")
		case errors.Is(err, apperrors.ErrPastDate):
			response.BadRequest(
				c,
				"PAST_DATE_NOT_ALLOWED",
				"Check-in date cannot be in the past",
				[]controllers.FieldError{{Field: "check_in", Message: "check_in date cannot be in the past"}},
			)
		case errors.Is(err, apperrors.ErrGuestLimitExceeded):
			response.Fail(
				c,
				http.StatusUnprocessableEntity,
				"GUEST_LIMIT_EXCEEDED",
				"Requested guest count exceeds listing max_guests capacity",
				nil,
			)
		case errors.Is(err, apperrors.ErrDatesUnavailable):
			response.Conflict(
				c,
				"DATES_UNAVAILABLE",
				"One or more nights in the selected date range are not available",
				nil,
			)
		default:
			response.InternalError(c, "Failed to compute stay quote")
		}
		return
	}

	response.OK(c, gin.H{"quote": quote})
}
