package cohost

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/controllers"
	"github.com/stayhub/backend/internal/domain/dtos"
)

func ValidateCreateCohostInvite(c *gin.Context) (dtos.CreateCohostInviteInput, *controllers.ValidationError) {
	var in dtos.CreateCohostInviteInput
	if err := c.ShouldBindJSON(&in); err != nil {
		return in, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
	}

	var fieldErrs []controllers.FieldError
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if in.Email == "" || !strings.Contains(in.Email, "@") {
		fieldErrs = append(fieldErrs, controllers.FieldError{
			Field:   "email",
			Message: "Valid email is required",
		})
	}

	if in.PayoutShareBPS < 0 || in.PayoutShareBPS > 10000 {
		fieldErrs = append(fieldErrs, controllers.FieldError{
			Field:   "payout_share_bps",
			Message: "payout_share_bps must be between 0 and 10000",
		})
	}

	if len(fieldErrs) > 0 {
		return in, controllers.NewValidationError("VALIDATION_ERROR", "Validation failed", fieldErrs...)
	}
	return in, nil
}

func ValidateUpdateCohost(c *gin.Context) (dtos.UpdateCohostInput, *controllers.ValidationError) {
	var in dtos.UpdateCohostInput
	if err := c.ShouldBindJSON(&in); err != nil {
		return in, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
	}

	var fieldErrs []controllers.FieldError
	if in.PayoutShareBPS != nil && (*in.PayoutShareBPS < 0 || *in.PayoutShareBPS > 10000) {
		fieldErrs = append(fieldErrs, controllers.FieldError{
			Field:   "payout_share_bps",
			Message: "payout_share_bps must be between 0 and 10000",
		})
	}

	if len(fieldErrs) > 0 {
		return in, controllers.NewValidationError("VALIDATION_ERROR", "Validation failed", fieldErrs...)
	}
	return in, nil
}
