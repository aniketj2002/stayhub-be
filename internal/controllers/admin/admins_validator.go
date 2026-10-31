package admin

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/controllers"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
)

func ValidateCreateAdmin(c *gin.Context) (dtos.CreateAdminInput, *controllers.ValidationError) {
	var input dtos.CreateAdminInput
	if err := c.ShouldBindJSON(&input); err != nil {
		return input, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
	}

	var fieldErrs []controllers.FieldError
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if input.Email == "" || !strings.Contains(input.Email, "@") {
		fieldErrs = append(fieldErrs, controllers.FieldError{
			Field:   "email",
			Message: "Valid email address is required",
		})
	}
	if len(input.Password) < 8 {
		fieldErrs = append(fieldErrs, controllers.FieldError{
			Field:   "password",
			Message: "Password must be at least 8 characters",
		})
	}
	input.FullName = strings.TrimSpace(input.FullName)
	if input.FullName == "" {
		fieldErrs = append(fieldErrs, controllers.FieldError{
			Field:   "full_name",
			Message: "Full name is required",
		})
	}

	if input.Role != "" {
		switch input.Role {
		case enums.AdminRoleSupportL1, enums.AdminRoleSupportL2, enums.AdminRoleTrustSafety,
			enums.AdminRoleFinance, enums.AdminRoleCatalogManager, enums.AdminRoleSuperAdmin:
		default:
			fieldErrs = append(fieldErrs, controllers.FieldError{
				Field:   "role",
				Message: "Invalid admin role",
			})
		}
	} else {
		input.Role = enums.AdminRoleSupportL1
	}

	if len(fieldErrs) > 0 {
		return input, controllers.NewValidationError("VALIDATION_ERROR", "Validation failed", fieldErrs...)
	}
	return input, nil
}

func ValidateUpdateAdmin(c *gin.Context) (dtos.UpdateAdminInput, *controllers.ValidationError) {
	var input dtos.UpdateAdminInput
	if err := c.ShouldBindJSON(&input); err != nil {
		return input, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
	}

	var fieldErrs []controllers.FieldError
	if input.Role != nil {
		switch *input.Role {
		case enums.AdminRoleSupportL1, enums.AdminRoleSupportL2, enums.AdminRoleTrustSafety,
			enums.AdminRoleFinance, enums.AdminRoleCatalogManager, enums.AdminRoleSuperAdmin:
		default:
			fieldErrs = append(fieldErrs, controllers.FieldError{
				Field:   "role",
				Message: "Invalid admin role",
			})
		}
	}

	if len(fieldErrs) > 0 {
		return input, controllers.NewValidationError("VALIDATION_ERROR", "Validation failed", fieldErrs...)
	}
	return input, nil
}
