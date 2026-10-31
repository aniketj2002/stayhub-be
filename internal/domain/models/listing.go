package models

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/enums"
)

type listingHostCtxKey struct{}

// Listing maps to the `listings` table.
type Listing struct {
	ID               uuid.UUID           `db:"id" json:"id"`
	HostID           uuid.UUID           `db:"host_id" json:"host_id"`
	Status           enums.ListingStatus `db:"status" json:"status"`
	LiveVersionID    *uuid.UUID          `db:"live_version_id" json:"live_version_id,omitempty"`
	WorkingVersionID    *uuid.UUID          `db:"working_version_id" json:"working_version_id,omitempty"`
	ICalExportToken     *string             `db:"ical_export_token" json:"ical_export_token,omitempty"`
	RatingAvg           float64             `db:"rating_avg" json:"rating_avg"`
	ReviewCount         int                 `db:"review_count" json:"review_count"`
	RatingCleanliness   *float64            `db:"rating_cleanliness" json:"rating_cleanliness,omitempty"`
	RatingAccuracy      *float64            `db:"rating_accuracy" json:"rating_accuracy,omitempty"`
	RatingCheckin       *float64            `db:"rating_checkin" json:"rating_checkin,omitempty"`
	RatingCommunication *float64            `db:"rating_communication" json:"rating_communication,omitempty"`
	RatingLocation      *float64            `db:"rating_location" json:"rating_location,omitempty"`
	RatingValue         *float64            `db:"rating_value" json:"rating_value,omitempty"`
	IsGuestFavorite     bool                `db:"is_guest_favorite" json:"is_guest_favorite"`
	CreatedAt           time.Time           `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time           `db:"updated_at" json:"updated_at"`
}

// ListingHost maps to the `listing_hosts` table (per-listing host & co-host permissions + config).
type ListingHost struct {
	ID                    uuid.UUID             `db:"id" json:"id"`
	ListingID             uuid.UUID             `db:"listing_id" json:"listing_id"`
	UserID                uuid.UUID             `db:"user_id" json:"user_id"`
	Role                  enums.ListingHostRole `db:"role" json:"role"`
	CanManageListing      bool                  `db:"can_manage_listing" json:"can_manage_listing"`
	CanManageCalendar     bool                  `db:"can_manage_calendar" json:"can_manage_calendar"`
	CanManageReservations bool                  `db:"can_manage_reservations" json:"can_manage_reservations"`
	CanMessageGuests      bool                  `db:"can_message_guests" json:"can_message_guests"`
	CanViewInsights       bool                  `db:"can_view_insights" json:"can_view_insights"`
	PayoutShareBPS        int                   `db:"payout_share_bps" json:"payout_share_bps"`
	Config                json.RawMessage       `db:"config" json:"config"`
	InvitedBy             *uuid.UUID            `db:"invited_by" json:"invited_by,omitempty"`
	CreatedAt             time.Time             `db:"created_at" json:"created_at"`
	UpdatedAt             time.Time             `db:"updated_at" json:"updated_at"`
}

func (lh *ListingHost) ActorType() enums.ActorType {
	if lh != nil && lh.Role == enums.ListingHostRoleCohost {
		return enums.ActorCohost
	}
	return enums.ActorHost
}

func WithListingHost(ctx context.Context, lh *ListingHost) context.Context {
	return context.WithValue(ctx, listingHostCtxKey{}, lh)
}

func ListingHostFromContext(ctx context.Context) (*ListingHost, bool) {
	v, ok := ctx.Value(listingHostCtxKey{}).(*ListingHost)
	return v, ok && v != nil
}

// ListingVersion maps to the `listing_versions` table.
type ListingVersion struct {
	ID                 uuid.UUID           `db:"id" json:"id"`
	ListingID          uuid.UUID           `db:"listing_id" json:"listing_id"`
	VersionNo          int                 `db:"version_no" json:"version_no"`
	Status             enums.VersionStatus `db:"status" json:"status"`
	WizardStep         int                 `db:"wizard_step" json:"wizard_step"`
	PropertyType       *string             `db:"property_type" json:"property_type,omitempty"`
	Title              *string             `db:"title" json:"title,omitempty"`
	Description        *string             `db:"description" json:"description,omitempty"`
	AddressLine1       *string             `db:"address_line1" json:"address_line1,omitempty"`
	AddressLine2       *string             `db:"address_line2" json:"address_line2,omitempty"`
	City               *string             `db:"city" json:"city,omitempty"`
	Region             *string             `db:"region" json:"region,omitempty"`
	Country            *string             `db:"country" json:"country,omitempty"`
	PostalCode         *string             `db:"postal_code" json:"postal_code,omitempty"`
	Latitude           *float64            `db:"latitude" json:"latitude,omitempty"`
	Longitude          *float64            `db:"longitude" json:"longitude,omitempty"`
	Timezone           string              `db:"timezone" json:"timezone"`
	MaxGuests          *int                `db:"max_guests" json:"max_guests,omitempty"`
	Bedrooms           *int                `db:"bedrooms" json:"bedrooms,omitempty"`
	Beds               *int                `db:"beds" json:"beds,omitempty"`
	Bathrooms          *float64            `db:"bathrooms" json:"bathrooms,omitempty"`
	BasePriceMinor     *int64              `db:"base_price_minor" json:"base_price_minor,omitempty"`
	CleaningFeeMinor   int64               `db:"cleaning_fee_minor" json:"cleaning_fee_minor"`
	Currency           string              `db:"currency" json:"currency"`
	InstantBook        bool                `db:"instant_book" json:"instant_book"`
	CheckInTime        string              `db:"check_in_time" json:"check_in_time"`
	CheckOutTime       string              `db:"check_out_time" json:"check_out_time"`
	HouseRules         *string             `db:"house_rules" json:"house_rules,omitempty"`
	CancellationPolicy *string             `db:"cancellation_policy" json:"cancellation_policy,omitempty"`
	Amenities          []string            `db:"amenities" json:"amenities"`
	MediaIDs           []uuid.UUID         `db:"media_ids" json:"media_ids"`
	SubmittedAt        *time.Time          `db:"submitted_at" json:"submitted_at,omitempty"`
	DecidedAt          *time.Time          `db:"decided_at" json:"decided_at,omitempty"`
	DecidedBy          *uuid.UUID          `db:"decided_by" json:"decided_by,omitempty"`
	DecisionNote       *string             `db:"decision_note" json:"decision_note,omitempty"`
	CreatedAt          time.Time           `db:"created_at" json:"created_at"`
	UpdatedAt          time.Time           `db:"updated_at" json:"updated_at"`
}

// ListingCheckResult maps to the `listing_check_results` table.
type ListingCheckResult struct {
	ID               uuid.UUID `db:"id" json:"id"`
	ListingVersionID uuid.UUID `db:"listing_version_id" json:"listing_version_id"`
	CheckCode        string    `db:"check_code" json:"check_code"`
	Outcome          string    `db:"outcome" json:"outcome"` // 'pass' | 'warn' | 'fail'
	Detail           *string   `db:"detail" json:"detail,omitempty"`
	CreatedAt        time.Time `db:"created_at" json:"created_at"`
}

// ListingCalendar maps to the `listing_calendar` table.
type ListingCalendar struct {
	ListingID     uuid.UUID            `db:"listing_id" json:"listing_id"`
	Night         time.Time            `db:"night" json:"night"`
	Status        enums.CalendarStatus `db:"status" json:"status"`
	ReservationID *uuid.UUID           `db:"reservation_id" json:"reservation_id,omitempty"`
	BlockReason   *string              `db:"block_reason" json:"block_reason,omitempty"`
	ICalFeedID    *uuid.UUID           `db:"ical_feed_id" json:"ical_feed_id,omitempty"`
	CreatedAt     time.Time            `db:"created_at" json:"created_at"`
}
