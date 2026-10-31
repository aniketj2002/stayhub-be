package admin

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/controllers"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
)

func ValidateListingReviewFilter(c *gin.Context) dtos.AdminListingReviewFilter {
	status := enums.VersionStatus(strings.TrimSpace(c.Query("status")))
	if status == "" {
		status = enums.VersionStatusSubmitted
	}
	return dtos.AdminListingReviewFilter{
		Status: status,
	}
}

func ValidateListingReviewDecision(c *gin.Context) (dtos.AdminListingDecisionInput, *controllers.ValidationError) {
	var input dtos.AdminListingDecisionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		return input, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
	}

	input.Note = strings.TrimSpace(input.Note)
	if input.Note == "" {
		return input, controllers.NewValidationError(
			"VALIDATION_ERROR",
			"Validation failed",
			controllers.FieldError{Field: "note", Message: "Decision note is required for listing review actions"},
		)
	}
	return input, nil
}
