package dtos

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/models"
)

type CreateListingInput struct {
	PropertyType *string `json:"property_type,omitempty"`
	Title        *string `json:"title,omitempty"`
	Timezone     string  `json:"timezone,omitempty"`
	Currency     string  `json:"currency,omitempty"`
}

type UpdateWorkingVersionInput struct {
	WizardStep         *int         `json:"wizard_step,omitempty"`
	PropertyType       *string      `json:"property_type,omitempty"`
	Title              *string      `json:"title,omitempty"`
	Description        *string      `json:"description,omitempty"`
	AddressLine1       *string      `json:"address_line1,omitempty"`
	AddressLine2       *string      `json:"address_line2,omitempty"`
	City               *string      `json:"city,omitempty"`
	Region             *string      `json:"region,omitempty"`
	Country            *string      `json:"country,omitempty"`
	PostalCode         *string      `json:"postal_code,omitempty"`
	Timezone           *string      `json:"timezone,omitempty"`
	MaxGuests          *int         `json:"max_guests,omitempty"`
	BasePriceMinor     *int64       `json:"base_price_minor,omitempty"`
	CleaningFeeMinor   *int64       `json:"cleaning_fee_minor,omitempty"`
	Currency           *string      `json:"currency,omitempty"`
	InstantBook        *bool        `json:"instant_book,omitempty"`
	CheckInTime        *string      `json:"check_in_time,omitempty"`
	CheckOutTime       *string      `json:"check_out_time,omitempty"`
	HouseRules         *string      `json:"house_rules,omitempty"`
	CancellationPolicy *string      `json:"cancellation_policy,omitempty"`
	Amenities          *[]string    `json:"amenities,omitempty"`
	MediaIDs           *[]uuid.UUID `json:"media_ids,omitempty"`
}

type UpdateHostConfigInput struct {
	Config json.RawMessage `json:"config"`
}

type ListingVersionResponse struct {
	models.ListingVersion
	Photos []MediaResponse `json:"photos"`
}

type HostListingDetailResponse struct {
	Listing        models.Listing             `json:"listing"`
	WorkingVersion *ListingVersionResponse    `json:"working_version,omitempty"`
	LiveVersion    *ListingVersionResponse    `json:"live_version,omitempty"`
	CheckResults   []models.ListingCheckResult `json:"check_results"`
	HostAccess     models.ListingHost         `json:"host_access"`
}

type HostListingSummaryResponse struct {
	ID             uuid.UUID             `json:"id"`
	HostID         uuid.UUID             `json:"host_id"`
	Status         enums.ListingStatus   `json:"status"`
	Title          *string               `json:"title,omitempty"`
	PropertyType   *string               `json:"property_type,omitempty"`
	City           *string               `json:"city,omitempty"`
	Country        *string               `json:"country,omitempty"`
	MaxGuests      *int                  `json:"max_guests,omitempty"`
	BasePriceMinor *int64                `json:"base_price_minor,omitempty"`
	Currency       string                `json:"currency"`
	InstantBook    bool                  `json:"instant_book"`
	WizardStep     int                   `json:"wizard_step"`
	CoverPhotoURL  *string               `json:"cover_photo_url,omitempty"`
	HostRole       enums.ListingHostRole `json:"host_role"`
	HostAccess     models.ListingHost    `json:"host_access"`
	CreatedAt      time.Time             `json:"created_at"`
	UpdatedAt      time.Time             `json:"updated_at"`
}

type SubmitListingResponse struct {
	Listing      models.Listing              `json:"listing"`
	Version      ListingVersionResponse      `json:"version"`
	CheckResults []models.ListingCheckResult `json:"check_results"`
	AllPassed    bool                        `json:"all_passed"`
}
