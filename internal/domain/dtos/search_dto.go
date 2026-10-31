package dtos

import (
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/models"
)

type SearchListingsFilter struct {
	City          string
	Country       string
	CheckIn       *time.Time
	CheckOut      *time.Time
	Guests        int
	PropertyType  string
	PropertyTypes []string
	InstantBook   *bool
	MinPriceMinor *int64
	MaxPriceMinor *int64
	Amenities     []string
	Bedrooms      *int
	Beds          *int
	Bathrooms     *float64
	GuestFavorite *bool
	Superhost     *bool
	NeLat         *float64
	NeLng         *float64
	SwLat         *float64
	SwLng         *float64
	Limit         int
	Offset        int
}

type SearchListingRow struct {
	Listing         models.Listing
	LiveVersion     models.ListingVersion
	HostIsSuperhost bool
}

type SearchListingCard struct {
	ID                uuid.UUID `json:"id"`
	HostID            uuid.UUID `json:"host_id"`
	LiveVersionID     uuid.UUID `json:"live_version_id"`
	PropertyType      string    `json:"property_type"`
	Title             string    `json:"title"`
	Description       *string   `json:"description,omitempty"`
	City              string    `json:"city"`
	Region            *string   `json:"region,omitempty"`
	Country           string    `json:"country"`
	Latitude          *float64  `json:"latitude,omitempty"`
	Longitude         *float64  `json:"longitude,omitempty"`
	Lat               *float64  `json:"lat,omitempty"`
	Lng               *float64  `json:"lng,omitempty"`
	MaxGuests         int       `json:"max_guests"`
	Bedrooms          *int      `json:"bedrooms,omitempty"`
	Beds              *int      `json:"beds,omitempty"`
	Bathrooms         *float64  `json:"bathrooms,omitempty"`
	InstantBook       bool      `json:"instant_book"`
	Currency          string    `json:"currency"`
	NightlyPriceMinor int64     `json:"nightly_price_minor"`
	CleaningFeeMinor  int64     `json:"cleaning_fee_minor"`
	CoverPhotoURL     *string   `json:"cover_photo_url"`
	PhotoURLs         []string  `json:"photo_urls"`
	Rating            *float64  `json:"rating"`
	ReviewCount       int       `json:"review_count"`
	IsGuestFavorite   bool      `json:"is_guest_favorite"`
	IsSuperhost       bool      `json:"is_superhost"`
	Nights            *int      `json:"nights,omitempty"`
	NightlyTotalMinor *int64    `json:"nightly_total_minor,omitempty"`
	GuestFeeMinor     *int64    `json:"guest_fee_minor,omitempty"`
	StayTotalMinor    *int64    `json:"stay_total_minor,omitempty"`
}

type PublicHostProfile struct {
	ID          uuid.UUID `json:"id"`
	FullName    string    `json:"full_name"`
	AvatarURL   *string   `json:"avatar_url,omitempty"`
	JoinedAt    time.Time `json:"joined_at"`
	IsSuperhost bool      `json:"is_superhost"`
	Bio         *string   `json:"bio,omitempty"`
}

type PublicListingDetailResponse struct {
	ID                 uuid.UUID                 `json:"id"`
	Status             enums.ListingStatus       `json:"status"`
	LiveVersionID      uuid.UUID                 `json:"live_version_id"`
	VersionNo          int                       `json:"version_no"`
	PropertyType       string                    `json:"property_type"`
	Title              string                    `json:"title"`
	Description        string                    `json:"description"`
	City               string                    `json:"city"`
	Region             *string                   `json:"region,omitempty"`
	Country            string                    `json:"country"`
	Latitude           *float64                  `json:"latitude,omitempty"`
	Longitude          *float64                  `json:"longitude,omitempty"`
	Timezone           string                    `json:"timezone"`
	MaxGuests          int                       `json:"max_guests"`
	Bedrooms           *int                      `json:"bedrooms,omitempty"`
	Beds               *int                      `json:"beds,omitempty"`
	Bathrooms          *float64                  `json:"bathrooms,omitempty"`
	BasePriceMinor     int64                     `json:"base_price_minor"`
	CleaningFeeMinor   int64                     `json:"cleaning_fee_minor"`
	Currency           string                    `json:"currency"`
	InstantBook        bool                      `json:"instant_book"`
	IsGuestFavorite    bool                      `json:"is_guest_favorite"`
	CheckInTime        string                    `json:"check_in_time"`
	CheckOutTime       string                    `json:"check_out_time"`
	HouseRules         string                    `json:"house_rules"`
	Photos             []MediaResponse           `json:"photos"`
	Amenities          []models.Amenity          `json:"amenities"`
	CancellationPolicy models.CancellationPolicy `json:"cancellation_policy"`
	Host               PublicHostProfile         `json:"host"`
	Rating             *float64                  `json:"rating"`       // Always null in V1
	ReviewCount        int                       `json:"review_count"` // 0 in V1
	UnavailableDates   []CalendarNightEntry      `json:"unavailable_dates"`
}

type QuoteRequestInput struct {
	CheckIn  time.Time
	CheckOut time.Time
	Guests   int
}

type QuoteMilestone struct {
	MinHoursBefore    int       `json:"min_hours_before"`
	CutoffAt          time.Time `json:"cutoff_at"`
	RefundPct         int       `json:"refund_pct"`
	RefundsServiceFee bool      `json:"refunds_service_fee"`
}

type QuoteResponse struct {
	ListingID          uuid.UUID                 `json:"listing_id"`
	ListingVersionID   uuid.UUID                 `json:"listing_version_id"`
	CheckIn            string                    `json:"check_in"`
	CheckOut           string                    `json:"check_out"`
	CheckInTime        string                    `json:"check_in_time"`
	CheckOutTime       string                    `json:"check_out_time"`
	Timezone           string                    `json:"timezone"`
	CheckInAt          time.Time                 `json:"check_in_at"`
	CheckOutAt         time.Time                 `json:"check_out_at"`
	Guests             int                       `json:"guests"`
	InstantBook        bool                      `json:"instant_book"`
	Currency           string                    `json:"currency"`
	Breakdown          models.StayQuoteBreakdown `json:"breakdown"`
	CancellationPolicy models.CancellationPolicy `json:"cancellation_policy"`
	RefundMilestones   []QuoteMilestone          `json:"refund_milestones"`
}
