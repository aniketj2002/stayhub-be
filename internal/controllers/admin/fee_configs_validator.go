package admin

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/controllers"
	"github.com/stayhub/backend/internal/domain/dtos"
)

func ValidateProposeFeeConfig(c *gin.Context) (dtos.ProposeFeeConfigInput, *controllers.ValidationError) {
	var input dtos.ProposeFeeConfigInput
	if err := c.ShouldBindJSON(&input); err != nil {
		return input, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
	}

	var fieldErrs []controllers.FieldError
	input.Reason = strings.TrimSpace(input.Reason)
	if input.Reason == "" {
		fieldErrs = append(fieldErrs, controllers.FieldError{
			Field:   "reason",
			Message: "Reason is required to propose fee configuration change",
		})
	}
	if input.GuestFeeBPS < 0 {
		fieldErrs = append(fieldErrs, controllers.FieldError{
			Field:   "guest_fee_bps",
			Message: "guest_fee_bps must not be negative",
		})
	}
	if input.HostFeeBPS < 0 {
		fieldErrs = append(fieldErrs, controllers.FieldError{
			Field:   "host_fee_bps",
			Message: "host_fee_bps must not be negative",
		})
	}

	if len(fieldErrs) > 0 {
		return input, controllers.NewValidationError("VALIDATION_ERROR", "Validation failed", fieldErrs...)
	}
	return input, nil
}

func ValidateRejectFeeConfig(c *gin.Context) (dtos.RejectFeeConfigInput, *controllers.ValidationError) {
	var input dtos.RejectFeeConfigInput
	if err := c.ShouldBindJSON(&input); err != nil {
		return input, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
	}

	input.Reason = strings.TrimSpace(input.Reason)
	if input.Reason == "" {
		return input, controllers.NewValidationError(
			"VALIDATION_ERROR",
			"Validation failed",
			controllers.FieldError{Field: "reason", Message: "Reason is required to reject a fee configuration"},
		)
	}
	return input, nil
}
