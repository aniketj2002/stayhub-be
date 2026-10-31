package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/models"
)

type ReviewRepository interface {
	CreateReview(ctx context.Context, review models.Review) (*models.Review, error)
	FindReviewByReservationAndAuthor(ctx context.Context, reservationID, authorID uuid.UUID) (*models.Review, error)
	ListReviewsByReservation(ctx context.Context, reservationID uuid.UUID) ([]models.Review, error)
	ListPublishedReviewsByListing(ctx context.Context, listingID uuid.UUID) ([]models.Review, error)
	PublishReviews(ctx context.Context, reviewIDs []uuid.UUID, publishedAt time.Time) error
	ListUnpublishedReviewsPastDeadline(ctx context.Context, deadline time.Time) ([]models.Review, error)
	UpdateListingRatingStats(ctx context.Context, listingID uuid.UUID, stats models.ListingRatingStats) error
}

type ReviewService interface {
	SubmitReview(ctx context.Context, author models.User, reservationID uuid.UUID, in dtos.CreateReviewInput) (*dtos.ReviewResponse, error)
	ListListingReviews(ctx context.Context, listingID uuid.UUID) (*dtos.ListingReviewSummaryResponse, error)
	GetReservationReviews(ctx context.Context, userID, reservationID uuid.UUID) ([]dtos.ReviewResponse, error)
	CloseReviewWindows(ctx context.Context) (int, error)
}
