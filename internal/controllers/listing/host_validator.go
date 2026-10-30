package listing

import (
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/controllers"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/platform/clock"
)

func ValidateCreateListingInput(c *gin.Context) (dtos.CreateListingInput, *controllers.ValidationError) {
	var in dtos.CreateListingInput
	if c.Request.Body != nil {
		if err := c.ShouldBindJSON(&in); err != nil && err != io.EOF {
			return dtos.CreateListingInput{}, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
		}
	}

	var errs []controllers.FieldError
	if in.Timezone != "" {
		if _, err := time.LoadLocation(strings.TrimSpace(in.Timezone)); err != nil {
			errs = append(errs, controllers.FieldError{Field: "timezone", Message: "timezone must be a valid IANA timezone (e.g., Europe/Lisbon)"})
		}
	}
	if in.Currency != "" && len(strings.TrimSpace(in.Currency)) != 3 {
		errs = append(errs, controllers.FieldError{Field: "currency", Message: "currency must be a 3-letter ISO code (e.g., EUR)"})
	}
	if len(errs) > 0 {
		return dtos.CreateListingInput{}, controllers.NewValidationError("VALIDATION_ERROR", "Invalid create listing payload", errs...)
	}
	return in, nil
}

func ValidateUpdateWorkingVersionInput(c *gin.Context) (dtos.UpdateWorkingVersionInput, *controllers.ValidationError) {
	var in dtos.UpdateWorkingVersionInput
	if err := c.ShouldBindJSON(&in); err != nil {
		return dtos.UpdateWorkingVersionInput{}, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
	}

	var errs []controllers.FieldError
	if in.WizardStep != nil && (*in.WizardStep < 1 || *in.WizardStep > 10) {
		errs = append(errs, controllers.FieldError{Field: "wizard_step", Message: "wizard_step must be between 1 and 10"})
	}
	if in.Country != nil && strings.TrimSpace(*in.Country) != "" && len(strings.TrimSpace(*in.Country)) != 2 {
		errs = append(errs, controllers.FieldError{Field: "country", Message: "country must be a 2-letter ISO code (e.g., PT, IN, JP)"})
	}
	if in.Timezone != nil && strings.TrimSpace(*in.Timezone) != "" {
		if _, err := time.LoadLocation(strings.TrimSpace(*in.Timezone)); err != nil {
			errs = append(errs, controllers.FieldError{Field: "timezone", Message: "timezone must be a valid IANA timezone"})
		}
	}
	if in.MaxGuests != nil && *in.MaxGuests <= 0 {
		errs = append(errs, controllers.FieldError{Field: "max_guests", Message: "max_guests must be a positive integer"})
	}
	if in.BasePriceMinor != nil && *in.BasePriceMinor < 0 {
		errs = append(errs, controllers.FieldError{Field: "base_price_minor", Message: "base_price_minor must be non-negative"})
	}
	if in.CleaningFeeMinor != nil && *in.CleaningFeeMinor < 0 {
		errs = append(errs, controllers.FieldError{Field: "cleaning_fee_minor", Message: "cleaning_fee_minor must be non-negative"})
	}
	if in.Currency != nil && strings.TrimSpace(*in.Currency) != "" && len(strings.TrimSpace(*in.Currency)) != 3 {
		errs = append(errs, controllers.FieldError{Field: "currency", Message: "currency must be a 3-letter ISO code"})
	}
	if in.CheckInTime != nil && !controllers.IsValidHHMM(*in.CheckInTime) {
		errs = append(errs, controllers.FieldError{Field: "check_in_time", Message: "check_in_time must be in HH:MM 24-hour format"})
	}
	if in.CheckOutTime != nil && !controllers.IsValidHHMM(*in.CheckOutTime) {
		errs = append(errs, controllers.FieldError{Field: "check_out_time", Message: "check_out_time must be in HH:MM 24-hour format"})
	}

	if len(errs) > 0 {
		return dtos.UpdateWorkingVersionInput{}, controllers.NewValidationError("VALIDATION_ERROR", "Invalid working version update payload", errs...)
	}
	return in, nil
}

