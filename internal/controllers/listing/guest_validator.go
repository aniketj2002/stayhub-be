package listing

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/controllers"
	"github.com/stayhub/backend/internal/domain/dtos"
)

func ValidateSearchListingsFilter(c *gin.Context) (dtos.SearchListingsFilter, *controllers.ValidationError) {
	var errs []controllers.FieldError

	city := strings.TrimSpace(c.Query("city"))
	if city == "" {
		city = strings.TrimSpace(c.Query("destination"))
	}
	country := strings.ToUpper(strings.TrimSpace(c.Query("country")))
	if country != "" && len(country) != 2 {
		errs = append(errs, controllers.FieldError{Field: "country", Message: "country must be a 2-letter ISO code"})
	}

	var checkIn, checkOut *time.Time
	rawCheckIn := strings.TrimSpace(c.Query("check_in"))
	rawCheckOut := strings.TrimSpace(c.Query("check_out"))

	if rawCheckIn != "" || rawCheckOut != "" {
		if rawCheckIn == "" {
			errs = append(errs, controllers.FieldError{Field: "check_in", Message: "check_in is required when check_out is provided"})
		} else {
			p, err := time.Parse("2006-01-02", rawCheckIn)
			if err != nil {
				errs = append(errs, controllers.FieldError{Field: "check_in", Message: "check_in must be in YYYY-MM-DD format"})
			} else {
				checkIn = &p
			}
		}

		if rawCheckOut == "" {
			errs = append(errs, controllers.FieldError{Field: "check_out", Message: "check_out is required when check_in is provided"})
		} else {
			p, err := time.Parse("2006-01-02", rawCheckOut)
			if err != nil {
				errs = append(errs, controllers.FieldError{Field: "check_out", Message: "check_out must be in YYYY-MM-DD format"})
			} else {
				checkOut = &p
			}
		}

		if checkIn != nil && checkOut != nil && !checkOut.After(*checkIn) {
			errs = append(errs, controllers.FieldError{Field: "check_out", Message: "check_out must be after check_in"})
		}
	}

	guests := 0
	if rawGuests := strings.TrimSpace(c.Query("guests")); rawGuests != "" {
		val, err := strconv.Atoi(rawGuests)
		if err != nil || val <= 0 {
			errs = append(errs, controllers.FieldError{Field: "guests", Message: "guests must be a positive integer"})
		} else {
			guests = val
		}
	}

	var instantBook *bool
	if rawIB := strings.TrimSpace(c.Query("instant_book")); rawIB != "" {
		val, err := strconv.ParseBool(rawIB)
		if err != nil {
			errs = append(errs, controllers.FieldError{Field: "instant_book", Message: "instant_book must be true or false"})
		} else {
			instantBook = &val
		}
	}

	var minPriceMinor *int64
	if rawMin := strings.TrimSpace(c.Query("min_price_minor")); rawMin != "" {
		val, err := strconv.ParseInt(rawMin, 10, 64)
		if err != nil || val < 0 {
			errs = append(errs, controllers.FieldError{Field: "min_price_minor", Message: "min_price_minor must be a non-negative integer"})
		} else {
			minPriceMinor = &val
		}
	}

	var maxPriceMinor *int64
	if rawMax := strings.TrimSpace(c.Query("max_price_minor")); rawMax != "" {
		val, err := strconv.ParseInt(rawMax, 10, 64)
		if err != nil || val < 0 {
			errs = append(errs, controllers.FieldError{Field: "max_price_minor", Message: "max_price_minor must be a non-negative integer"})
		} else {
			maxPriceMinor = &val
		}
	}

	var amenities []string
	if rawAmenities := strings.TrimSpace(c.Query("amenities")); rawAmenities != "" {
		for _, part := range strings.Split(rawAmenities, ",") {
			if code := strings.TrimSpace(part); code != "" {
				amenities = append(amenities, code)
			}
		}
	}

	limit, offset := controllers.ParsePagination(c.Query("limit"), c.Query("offset"), 50, 100, &errs)

	if len(errs) > 0 {
		return dtos.SearchListingsFilter{}, controllers.NewValidationError("VALIDATION_ERROR", "Invalid search filter parameters", errs...)
	}

	return dtos.SearchListingsFilter{
		City:          city,
		Country:       country,
		CheckIn:       checkIn,
		CheckOut:      checkOut,
		Guests:        guests,
		PropertyType:  strings.TrimSpace(c.Query("property_type")),
		InstantBook:   instantBook,
		MinPriceMinor: minPriceMinor,
		MaxPriceMinor: maxPriceMinor,
		Amenities:     amenities,
		Limit:         limit,
		Offset:        offset,
	}, nil
}

type rawQuoteRequest struct {
	CheckIn  string `json:"check_in"`
	CheckOut string `json:"check_out"`
	Guests   int    `json:"guests"`
}

func ValidateQuoteRequestInput(c *gin.Context) (dtos.QuoteRequestInput, *controllers.ValidationError) {
	var body rawQuoteRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		return dtos.QuoteRequestInput{}, controllers.NewValidationError("INVALID_JSON", "Request body must be valid JSON")
	}

	checkIn, checkOut, errs := controllers.ParseDateRange(body.CheckIn, body.CheckOut)
	if body.Guests <= 0 {
		errs = append(errs, controllers.FieldError{Field: "guests", Message: "guests must be at least 1"})
	}

	if len(errs) > 0 {
		return dtos.QuoteRequestInput{}, controllers.NewValidationError("VALIDATION_ERROR", "Invalid quote payload", errs...)
	}

	return dtos.QuoteRequestInput{
		CheckIn:  checkIn,
		CheckOut: checkOut,
		Guests:   body.Guests,
	}, nil
}
