package pricing

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/interfaces"
	domainPricing "github.com/stayhub/backend/internal/domain/pricing"
	"github.com/stayhub/backend/internal/http/response"
)

type Controller struct {
	pricingService interfaces.PricingService
}

func NewController(pricingService interfaces.PricingService) *Controller {
	return &Controller{pricingService: pricingService}
}

func (ctrl *Controller) GetPricingRules(c *gin.Context) {
	listingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid listing ID format", nil)
		return
	}

	res, err := ctrl.pricingService.GetPricingRule(c.Request.Context(), listingID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Pricing rules not found")
			return
		}
		response.InternalError(c, "Failed to get pricing rules")
		return
	}
	response.OK(c, res)
}

func (ctrl *Controller) SetPricingRules(c *gin.Context) {
	listingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid listing ID format", nil)
		return
	}

	in, valErr := ValidatePricingRuleInput(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	res, err := ctrl.pricingService.SetPricingRule(c.Request.Context(), listingID, in)
	if err != nil {
		if errors.Is(err, apperrors.ErrValidation) {
			response.BadRequest(c, "VALIDATION_ERROR", err.Error(), nil)
			return
		}
		response.InternalError(c, "Failed to set pricing rules")
		return
	}
	response.OK(c, res)
}

func (ctrl *Controller) GetDateOverrides(c *gin.Context) {
	listingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid listing ID format", nil)
		return
	}

	fromStr := c.DefaultQuery("from", time.Now().Format("2006-01-02"))
	toStr := c.DefaultQuery("to", time.Now().AddDate(1, 0, 0).Format("2006-01-02"))

	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		response.BadRequest(c, "INVALID_DATE", "from must be YYYY-MM-DD", nil)
		return
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		response.BadRequest(c, "INVALID_DATE", "to must be YYYY-MM-DD", nil)
		return
	}

	res, err := ctrl.pricingService.GetDateOverrides(c.Request.Context(), listingID, from, to)
	if err != nil {
		response.InternalError(c, "Failed to get date overrides")
		return
	}
	response.OK(c, gin.H{"overrides": res})
}

func (ctrl *Controller) SetDateOverrides(c *gin.Context) {
	listingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid listing ID format", nil)
		return
	}

	in, valErr := ValidateDateOverrideRange(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	if err := ctrl.pricingService.SetDateOverrides(c.Request.Context(), listingID, in); err != nil {
		if errors.Is(err, apperrors.ErrValidation) {
			response.BadRequest(c, "VALIDATION_ERROR", err.Error(), nil)
			return
		}
		response.InternalError(c, "Failed to set date overrides")
		return
	}
	c.Status(http.StatusNoContent)
}

func (ctrl *Controller) DeleteDateOverrides(c *gin.Context) {
	listingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid listing ID format", nil)
		return
	}

	from, to, valErr := ValidateDateOverrideDelete(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	if err := ctrl.pricingService.DeleteDateOverrides(c.Request.Context(), listingID, from, to); err != nil {
		response.InternalError(c, "Failed to delete date overrides")
		return
	}
	c.Status(http.StatusNoContent)
}

func (ctrl *Controller) GetQuote(c *gin.Context) {
	listingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid listing ID format", nil)
		return
	}

	checkInStr := c.Query("check_in")
	checkOutStr := c.Query("check_out")

	checkIn, err := time.Parse("2006-01-02", checkInStr)
	if err != nil {
		response.BadRequest(c, "INVALID_DATE", "check_in must be YYYY-MM-DD", nil)
		return
	}
	checkOut, err := time.Parse("2006-01-02", checkOutStr)
	if err != nil {
		response.BadRequest(c, "INVALID_DATE", "check_out must be YYYY-MM-DD", nil)
		return
	}

	breakdown, err := ctrl.pricingService.CalculateQuote(c.Request.Context(), listingID, checkIn, checkOut)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Listing not found or not live")
			return
		}
		if errors.Is(err, domainPricing.ErrMinNightsNotMet) || errors.Is(err, domainPricing.ErrInvalidDates) || errors.Is(err, apperrors.ErrValidation) {
			response.BadRequest(c, "VALIDATION_ERROR", err.Error(), nil)
			return
		}
		response.InternalError(c, "Failed to calculate quote")
		return
	}
	response.OK(c, gin.H{"breakdown": breakdown})
}