func ValidateUpdateHostConfigInput(c *gin.Context) (dtos.UpdateHostConfigInput, *controllers.ValidationError) {
	var in dtos.UpdateHostConfigInput
	if err := c.ShouldBindJSON(&in); err != nil {
		return dtos.UpdateHostConfigInput{}, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
	}

	if len(in.Config) == 0 {
		return dtos.UpdateHostConfigInput{}, controllers.NewValidationError(
			"VALIDATION_ERROR",
			"config is required",
			controllers.FieldError{Field: "config", Message: "config must be a non-empty JSON object"},
		)
	}
	var obj map[string]any
	if err := json.Unmarshal(in.Config, &obj); err != nil {
		return dtos.UpdateHostConfigInput{}, controllers.NewValidationError(
			"VALIDATION_ERROR",
			"Invalid config JSON object",
			controllers.FieldError{Field: "config", Message: "config must be a JSON object"},
		)
	}
	return in, nil
}

func ValidateCalendarQueryInput(c *gin.Context, clk clock.Clock) (dtos.CalendarQueryInput, *controllers.ValidationError) {
	var errs []controllers.FieldError
	now := clk.Now().UTC()
	defaultStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	defaultEnd := defaultStart.AddDate(0, 3, 0)

	startDate := defaultStart
	if rawStart := strings.TrimSpace(c.Query("start_date")); rawStart != "" {
		parsed, err := time.Parse("2006-01-02", rawStart)
		if err != nil {
			errs = append(errs, controllers.FieldError{Field: "start_date", Message: "start_date must be in YYYY-MM-DD format"})
		} else {
			startDate = parsed
		}
	}

	endDate := defaultEnd
	if rawEnd := strings.TrimSpace(c.Query("end_date")); rawEnd != "" {
		parsed, err := time.Parse("2006-01-02", rawEnd)
		if err != nil {
			errs = append(errs, controllers.FieldError{Field: "end_date", Message: "end_date must be in YYYY-MM-DD format"})
		} else {
			endDate = parsed
		}
	} else if strings.TrimSpace(c.Query("start_date")) != "" {
		endDate = startDate.AddDate(0, 3, 0)
	}

	if len(errs) == 0 {
		if !endDate.After(startDate) {
			errs = append(errs, controllers.FieldError{Field: "end_date", Message: "end_date must be after start_date"})
		} else if endDate.Sub(startDate) > 366*24*time.Hour {
			errs = append(errs, controllers.FieldError{Field: "end_date", Message: "date range cannot exceed 366 days"})
		}
	}

	if len(errs) > 0 {
		return dtos.CalendarQueryInput{}, controllers.NewValidationError("VALIDATION_ERROR", "Invalid calendar query parameters", errs...)
	}
	return dtos.CalendarQueryInput{StartDate: startDate, EndDate: endDate}, nil
}

type rawCalendarRangeBody struct {
	StartDate   string  `json:"start_date"`
	EndDate     string  `json:"end_date"`
	BlockReason *string `json:"block_reason,omitempty"`
}

func ValidateCalendarBlockInput(c *gin.Context) (dtos.CalendarBlockInput, *controllers.ValidationError) {
	var body rawCalendarRangeBody
	if err := c.ShouldBindJSON(&body); err != nil {
		return dtos.CalendarBlockInput{}, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
	}

	startDate, endDate, errs := controllers.ParseDateRange(body.StartDate, body.EndDate)
	if len(errs) > 0 {
		return dtos.CalendarBlockInput{}, controllers.NewValidationError("VALIDATION_ERROR", "Invalid calendar block payload", errs...)
	}

	return dtos.CalendarBlockInput{
		StartDate:   startDate,
		EndDate:     endDate,
		BlockReason: body.BlockReason,
	}, nil
}

func ValidateCalendarUnblockInput(c *gin.Context) (dtos.CalendarUnblockInput, *controllers.ValidationError) {
	var body rawCalendarRangeBody
	if c.Request.Body != nil {
		_ = c.ShouldBindJSON(&body)
	}
	if body.StartDate == "" {
		body.StartDate = c.Query("start_date")
	}
	if body.EndDate == "" {
		body.EndDate = c.Query("end_date")
	}

	startDate, endDate, errs := controllers.ParseDateRange(body.StartDate, body.EndDate)
	if len(errs) > 0 {
		return dtos.CalendarUnblockInput{}, controllers.NewValidationError("VALIDATION_ERROR", "Invalid calendar unblock payload", errs...)
	}

	return dtos.CalendarUnblockInput{
		StartDate: startDate,
		EndDate:   endDate,
	}, nil
}
