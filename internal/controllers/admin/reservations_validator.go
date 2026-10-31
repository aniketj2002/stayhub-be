package admin

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/controllers"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
)

func ValidateAdminReservationFilter(c *gin.Context) (dtos.AdminReservationFilter, *controllers.ValidationError) {
	var fieldErrs []controllers.FieldError
	var filter dtos.AdminReservationFilter

	filter.Code = strings.TrimSpace(c.Query("code"))
	filter.GuestID = controllers.ParseOptionalUUID(c.Query("guest"), "guest", &fieldErrs)
	filter.HostID = controllers.ParseOptionalUUID(c.Query("host"), "host", &fieldErrs)
	filter.ListingID = controllers.ParseOptionalUUID(c.Query("listing"), "listing", &fieldErrs)

	if rawStatus := strings.TrimSpace(c.Query("status")); rawStatus != "" {
		st := enums.ReservationStatus(rawStatus)
		filter.Status = &st
	}

	if rawFrom := strings.TrimSpace(c.Query("from")); rawFrom != "" {
		t, err := time.Parse("2006-01-02", rawFrom)
		if err != nil {
			t, err = time.Parse(time.RFC3339, rawFrom)
		}
		if err != nil {
			fieldErrs = append(fieldErrs, controllers.FieldError{
				Field:   "from",
				Message: "from must be YYYY-MM-DD or RFC3339 timestamp",
			})
		} else {
			filter.From = &t
		}
	}

	if rawTo := strings.TrimSpace(c.Query("to")); rawTo != "" {
		t, err := time.Parse("2006-01-02", rawTo)
		if err != nil {
			t, err = time.Parse(time.RFC3339, rawTo)
		}
		if err != nil {
			fieldErrs = append(fieldErrs, controllers.FieldError{
				Field:   "to",
				Message: "to must be YYYY-MM-DD or RFC3339 timestamp",
			})
		} else {
			filter.To = &t
		}
	}

	limit, offset := controllers.ParsePagination(c.Query("limit"), c.Query("offset"), 20, 100, &fieldErrs)
	filter.Limit = limit
	filter.Offset = offset

	if len(fieldErrs) > 0 {
		return filter, controllers.NewValidationError("INVALID_QUERY_PARAMS", "Invalid reservation query parameters", fieldErrs...)
	}
	return filter, nil
}

func ValidateAdminCancelReservation(c *gin.Context) (dtos.AdminCancelReservationInput, *controllers.ValidationError) {
	var input dtos.AdminCancelReservationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		return input, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
	}

	var fieldErrs []controllers.FieldError
	input.Reason = strings.TrimSpace(input.Reason)
	if input.Reason == "" {
		fieldErrs = append(fieldErrs, controllers.FieldError{
			Field:   "reason",
			Message: "Reason is required for admin actions",
		})
	}

	input.RefundMode = strings.ToLower(strings.TrimSpace(input.RefundMode))
	if input.RefundMode != "policy" && input.RefundMode != "full" && input.RefundMode != "custom" {
		fieldErrs = append(fieldErrs, controllers.FieldError{
			Field:   "refund_mode",
			Message: "refund_mode must be 'policy', 'full', or 'custom'",
		})
	}

	if input.RefundMode == "custom" {
		if input.AmountMinor == nil || *input.AmountMinor <= 0 {
			fieldErrs = append(fieldErrs, controllers.FieldError{
				Field:   "amount_minor",
				Message: "amount_minor is required and must be greater than zero when refund_mode is 'custom'",
			})
		}
	}

	if len(fieldErrs) > 0 {
		return input, controllers.NewValidationError("VALIDATION_ERROR", "Validation failed", fieldErrs...)
	}
	return input, nil
}

func ValidateAdminRefundReservation(c *gin.Context) (dtos.AdminRefundReservationInput, *controllers.ValidationError) {
	var input dtos.AdminRefundReservationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		return input, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
	}

	var fieldErrs []controllers.FieldError
	input.Reason = strings.TrimSpace(input.Reason)
	if input.Reason == "" {
		fieldErrs = append(fieldErrs, controllers.FieldError{
			Field:   "reason",
			Message: "Reason is required for admin refund",
		})
	}

	if input.AmountMinor <= 0 {
		fieldErrs = append(fieldErrs, controllers.FieldError{
			Field:   "amount_minor",
			Message: "amount_minor must be greater than zero",
		})
	}

	if len(fieldErrs) > 0 {
		return input, controllers.NewValidationError("VALIDATION_ERROR", "Validation failed", fieldErrs...)
	}
	return input, nil
}

func ParseUUIDParam(c *gin.Context, paramName string) (uuid.UUID, *controllers.ValidationError) {
	raw := strings.TrimSpace(c.Param(paramName))
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, controllers.NewValidationError(
			"INVALID_PATH_PARAM",
			fmt.Sprintf("%s must be a valid UUID", paramName),
			controllers.FieldError{Field: paramName, Message: "Invalid UUID format"},
		)
	}
	return id, nil
}
