package media

import (
	"io"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/controllers"
	"github.com/stayhub/backend/internal/domain/dtos"
)

const maxMediaSizeBytes = 20 * 1024 * 1024 // 20 MB

var allowedMediaContentTypes = map[string]bool{
	"image/jpeg": true,
	"image/jpg":  true,
	"image/png":  true,
	"image/webp": true,
	"image/avif": true,
	"image/gif":  true,
}

var allowedMediaPurposes = map[string]bool{
	"listing_photo": true,
	"avatar":        true,
	"evidence":      true,
}

func ValidatePresignMediaInput(c *gin.Context) (dtos.PresignMediaInput, *controllers.ValidationError) {
	var in dtos.PresignMediaInput
	if err := c.ShouldBindJSON(&in); err != nil {
		return dtos.PresignMediaInput{}, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
	}

	var errs []controllers.FieldError
	in.ContentType = strings.ToLower(strings.TrimSpace(in.ContentType))
	in.Filename = strings.TrimSpace(in.Filename)
	in.Purpose = strings.ToLower(strings.TrimSpace(in.Purpose))

	if in.ContentType == "" {
		errs = append(errs, controllers.FieldError{Field: "content_type", Message: "content_type is required"})
	} else if !allowedMediaContentTypes[in.ContentType] {
		errs = append(errs, controllers.FieldError{
			Field:   "content_type",
			Message: "content_type must be one of: image/jpeg, image/png, image/webp, image/avif, image/gif",
		})
	}

	if in.SizeBytes <= 0 {
		errs = append(errs, controllers.FieldError{Field: "size_bytes", Message: "size_bytes must be greater than 0"})
	} else if in.SizeBytes > maxMediaSizeBytes {
		errs = append(errs, controllers.FieldError{Field: "size_bytes", Message: "size_bytes must not exceed 20971520 (20 MB)"})
	}

	if in.Width != nil && *in.Width <= 0 {
		errs = append(errs, controllers.FieldError{Field: "width", Message: "width must be a positive integer"})
	}
	if in.Height != nil && *in.Height <= 0 {
		errs = append(errs, controllers.FieldError{Field: "height", Message: "height must be a positive integer"})
	}

	if in.Purpose == "" {
		in.Purpose = "listing_photo"
	} else if !allowedMediaPurposes[in.Purpose] {
		errs = append(errs, controllers.FieldError{
			Field:   "purpose",
			Message: "purpose must be one of: listing_photo, avatar, evidence",
		})
	}

	if len(errs) > 0 {
		return dtos.PresignMediaInput{}, controllers.NewValidationError("VALIDATION_ERROR", "Invalid media presign payload", errs...)
	}
	return in, nil
}

func ValidateConfirmMediaInput(c *gin.Context) (uuid.UUID, dtos.ConfirmMediaInput, *controllers.ValidationError) {
	mediaID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		return uuid.Nil, dtos.ConfirmMediaInput{}, controllers.NewValidationError(
			"VALIDATION_ERROR",
			"Invalid media ID",
			controllers.FieldError{Field: "id", Message: "id must be a valid UUID"},
		)
	}

	var in dtos.ConfirmMediaInput
	if c.Request.Body != nil {
		if err := c.ShouldBindJSON(&in); err != nil && err != io.EOF {
			return uuid.Nil, dtos.ConfirmMediaInput{}, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
		}
	}

	var errs []controllers.FieldError
	if in.Width != nil && *in.Width <= 0 {
		errs = append(errs, controllers.FieldError{Field: "width", Message: "width must be a positive integer"})
	}
	if in.Height != nil && *in.Height <= 0 {
		errs = append(errs, controllers.FieldError{Field: "height", Message: "height must be a positive integer"})
	}
	if len(errs) > 0 {
		return uuid.Nil, dtos.ConfirmMediaInput{}, controllers.NewValidationError("VALIDATION_ERROR", "Invalid media confirm payload", errs...)
	}

	return mediaID, in, nil
}
