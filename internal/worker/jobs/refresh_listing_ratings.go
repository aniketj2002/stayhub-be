package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/stayhub/backend/internal/platform/db"
)

type RefreshListingRatingsJob struct {
	db *db.DB
}

func NewRefreshListingRatingsJob(database *db.DB) *RefreshListingRatingsJob {
	return &RefreshListingRatingsJob{db: database}
}

func (j *RefreshListingRatingsJob) Name() string {
	return "refresh_listing_ratings"
}

func (j *RefreshListingRatingsJob) Run(ctx context.Context, _ time.Time, _ int) (int, error) {
	q := `
		WITH review_stats AS (
			SELECT
				listing_id,
				ROUND(AVG(overall_rating)::numeric, 2) AS avg_rating,
				COUNT(*) AS cnt,
				ROUND(AVG(rating_cleanliness)::numeric, 2) AS avg_cleanliness,
				ROUND(AVG(rating_accuracy)::numeric, 2) AS avg_accuracy,
				ROUND(AVG(rating_checkin)::numeric, 2) AS avg_checkin,
				ROUND(AVG(rating_communication)::numeric, 2) AS avg_communication,
				ROUND(AVG(rating_location)::numeric, 2) AS avg_location,
				ROUND(AVG(rating_value)::numeric, 2) AS avg_value
			FROM reviews
			WHERE target_type = 'listing' AND published_at IS NOT NULL
			GROUP BY listing_id
		)
		UPDATE listings l
		SET
			rating_avg = rs.avg_rating,
			review_count = rs.cnt,
			rating_cleanliness = rs.avg_cleanliness,
			rating_accuracy = rs.avg_accuracy,
			rating_checkin = rs.avg_checkin,
			rating_communication = rs.avg_communication,
			rating_location = rs.avg_location,
			rating_value = rs.avg_value
		FROM review_stats rs
		WHERE l.id = rs.listing_id
	`
	res, err := j.db.Conn(ctx).Exec(ctx, q)
	if err != nil {
		return 0, fmt.Errorf("refresh listing ratings: %w", err)
	}
	return int(res.RowsAffected()), nil
}
