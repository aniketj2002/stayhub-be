package controllers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationError struct {
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Fields  []FieldError `json:"fields,omitempty"`
}

func (e *ValidationError) Error() string {
	return e.Message
}

func NewValidationError(code, message string, fields ...FieldError) *ValidationError {
	return &ValidationError{
		Code:    code,
		Message: message,
		Fields:  fields,
	}
}

func ParseOptionalUUID(raw, fieldName string, errs *[]FieldError) *uuid.UUID {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	parsed, err := uuid.Parse(trimmed)
	if err != nil {
		*errs = append(*errs, FieldError{
			Field:   fieldName,
			Message: fmt.Sprintf("%s must be a valid UUID", fieldName),
		})
		return nil
	}
	return &parsed
}

func ParsePagination(rawLimit, rawOffset string, defaultLimit, maxLimit int, errs *[]FieldError) (int, int) {
	limit := defaultLimit
	if trimmed := strings.TrimSpace(rawLimit); trimmed != "" {
		val, err := strconv.Atoi(trimmed)
		if err != nil || val <= 0 {
			*errs = append(*errs, FieldError{
				Field:   "limit",
				Message: "limit must be a positive integer",
			})
		} else if val > maxLimit {
			limit = maxLimit
		} else {
			limit = val
		}
	}

	offset := 0
	if trimmed := strings.TrimSpace(rawOffset); trimmed != "" {
		val, err := strconv.Atoi(trimmed)
		if err != nil || val < 0 {
			*errs = append(*errs, FieldError{
				Field:   "offset",
				Message: "offset must be a non-negative integer",
			})
		} else {
			offset = val
		}
	}

	return limit, offset
}

func ParseDateRange(rawStart, rawEnd string) (time.Time, time.Time, []FieldError) {
	var errs []FieldError
	var startDate, endDate time.Time

	if strings.TrimSpace(rawStart) == "" {
		errs = append(errs, FieldError{Field: "start_date", Message: "start_date is required (YYYY-MM-DD)"})
	} else {
		p, err := time.Parse("2006-01-02", strings.TrimSpace(rawStart))
		if err != nil {
			errs = append(errs, FieldError{Field: "start_date", Message: "start_date must be in YYYY-MM-DD format"})
		} else {
			startDate = p
		}
	}

	if strings.TrimSpace(rawEnd) == "" {
		errs = append(errs, FieldError{Field: "end_date", Message: "end_date is required (YYYY-MM-DD)"})
	} else {
		p, err := time.Parse("2006-01-02", strings.TrimSpace(rawEnd))
		if err != nil {
			errs = append(errs, FieldError{Field: "end_date", Message: "end_date must be in YYYY-MM-DD format"})
		} else {
			endDate = p
		}
	}

	if len(errs) == 0 {
		if !endDate.After(startDate) {
			errs = append(errs, FieldError{Field: "end_date", Message: "end_date must be after start_date"})
		} else if endDate.Sub(startDate) > 366*24*time.Hour {
			errs = append(errs, FieldError{Field: "end_date", Message: "date range cannot exceed 366 days"})
		}
	}
	return startDate, endDate, errs
}

func IsValidHHMM(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	_, err := time.Parse("15:04", trimmed)
	return err == nil
}
