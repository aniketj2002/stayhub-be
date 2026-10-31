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
	} else if rawMinMajor := strings.TrimSpace(c.Query("min_price")); rawMinMajor != "" {
		if val, err := strconv.ParseInt(rawMinMajor, 10, 64); err == nil && val >= 0 {
			vMinor := val * 100
			minPriceMinor = &vMinor
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
	} else if rawMaxMajor := strings.TrimSpace(c.Query("max_price")); rawMaxMajor != "" {
		if val, err := strconv.ParseInt(rawMaxMajor, 10, 64); err == nil && val >= 0 {
			vMinor := val * 100
			maxPriceMinor = &vMinor
		}
	}

	var bedrooms *int
	if rawBedrooms := strings.TrimSpace(c.Query("bedrooms")); rawBedrooms != "" {
		if val, err := strconv.Atoi(rawBedrooms); err == nil && val >= 0 {
			bedrooms = &val
		}
	}

	var beds *int
	if rawBeds := strings.TrimSpace(c.Query("beds")); rawBeds != "" {
		if val, err := strconv.Atoi(rawBeds); err == nil && val >= 0 {
			beds = &val
		}
	}

	var bathrooms *float64
	if rawBathrooms := strings.TrimSpace(c.Query("bathrooms")); rawBathrooms != "" {
		if val, err := strconv.ParseFloat(rawBathrooms, 64); err == nil && val >= 0 {
			bathrooms = &val
		}
	}

	var guestFavorite *bool
	if rawGF := strings.TrimSpace(c.Query("guest_favorite")); rawGF != "" {
		if val, err := strconv.ParseBool(rawGF); err == nil {
			guestFavorite = &val
		}
	}

	var superhost *bool
	if rawSH := strings.TrimSpace(c.Query("superhost")); rawSH != "" {
		if val, err := strconv.ParseBool(rawSH); err == nil {
			superhost = &val
		}
	}

	var propertyTypes []string
	if rawPTs := c.QueryArray("property_type"); len(rawPTs) > 0 {
		for _, pt := range rawPTs {
			for _, part := range strings.Split(pt, ",") {
				if t := strings.TrimSpace(part); t != "" && t != "any" {
					propertyTypes = append(propertyTypes, t)
				}
			}
		}
	} else if rawPT := strings.TrimSpace(c.Query("property_types")); rawPT != "" {
		for _, part := range strings.Split(rawPT, ",") {
			if t := strings.TrimSpace(part); t != "" && t != "any" {
				propertyTypes = append(propertyTypes, t)
			}
		}
	}

	var neLat, neLng, swLat, swLng *float64
	if rawNeLat := strings.TrimSpace(c.Query("ne_lat")); rawNeLat != "" {
		if val, err := strconv.ParseFloat(rawNeLat, 64); err == nil {
			neLat = &val
		}
	}
	if rawNeLng := strings.TrimSpace(c.Query("ne_lng")); rawNeLng != "" {
		if val, err := strconv.ParseFloat(rawNeLng, 64); err == nil {
			neLng = &val
		}
	}
	if rawSwLat := strings.TrimSpace(c.Query("sw_lat")); rawSwLat != "" {
		if val, err := strconv.ParseFloat(rawSwLat, 64); err == nil {
			swLat = &val
		}
	}
	if rawSwLng := strings.TrimSpace(c.Query("sw_lng")); rawSwLng != "" {
		if val, err := strconv.ParseFloat(rawSwLng, 64); err == nil {
			swLng = &val
		}
	}
	if (neLat == nil || neLng == nil || swLat == nil || swLng == nil) && strings.TrimSpace(c.Query("bbox")) != "" {
		parts := strings.Split(strings.TrimSpace(c.Query("bbox")), ",")
		if len(parts) == 4 {
			// standard bbox format: sw_lng, sw_lat, ne_lng, ne_lat
			if sLng, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64); err1 == nil {
				if sLat, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err2 == nil {
					if nLng, err3 := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64); err3 == nil {
						if nLat, err4 := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64); err4 == nil {
							swLng = &sLng
							swLat = &sLat
							neLng = &nLng
							neLat = &nLat
						}
					}
				}
			}
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
		PropertyTypes: propertyTypes,
		InstantBook:   instantBook,
		MinPriceMinor: minPriceMinor,
		MaxPriceMinor: maxPriceMinor,
		Amenities:     amenities,
		Bedrooms:      bedrooms,
		Beds:          beds,
		Bathrooms:     bathrooms,
		GuestFavorite: guestFavorite,
		Superhost:     superhost,
		NeLat:         neLat,
		NeLng:         neLng,
		SwLat:         swLat,
		SwLng:         swLng,
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
