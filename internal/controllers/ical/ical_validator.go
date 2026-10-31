package ical

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/controllers"
	"github.com/stayhub/backend/internal/domain/dtos"
	platformIcal "github.com/stayhub/backend/internal/platform/ical"
)

func ValidateCreateICalFeedInput(c *gin.Context) (dtos.CreateICalFeedInput, *controllers.ValidationError) {
	var in dtos.CreateICalFeedInput
	if err := c.ShouldBindJSON(&in); err != nil {
		return in, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
	}

	var fieldErrs []controllers.FieldError
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		fieldErrs = append(fieldErrs, controllers.FieldError{
			Field:   "name",
			Message: "Feed name is required",
		})
	}

	in.URL = strings.TrimSpace(in.URL)
	if in.URL == "" {
		fieldErrs = append(fieldErrs, controllers.FieldError{
			Field:   "url",
			Message: "Feed URL is required",
		})
	} else if err := platformIcal.ValidateSSRF(in.URL); err != nil {
		fieldErrs = append(fieldErrs, controllers.FieldError{
			Field:   "url",
			Message: err.Error(),
		})
	}

	if len(fieldErrs) > 0 {
		return in, controllers.NewValidationError("VALIDATION_ERROR", "Validation failed", fieldErrs...)
	}
	return in, nil
}
