package auth

import (
	"net/mail"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/controllers"
	"github.com/stayhub/backend/internal/domain/dtos"
)

func ValidateRegisterUserInput(c *gin.Context) (dtos.RegisterUserInput, *controllers.ValidationError) {
	var in dtos.RegisterUserInput
	if err := c.ShouldBindJSON(&in); err != nil {
		return dtos.RegisterUserInput{}, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
	}

	var errs []controllers.FieldError
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.FullName = strings.TrimSpace(in.FullName)

	if in.Email == "" {
		errs = append(errs, controllers.FieldError{Field: "email", Message: "email is required"})
	} else if _, err := mail.ParseAddress(in.Email); err != nil {
		errs = append(errs, controllers.FieldError{Field: "email", Message: "email must be a valid email address"})
	}

	if len(in.Password) < 8 {
		errs = append(errs, controllers.FieldError{Field: "password", Message: "password must be at least 8 characters"})
	} else if len(in.Password) > 72 {
		errs = append(errs, controllers.FieldError{Field: "password", Message: "password must not exceed 72 characters"})
	}

	if in.FullName == "" {
		errs = append(errs, controllers.FieldError{Field: "full_name", Message: "full_name is required"})
	} else if len(in.FullName) > 120 {
		errs = append(errs, controllers.FieldError{Field: "full_name", Message: "full_name must not exceed 120 characters"})
	}

	if len(errs) > 0 {
		return dtos.RegisterUserInput{}, controllers.NewValidationError("VALIDATION_ERROR", "Invalid registration payload", errs...)
	}
	return in, nil
}

func ValidateLoginInput(c *gin.Context) (dtos.LoginInput, *controllers.ValidationError) {
	var in dtos.LoginInput
	if err := c.ShouldBindJSON(&in); err != nil {
		return dtos.LoginInput{}, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
	}

	var errs []controllers.FieldError
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))

	if in.Email == "" {
		errs = append(errs, controllers.FieldError{Field: "email", Message: "email is required"})
	} else if _, err := mail.ParseAddress(in.Email); err != nil {
		errs = append(errs, controllers.FieldError{Field: "email", Message: "email must be a valid email address"})
	}

	if in.Password == "" {
		errs = append(errs, controllers.FieldError{Field: "password", Message: "password is required"})
	}

	if len(errs) > 0 {
		return dtos.LoginInput{}, controllers.NewValidationError("VALIDATION_ERROR", "Invalid login payload", errs...)
	}
	return in, nil
}

func ValidateRefreshTokenInput(c *gin.Context) (dtos.RefreshTokenInput, *controllers.ValidationError) {
	var in dtos.RefreshTokenInput
	if err := c.ShouldBindJSON(&in); err != nil {
		return dtos.RefreshTokenInput{}, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
	}

	in.RefreshToken = strings.TrimSpace(in.RefreshToken)
	if in.RefreshToken == "" {
		return dtos.RefreshTokenInput{}, controllers.NewValidationError(
			"VALIDATION_ERROR",
			"Invalid refresh token payload",
			controllers.FieldError{Field: "refresh_token", Message: "refresh_token is required"},
		)
	}
	return in, nil
}

func ValidateUpdateProfileInput(c *gin.Context) (dtos.UpdateProfileInput, *controllers.ValidationError) {
	var in dtos.UpdateProfileInput
	if err := c.ShouldBindJSON(&in); err != nil {
		return dtos.UpdateProfileInput{}, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
	}

	var errs []controllers.FieldError
	if in.FullName != nil {
		trimmed := strings.TrimSpace(*in.FullName)
		if trimmed == "" {
			errs = append(errs, controllers.FieldError{Field: "full_name", Message: "full_name cannot be empty"})
		} else if len(trimmed) > 120 {
			errs = append(errs, controllers.FieldError{Field: "full_name", Message: "full_name must not exceed 120 characters"})
		} else {
			in.FullName = &trimmed
		}
	}

	if in.FullName == nil && in.AvatarMediaID == nil {
		errs = append(errs, controllers.FieldError{Field: "body", Message: "at least one of full_name or avatar_media_id must be provided"})
	}

	if len(errs) > 0 {
		return dtos.UpdateProfileInput{}, controllers.NewValidationError("VALIDATION_ERROR", "Invalid profile update payload", errs...)
	}
	return in, nil
}
