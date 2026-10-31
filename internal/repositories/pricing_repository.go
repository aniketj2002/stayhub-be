package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/db"
)

type PricingRepository struct {
	db *db.DB
}

func NewPricingRepository(database *db.DB) interfaces.PricingRepository {
	return &PricingRepository{db: database}
}

func (r *PricingRepository) GetPricingRule(ctx context.Context, listingID uuid.UUID) (*models.ListingPricingRule, error) {
	const q = `
		SELECT listing_id, weekend_price_minor, weekend_days, min_nights,
		       weekly_discount_bps, monthly_discount_bps, created_at, updated_at
		FROM listing_pricing_rules
		WHERE listing_id = $1
	`
	var rule models.ListingPricingRule
	var weekendDays []int16
	err := r.db.Conn(ctx).QueryRow(ctx, q, listingID).Scan(
		&rule.ListingID,
		&rule.WeekendPriceMinor,
		&weekendDays,
		&rule.MinNights,
		&rule.WeeklyDiscountBPS,
		&rule.MonthlyDiscountBPS,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("get pricing rule: %w", err)
	}
	rule.WeekendDays = make([]int, len(weekendDays))
	for i, d := range weekendDays {
		rule.WeekendDays[i] = int(d)
	}
	return &rule, nil
}

func (r *PricingRepository) UpsertPricingRule(ctx context.Context, rule models.ListingPricingRule) (*models.ListingPricingRule, error) {
	const q = `
		INSERT INTO listing_pricing_rules (
			listing_id, weekend_price_minor, weekend_days, min_nights,
			weekly_discount_bps, monthly_discount_bps, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, now(), now()
		)
		ON CONFLICT (listing_id) DO UPDATE
		SET weekend_price_minor = EXCLUDED.weekend_price_minor,
		    weekend_days = EXCLUDED.weekend_days,
		    min_nights = EXCLUDED.min_nights,
		    weekly_discount_bps = EXCLUDED.weekly_discount_bps,
		    monthly_discount_bps = EXCLUDED.monthly_discount_bps,
		    updated_at = now()
		RETURNING listing_id, weekend_price_minor, weekend_days, min_nights,
		          weekly_discount_bps, monthly_discount_bps, created_at, updated_at
	`
	weekendDays := make([]int16, len(rule.WeekendDays))
	for i, d := range rule.WeekendDays {
		weekendDays[i] = int16(d)
	}
	var out models.ListingPricingRule
	var outDays []int16
	err := r.db.Conn(ctx).QueryRow(
		ctx, q,
		rule.ListingID,
		rule.WeekendPriceMinor,
		weekendDays,
		rule.MinNights,
		rule.WeeklyDiscountBPS,
		rule.MonthlyDiscountBPS,
	).Scan(
		&out.ListingID,
		&out.WeekendPriceMinor,
		&outDays,
		&out.MinNights,
		&out.WeeklyDiscountBPS,
		&out.MonthlyDiscountBPS,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert pricing rule: %w", err)
	}
	out.WeekendDays = make([]int, len(outDays))
	for i, d := range outDays {
		out.WeekendDays[i] = int(d)
	}
	return &out, nil
}

func (r *PricingRepository) GetDateOverrides(ctx context.Context, listingID uuid.UUID, from, to time.Time) ([]models.ListingDateOverride, error) {
	const q = `
		SELECT listing_id, night, price_minor, min_nights, created_at, updated_at
		FROM listing_date_overrides
		WHERE listing_id = $1 AND night >= $2 AND night <= $3
		ORDER BY night ASC
	`
	rows, err := r.db.Conn(ctx).Query(ctx, q, listingID, from, to)
	if err != nil {
		return nil, fmt.Errorf("query date overrides: %w", err)
	}
	defer rows.Close()

	var list []models.ListingDateOverride
	for rows.Next() {
		var o models.ListingDateOverride
		if err := rows.Scan(
			&o.ListingID,
			&o.Night,
			&o.PriceMinor,
			&o.MinNights,
			&o.CreatedAt,
			&o.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan date override: %w", err)
		}
		list = append(list, o)
	}
	return list, rows.Err()
}

func (r *PricingRepository) UpsertDateOverrides(ctx context.Context, listingID uuid.UUID, overrides []models.ListingDateOverride) error {
	if len(overrides) == 0 {
		return nil
	}
	const q = `
		INSERT INTO listing_date_overrides (
			listing_id, night, price_minor, min_nights, created_at, updated_at
		) VALUES ($1, $2, $3, $4, now(), now())
		ON CONFLICT (listing_id, night) DO UPDATE
		SET price_minor = EXCLUDED.price_minor,
		    min_nights = EXCLUDED.min_nights,
		    updated_at = now()
	`
	b := &pgx.Batch{}
	for _, o := range overrides {
		b.Queue(q, listingID, o.Night, o.PriceMinor, o.MinNights)
	}
	br := r.db.Conn(ctx).SendBatch(ctx, b)
	defer br.Close()

	for range overrides {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("exec batch override: %w", err)
		}
	}
	return nil
}

func (r *PricingRepository) DeleteDateOverrides(ctx context.Context, listingID uuid.UUID, from, to time.Time) error {
	const q = `
		DELETE FROM listing_date_overrides
		WHERE listing_id = $1 AND night >= $2 AND night <= $3
	`
	_, err := r.db.Conn(ctx).Exec(ctx, q, listingID, from, to)
	if err != nil {
		return fmt.Errorf("delete date overrides: %w", err)
	}
	return nil
}

func (r *PricingRepository) CreateReservationNights(ctx context.Context, nights []models.ReservationNight) error {
	if len(nights) == 0 {
		return nil
	}
	const q = `
		INSERT INTO reservation_nights (
			reservation_id, night, price_minor, rate_type, created_at
		) VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (reservation_id, night) DO NOTHING
	`
	b := &pgx.Batch{}
	for _, n := range nights {
		b.Queue(q, n.ReservationID, n.Night, n.PriceMinor, n.RateType)
	}
	br := r.db.Conn(ctx).SendBatch(ctx, b)
	defer br.Close()

	for range nights {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("insert reservation night: %w", err)
		}
	}
	return nil
}

func (r *PricingRepository) ListReservationNights(ctx context.Context, reservationID uuid.UUID) ([]models.ReservationNight, error) {
	const q = `
		SELECT id, reservation_id, night, price_minor, rate_type, created_at
		FROM reservation_nights
		WHERE reservation_id = $1
		ORDER BY night ASC
	`
	rows, err := r.db.Conn(ctx).Query(ctx, q, reservationID)
	if err != nil {
		return nil, fmt.Errorf("list reservation nights: %w", err)
	}
	defer rows.Close()

	var list []models.ReservationNight
	for rows.Next() {
		var n models.ReservationNight
		if err := rows.Scan(
			&n.ID,
			&n.ReservationID,
			&n.Night,
			&n.PriceMinor,
			&n.RateType,
			&n.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan reservation night: %w", err)
		}
		list = append(list, n)
	}
	return list, rows.Err()
}
