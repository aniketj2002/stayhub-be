package pricing

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/controllers"
	"github.com/stayhub/backend/internal/domain/dtos"
)

func ValidatePricingRuleInput(c *gin.Context) (dtos.ListingPricingRuleInput, *controllers.ValidationError) {
	var in dtos.ListingPricingRuleInput
	if err := c.ShouldBindJSON(&in); err != nil {
		return in, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
	}

	var fieldErrs []controllers.FieldError
	if in.MinNights < 0 {
		fieldErrs = append(fieldErrs, controllers.FieldError{
			Field:   "min_nights",
			Message: "min_nights must be at least 1",
		})
	}
	if in.WeeklyDiscountBPS < 0 || in.WeeklyDiscountBPS > 10000 {
		fieldErrs = append(fieldErrs, controllers.FieldError{
			Field:   "weekly_discount_bps",
			Message: "weekly_discount_bps must be between 0 and 10000",
		})
	}
	if in.MonthlyDiscountBPS < 0 || in.MonthlyDiscountBPS > 10000 {
		fieldErrs = append(fieldErrs, controllers.FieldError{
			Field:   "monthly_discount_bps",
			Message: "monthly_discount_bps must be between 0 and 10000",
		})
	}
	if in.WeekendPriceMinor != nil && *in.WeekendPriceMinor < 0 {
		fieldErrs = append(fieldErrs, controllers.FieldError{
			Field:   "weekend_price_minor",
			Message: "weekend_price_minor must not be negative",
		})
	}

	if len(fieldErrs) > 0 {
		return in, controllers.NewValidationError("VALIDATION_ERROR", "Validation failed", fieldErrs...)
	}
	return in, nil
}

func ValidateDateOverrideRange(c *gin.Context) (dtos.DateOverrideRangeInput, *controllers.ValidationError) {
	var in dtos.DateOverrideRangeInput
	if err := c.ShouldBindJSON(&in); err != nil {
		return in, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
	}

	var fieldErrs []controllers.FieldError
	from, err := time.Parse("2006-01-02", in.From)
	if err != nil {
		fieldErrs = append(fieldErrs, controllers.FieldError{Field: "from", Message: "from must be YYYY-MM-DD"})
	}
	to, err := time.Parse("2006-01-02", in.To)
	if err != nil {
		fieldErrs = append(fieldErrs, controllers.FieldError{Field: "to", Message: "to must be YYYY-MM-DD"})
	}
	if err == nil && to.Before(from) {
		fieldErrs = append(fieldErrs, controllers.FieldError{Field: "to", Message: "to date must be on or after from date"})
	}
	if in.PriceMinor < 0 {
		fieldErrs = append(fieldErrs, controllers.FieldError{Field: "price_minor", Message: "price_minor must not be negative"})
	}
	if in.MinNights != nil && *in.MinNights < 1 {
		fieldErrs = append(fieldErrs, controllers.FieldError{Field: "min_nights", Message: "min_nights must be at least 1"})
	}

	if len(fieldErrs) > 0 {
		return in, controllers.NewValidationError("VALIDATION_ERROR", "Validation failed", fieldErrs...)
	}
	return in, nil
}

func ValidateDateOverrideDelete(c *gin.Context) (time.Time, time.Time, *controllers.ValidationError) {
	fromStr := c.Query("from")
	toStr := c.Query("to")

	var fieldErrs []controllers.FieldError
	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		fieldErrs = append(fieldErrs, controllers.FieldError{Field: "from", Message: "from query parameter must be YYYY-MM-DD"})
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		fieldErrs = append(fieldErrs, controllers.FieldError{Field: "to", Message: "to query parameter must be YYYY-MM-DD"})
	}
	if err == nil && to.Before(from) {
		fieldErrs = append(fieldErrs, controllers.FieldError{Field: "to", Message: "to date must be on or after from date"})
	}

	if len(fieldErrs) > 0 {
		return time.Time{}, time.Time{}, controllers.NewValidationError("VALIDATION_ERROR", "Validation failed", fieldErrs...)
	}
	return from, to, nil
}
