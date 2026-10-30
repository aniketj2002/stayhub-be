package reservation

import (
	"io"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/controllers"
	"github.com/stayhub/backend/internal/domain/dtos"
)

type rawCreateReservationBody struct {
	ListingID          string  `json:"listing_id"`
	CheckIn            string  `json:"check_in"`
	CheckOut           string  `json:"check_out"`
	Guests             int     `json:"guests"`
	GuestMessage       *string `json:"guest_message,omitempty"`
	PaymentMethodToken string  `json:"payment_method_token"`
}

func ValidateCreateReservationInput(c *gin.Context) (dtos.CreateReservationInput, *controllers.ValidationError) {
	var body rawCreateReservationBody
	if err := c.ShouldBindJSON(&body); err != nil {
		return dtos.CreateReservationInput{}, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
	}

	var errs []controllers.FieldError

	listingID, err := uuid.Parse(strings.TrimSpace(body.ListingID))
	if err != nil {
		errs = append(errs, controllers.FieldError{Field: "listing_id", Message: "listing_id must be a valid UUID"})
	}

	checkIn, checkOut, dateErrs := controllers.ParseDateRange(body.CheckIn, body.CheckOut)
	for _, de := range dateErrs {
		field := de.Field
		if field == "start_date" {
			field = "check_in"
		} else if field == "end_date" {
			field = "check_out"
		}
		errs = append(errs, controllers.FieldError{Field: field, Message: strings.ReplaceAll(strings.ReplaceAll(de.Message, "start_date", "check_in"), "end_date", "check_out")})
	}

	todayUTC := time.Now().UTC().Truncate(24 * time.Hour)
	if len(dateErrs) == 0 && checkIn.Before(todayUTC) {
		errs = append(errs, controllers.FieldError{Field: "check_in", Message: "check_in date cannot be in the past"})
	}

	if body.Guests <= 0 {
		errs = append(errs, controllers.FieldError{Field: "guests", Message: "guests must be at least 1"})
	}

	token := strings.TrimSpace(body.PaymentMethodToken)
	if token == "" {
		token = "tok_simulated_default"
	}

	if len(errs) > 0 {
		return dtos.CreateReservationInput{}, controllers.NewValidationError("VALIDATION_ERROR", "Invalid create reservation payload", errs...)
	}

	return dtos.CreateReservationInput{
		ListingID:          listingID,
		CheckIn:            checkIn,
		CheckOut:           checkOut,
		Guests:             body.Guests,
		GuestMessage:       body.GuestMessage,
		PaymentMethodToken: token,
	}, nil
}

func ValidateDeclineReservationInput(c *gin.Context) (dtos.DeclineReservationInput, *controllers.ValidationError) {
	var in dtos.DeclineReservationInput
	if err := c.ShouldBindJSON(&in); err != nil {
		return dtos.DeclineReservationInput{}, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
	}

	in.DeclineReason = strings.TrimSpace(in.DeclineReason)
	if in.DeclineReason == "" {
		return dtos.DeclineReservationInput{}, controllers.NewValidationError(
			"VALIDATION_ERROR",
			"decline_reason is required",
			controllers.FieldError{Field: "decline_reason", Message: "decline_reason cannot be empty"},
		)
	}
	return in, nil
}

func ValidateCancelReservationInput(c *gin.Context) (dtos.CancelReservationInput, *controllers.ValidationError) {
	var in dtos.CancelReservationInput
	if c.Request.Body != nil {
		if err := c.ShouldBindJSON(&in); err != nil && err != io.EOF {
			return dtos.CancelReservationInput{}, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
		}
	}
	if in.CancelReason != nil {
		trimmed := strings.TrimSpace(*in.CancelReason)
		if trimmed == "" {
			in.CancelReason = nil
		} else {
			in.CancelReason = &trimmed
		}
	}
	return in, nil
}
