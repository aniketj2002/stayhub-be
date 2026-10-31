package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/enums"
)

// Review maps to the `reviews` table.
type Review struct {
	ID                   uuid.UUID              `db:"id" json:"id"`
	ReservationID        uuid.UUID              `db:"reservation_id" json:"reservation_id"`
	AuthorID             uuid.UUID              `db:"author_id" json:"author_id"`
	TargetType           enums.ReviewTargetType `db:"target_type" json:"target_type"`
	ListingID            uuid.UUID              `db:"listing_id" json:"listing_id"`
	TargetUserID         *uuid.UUID             `db:"target_user_id" json:"target_user_id,omitempty"`
	Rating               int                    `db:"rating" json:"rating"`
	CleanlinessRating    *int                   `db:"cleanliness_rating" json:"cleanliness_rating,omitempty"`
	AccuracyRating       *int                   `db:"accuracy_rating" json:"accuracy_rating,omitempty"`
	CheckInRating        *int                   `db:"check_in_rating" json:"check_in_rating,omitempty"`
	CommunicationRating  *int                   `db:"communication_rating" json:"communication_rating,omitempty"`
	LocationRating       *int                   `db:"location_rating" json:"location_rating,omitempty"`
	ValueRating          *int                   `db:"value_rating" json:"value_rating,omitempty"`
	PublicReview         string                 `db:"public_review" json:"public_review"`
	PrivateNote          *string                `db:"private_note" json:"private_note,omitempty"`
	PublishedAt          *time.Time             `db:"published_at" json:"published_at,omitempty"`
	CreatedAt            time.Time              `db:"created_at" json:"created_at"`
	UpdatedAt            time.Time              `db:"updated_at" json:"updated_at"`
}

// ListingRatingStats represents aggregated review ratings for a listing.
type ListingRatingStats struct {
	RatingAvg           float64  `db:"rating_avg" json:"rating_avg"`
	ReviewCount         int      `db:"review_count" json:"review_count"`
	RatingCleanliness   *float64 `db:"rating_cleanliness" json:"rating_cleanliness,omitempty"`
	RatingAccuracy      *float64 `db:"rating_accuracy" json:"rating_accuracy,omitempty"`
	RatingCheckin       *float64 `db:"rating_checkin" json:"rating_checkin,omitempty"`
	RatingCommunication *float64 `db:"rating_communication" json:"rating_communication,omitempty"`
	RatingLocation      *float64 `db:"rating_location" json:"rating_location,omitempty"`
	RatingValue         *float64 `db:"rating_value" json:"rating_value,omitempty"`
}
