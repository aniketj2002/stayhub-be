package dtos

import (
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/enums"
)

type CreateReviewInput struct {
	Rating               int     `json:"rating"`
	CleanlinessRating    *int    `json:"cleanliness_rating,omitempty"`
	AccuracyRating       *int    `json:"accuracy_rating,omitempty"`
	CheckInRating        *int    `json:"check_in_rating,omitempty"`
	CommunicationRating  *int    `json:"communication_rating,omitempty"`
	LocationRating       *int    `json:"location_rating,omitempty"`
	ValueRating          *int    `json:"value_rating,omitempty"`
	PublicReview         string  `json:"public_review"`
	PrivateNote          *string `json:"private_note,omitempty"`
}

type ReviewResponse struct {
	ID                   uuid.UUID              `json:"id"`
	ReservationID        uuid.UUID              `json:"reservation_id"`
	AuthorID             uuid.UUID              `json:"author_id"`
	AuthorName           string                 `json:"author_name"`
	TargetType           enums.ReviewTargetType `json:"target_type"`
	ListingID            uuid.UUID              `json:"listing_id"`
	TargetUserID         *uuid.UUID             `json:"target_user_id,omitempty"`
	Rating               int                    `json:"rating"`
	CleanlinessRating    *int                   `json:"cleanliness_rating,omitempty"`
	AccuracyRating       *int                   `json:"accuracy_rating,omitempty"`
	CheckInRating        *int                   `json:"check_in_rating,omitempty"`
	CommunicationRating  *int                   `json:"communication_rating,omitempty"`
	LocationRating       *int                   `json:"location_rating,omitempty"`
	ValueRating          *int                   `json:"value_rating,omitempty"`
	PublicReview         string                 `json:"public_review"`
	PrivateNote          *string                `json:"private_note,omitempty"`
	PublishedAt          *time.Time             `json:"published_at,omitempty"`
	CreatedAt            time.Time              `json:"created_at"`
}

type ListingReviewSummaryResponse struct {
	RatingAvg           float64          `json:"rating_avg"`
	ReviewCount         int              `json:"review_count"`
	RatingCleanliness   *float64         `json:"rating_cleanliness,omitempty"`
	RatingAccuracy      *float64         `json:"rating_accuracy,omitempty"`
	RatingCheckin       *float64         `json:"rating_checkin,omitempty"`
	RatingCommunication *float64         `json:"rating_communication,omitempty"`
	RatingLocation      *float64         `json:"rating_location,omitempty"`
	RatingValue         *float64         `json:"rating_value,omitempty"`
	Reviews             []ReviewResponse `json:"reviews"`
}
