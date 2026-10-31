package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/db"
)

type ReviewRepository struct {
	db *db.DB
}

func NewReviewRepository(database *db.DB) interfaces.ReviewRepository {
	return &ReviewRepository{db: database}
}

func (r *ReviewRepository) CreateReview(ctx context.Context, rev models.Review) (*models.Review, error) {
	if rev.ID == uuid.Nil {
		rev.ID = uuid.New()
	}

	query := `
		INSERT INTO reviews (
			id, reservation_id, author_id, target_type, listing_id, target_user_id,
			rating, cleanliness_rating, accuracy_rating, check_in_rating,
			communication_rating, location_rating, value_rating,
			public_review, private_note, published_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13,
			$14, $15, $16, now(), now()
		)
		RETURNING id, reservation_id, author_id, target_type, listing_id, target_user_id,
		          rating, cleanliness_rating, accuracy_rating, check_in_rating,
		          communication_rating, location_rating, value_rating,
		          public_review, private_note, published_at, created_at, updated_at;
	`
	var out models.Review
	err := r.db.Pool.QueryRow(
		ctx,
		query,
		rev.ID,
		rev.ReservationID,
		rev.AuthorID,
		rev.TargetType,
		rev.ListingID,
		rev.TargetUserID,
		rev.Rating,
		rev.CleanlinessRating,
		rev.AccuracyRating,
		rev.CheckInRating,
		rev.CommunicationRating,
		rev.LocationRating,
		rev.ValueRating,
		rev.PublicReview,
		rev.PrivateNote,
		rev.PublishedAt,
	).Scan(
		&out.ID,
		&out.ReservationID,
		&out.AuthorID,
		&out.TargetType,
		&out.ListingID,
		&out.TargetUserID,
		&out.Rating,
		&out.CleanlinessRating,
		&out.AccuracyRating,
		&out.CheckInRating,
		&out.CommunicationRating,
		&out.LocationRating,
		&out.ValueRating,
		&out.PublicReview,
		&out.PrivateNote,
		&out.PublishedAt,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *ReviewRepository) FindReviewByReservationAndAuthor(
	ctx context.Context,
	reservationID, authorID uuid.UUID,
) (*models.Review, error) {
	query := `
		SELECT id, reservation_id, author_id, target_type, listing_id, target_user_id,
		       rating, cleanliness_rating, accuracy_rating, check_in_rating,
		       communication_rating, location_rating, value_rating,
		       public_review, private_note, published_at, created_at, updated_at
		FROM reviews
		WHERE reservation_id = $1 AND author_id = $2;
	`
	var out models.Review
	err := r.db.Pool.QueryRow(ctx, query, reservationID, authorID).Scan(
		&out.ID,
		&out.ReservationID,
		&out.AuthorID,
		&out.TargetType,
		&out.ListingID,
		&out.TargetUserID,
		&out.Rating,
		&out.CleanlinessRating,
		&out.AccuracyRating,
		&out.CheckInRating,
		&out.CommunicationRating,
		&out.LocationRating,
		&out.ValueRating,
		&out.PublicReview,
		&out.PrivateNote,
		&out.PublishedAt,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &out, nil
}

func (r *ReviewRepository) ListReviewsByReservation(ctx context.Context, reservationID uuid.UUID) ([]models.Review, error) {
	query := `
		SELECT id, reservation_id, author_id, target_type, listing_id, target_user_id,
		       rating, cleanliness_rating, accuracy_rating, check_in_rating,
		       communication_rating, location_rating, value_rating,
		       public_review, private_note, published_at, created_at, updated_at
		FROM reviews
		WHERE reservation_id = $1;
	`
	rows, err := r.db.Pool.Query(ctx, query, reservationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Review
	for rows.Next() {
		var rev models.Review
		if err := rows.Scan(
			&rev.ID,
			&rev.ReservationID,
			&rev.AuthorID,
			&rev.TargetType,
			&rev.ListingID,
			&rev.TargetUserID,
			&rev.Rating,
			&rev.CleanlinessRating,
			&rev.AccuracyRating,
			&rev.CheckInRating,
			&rev.CommunicationRating,
			&rev.LocationRating,
			&rev.ValueRating,
			&rev.PublicReview,
			&rev.PrivateNote,
			&rev.PublishedAt,
			&rev.CreatedAt,
			&rev.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, rev)
	}
	return out, rows.Err()
}

func (r *ReviewRepository) ListPublishedReviewsByListing(ctx context.Context, listingID uuid.UUID) ([]models.Review, error) {
	query := `
		SELECT id, reservation_id, author_id, target_type, listing_id, target_user_id,
		       rating, cleanliness_rating, accuracy_rating, check_in_rating,
		       communication_rating, location_rating, value_rating,
		       public_review, private_note, published_at, created_at, updated_at
		FROM reviews
		WHERE listing_id = $1
		  AND target_type = 'listing'
		  AND published_at IS NOT NULL
		ORDER BY published_at DESC;
	`
	rows, err := r.db.Pool.Query(ctx, query, listingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Review
	for rows.Next() {
		var rev models.Review
		if err := rows.Scan(
			&rev.ID,
			&rev.ReservationID,
			&rev.AuthorID,
			&rev.TargetType,
			&rev.ListingID,
			&rev.TargetUserID,
			&rev.Rating,
			&rev.CleanlinessRating,
			&rev.AccuracyRating,
			&rev.CheckInRating,
			&rev.CommunicationRating,
			&rev.LocationRating,
			&rev.ValueRating,
			&rev.PublicReview,
			&rev.PrivateNote,
			&rev.PublishedAt,
			&rev.CreatedAt,
			&rev.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, rev)
	}
	return out, rows.Err()
}

func (r *ReviewRepository) PublishReviews(ctx context.Context, reviewIDs []uuid.UUID, publishedAt time.Time) error {
	if len(reviewIDs) == 0 {
		return nil
	}
	query := `
		UPDATE reviews
		SET published_at = $1, updated_at = now()
		WHERE id = ANY($2) AND published_at IS NULL;
	`
	_, err := r.db.Pool.Exec(ctx, query, publishedAt, reviewIDs)
	return err
}

func (r *ReviewRepository) ListUnpublishedReviewsPastDeadline(ctx context.Context, deadline time.Time) ([]models.Review, error) {
	query := `
		SELECT rev.id, rev.reservation_id, rev.author_id, rev.target_type, rev.listing_id, rev.target_user_id,
		       rev.rating, rev.cleanliness_rating, rev.accuracy_rating, rev.check_in_rating,
		       rev.communication_rating, rev.location_rating, rev.value_rating,
		       rev.public_review, rev.private_note, rev.published_at, rev.created_at, rev.updated_at
		FROM reviews rev
		JOIN reservations res ON res.id = rev.reservation_id
		WHERE rev.published_at IS NULL
		  AND res.review_deadline IS NOT NULL
		  AND res.review_deadline <= $1;
	`
	rows, err := r.db.Pool.Query(ctx, query, deadline)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Review
	for rows.Next() {
		var rev models.Review
		if err := rows.Scan(
			&rev.ID,
			&rev.ReservationID,
			&rev.AuthorID,
			&rev.TargetType,
			&rev.ListingID,
			&rev.TargetUserID,
			&rev.Rating,
			&rev.CleanlinessRating,
			&rev.AccuracyRating,
			&rev.CheckInRating,
			&rev.CommunicationRating,
			&rev.LocationRating,
			&rev.ValueRating,
			&rev.PublicReview,
			&rev.PrivateNote,
			&rev.PublishedAt,
			&rev.CreatedAt,
			&rev.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, rev)
	}
	return out, rows.Err()
}

func (r *ReviewRepository) UpdateListingRatingStats(ctx context.Context, listingID uuid.UUID, stats models.ListingRatingStats) error {
	query := `
		UPDATE listings
		SET rating_avg = $1,
		    review_count = $2,
		    rating_cleanliness = $3,
		    rating_accuracy = $4,
		    rating_checkin = $5,
		    rating_communication = $6,
		    rating_location = $7,
		    rating_value = $8,
		    updated_at = now()
		WHERE id = $9;
	`
	_, err := r.db.Pool.Exec(
		ctx,
		query,
		stats.RatingAvg,
		stats.ReviewCount,
		stats.RatingCleanliness,
		stats.RatingAccuracy,
		stats.RatingCheckin,
		stats.RatingCommunication,
		stats.RatingLocation,
		stats.RatingValue,
		listingID,
	)
	return err
}
