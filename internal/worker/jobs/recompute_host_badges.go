package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/stayhub/backend/internal/platform/db"
)

type RecomputeHostBadgesJob struct {
	db *db.DB
}

func NewRecomputeHostBadgesJob(database *db.DB) *RecomputeHostBadgesJob {
	return &RecomputeHostBadgesJob{db: database}
}

func (j *RecomputeHostBadgesJob) Name() string {
	return "recompute_host_badges"
}

func (j *RecomputeHostBadgesJob) Run(ctx context.Context, _ time.Time, _ int) (int, error) {
	// 1. Recompute Superhost status for hosts:
	// Criteria: rating_avg >= 4.80, completed_stays >= 10, host cancellation rate < 1%
	superhostSQL := `
		WITH host_metrics AS (
			SELECT
				u.id AS user_id,
				COALESCE(AVG(r.overall_rating), 0) AS avg_rating,
				COUNT(DISTINCT res.id) FILTER (WHERE res.status = 'completed') AS completed_stays,
				COUNT(DISTINCT res.id) FILTER (WHERE res.status = 'cancelled_by_host') AS host_cancellations,
				COUNT(DISTINCT res.id) AS total_reservations
			FROM users u
			LEFT JOIN listings l ON l.host_id = u.id
			LEFT JOIN reservations res ON res.listing_id = l.id
			LEFT JOIN reviews r ON r.listing_id = l.id AND r.target_type = 'listing' AND r.published_at IS NOT NULL
			WHERE u.is_host = true
			GROUP BY u.id
		)
		UPDATE users u
		SET is_superhost = (
			hm.avg_rating >= 4.80 AND
			hm.completed_stays >= 10 AND
			(hm.total_reservations = 0 OR (hm.host_cancellations::float / hm.total_reservations) < 0.01)
		)
		FROM host_metrics hm
		WHERE u.id = hm.user_id AND u.is_superhost <> (
			hm.avg_rating >= 4.80 AND
			hm.completed_stays >= 10 AND
			(hm.total_reservations = 0 OR (hm.host_cancellations::float / hm.total_reservations) < 0.01)
		)
	`
	resSuperhost, err := j.db.Conn(ctx).Exec(ctx, superhostSQL)
	if err != nil {
		return 0, fmt.Errorf("recompute superhost badges: %w", err)
	}

	// 2. Recompute Guest Favorite for listings:
	// Criteria: rating_avg >= 4.80 AND review_count >= 5
	guestFavSQL := `
		UPDATE listings
		SET is_guest_favorite = (rating_avg >= 4.80 AND review_count >= 5)
		WHERE is_guest_favorite <> (rating_avg >= 4.80 AND review_count >= 5)
	`
	resGuestFav, err := j.db.Conn(ctx).Exec(ctx, guestFavSQL)
	if err != nil {
		return int(resSuperhost.RowsAffected()), fmt.Errorf("recompute guest favorite badges: %w", err)
	}

	return int(resSuperhost.RowsAffected() + resGuestFav.RowsAffected()), nil
}
