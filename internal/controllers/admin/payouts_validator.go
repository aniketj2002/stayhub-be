package admin

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/controllers"
	"github.com/stayhub/backend/internal/domain/dtos"
)

func ValidatePayoutHold(c *gin.Context) (dtos.AdminPayoutHoldInput, *controllers.ValidationError) {
	var input dtos.AdminPayoutHoldInput
	if err := c.ShouldBindJSON(&input); err != nil {
		return input, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
	}

	input.Reason = strings.TrimSpace(input.Reason)
	if input.Reason == "" {
		return input, controllers.NewValidationError(
			"VALIDATION_ERROR",
			"Validation failed",
			controllers.FieldError{Field: "reason", Message: "Reason is required to hold a payout"},
		)
	}
	return input, nil
}

func ValidatePayoutRelease(c *gin.Context) (dtos.AdminPayoutReleaseInput, *controllers.ValidationError) {
	var input dtos.AdminPayoutReleaseInput
	if err := c.ShouldBindJSON(&input); err != nil {
		return input, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
	}

	input.Reason = strings.TrimSpace(input.Reason)
	if input.Reason == "" {
		return input, controllers.NewValidationError(
			"VALIDATION_ERROR",
			"Validation failed",
			controllers.FieldError{Field: "reason", Message: "Reason is required to release a payout hold"},
		)
	}
	return input, nil
}
