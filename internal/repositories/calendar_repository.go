package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/db"
)

type CalendarRepository struct {
	db *db.DB
}

func NewCalendarRepository(database *db.DB) interfaces.CalendarRepository {
	return &CalendarRepository{db: database}
}

func (r *CalendarRepository) ListRange(ctx context.Context, listingID uuid.UUID, startDate, endDate time.Time) ([]models.ListingCalendar, error) {
	const q = `
		SELECT listing_id, night, status, reservation_id, block_reason, created_at
		FROM listing_calendar
		WHERE listing_id = $1
		  AND night >= $2
		  AND night < $3
		ORDER BY night ASC
	`
	rows, err := r.db.Conn(ctx).Query(
		ctx,
		q,
		listingID,
		startDate.Format("2006-01-02"),
		endDate.Format("2006-01-02"),
	)
	if err != nil {
		return nil, fmt.Errorf("list calendar range: %w", err)
	}
	defer rows.Close()

	out := make([]models.ListingCalendar, 0)
	for rows.Next() {
		var c models.ListingCalendar
		if err := rows.Scan(
			&c.ListingID,
			&c.Night,
			&c.Status,
			&c.ReservationID,
			&c.BlockReason,
			&c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan calendar row: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *CalendarRepository) BlockNights(ctx context.Context, listingID uuid.UUID, nights []time.Time, reason *string) error {
	if len(nights) == 0 {
		return nil
	}
	startDate := nights[0]
	endDate := nights[len(nights)-1].AddDate(0, 0, 1)

	// Ensure none of the requested nights are currently booked
	existing, err := r.ListRange(ctx, listingID, startDate, endDate)
	if err != nil {
		return err
	}
	for _, row := range existing {
		if row.Status == enums.CalendarStatusBooked {
			return apperrors.ErrCalendarNightBooked
		}
	}

	const upsertQ = `
		INSERT INTO listing_calendar (listing_id, night, status, reservation_id, block_reason)
		VALUES ($1, $2, 'blocked', NULL, $3)
		ON CONFLICT (listing_id, night) DO UPDATE
		SET block_reason = EXCLUDED.block_reason
		WHERE listing_calendar.status = 'blocked'
	`
	for _, night := range nights {
		tag, err := r.db.Conn(ctx).Exec(ctx, upsertQ, listingID, night.Format("2006-01-02"), reason)
		if err != nil {
			return fmt.Errorf("block calendar night %s: %w", night.Format("2006-01-02"), err)
		}
		if tag.RowsAffected() == 0 {
			return apperrors.ErrCalendarNightBooked
		}
	}
	return nil
}

func (r *CalendarRepository) UnblockNights(ctx context.Context, listingID uuid.UUID, startDate, endDate time.Time) error {
	existing, err := r.ListRange(ctx, listingID, startDate, endDate)
	if err != nil {
		return err
	}
	for _, row := range existing {
		if row.Status == enums.CalendarStatusBooked {
			return apperrors.ErrCalendarNightBooked
		}
	}

	const deleteQ = `
		DELETE FROM listing_calendar
		WHERE listing_id = $1
		  AND night >= $2
		  AND night < $3
		  AND status = 'blocked'
	`
	_, err = r.db.Conn(ctx).Exec(
		ctx,
		deleteQ,
		listingID,
		startDate.Format("2006-01-02"),
		endDate.Format("2006-01-02"),
	)
	if err != nil {
		return fmt.Errorf("unblock calendar nights: %w", err)
	}
	return nil
}

// BookNights atomically locks every night in `nights` for a reservation.
// Returns apperrors.ErrDatesUnavailable if any night already exists in listing_calendar (booked or blocked).
func (r *CalendarRepository) BookNights(ctx context.Context, listingID, reservationID uuid.UUID, nights []time.Time) error {
	const insertQ = `
		INSERT INTO listing_calendar (listing_id, night, status, reservation_id)
		VALUES ($1, $2, 'booked', $3)
		ON CONFLICT (listing_id, night) DO NOTHING
	`
	for _, night := range nights {
		tag, err := r.db.Conn(ctx).Exec(ctx, insertQ, listingID, night.Format("2006-01-02"), reservationID)
		if err != nil {
			return fmt.Errorf("book night %s: %w", night.Format("2006-01-02"), err)
		}
		if tag.RowsAffected() == 0 {
			return apperrors.ErrDatesUnavailable
		}
	}
	return nil
}

func (r *CalendarRepository) ReleaseReservationNights(ctx context.Context, reservationID uuid.UUID) error {
	const deleteQ = `
		DELETE FROM listing_calendar
		WHERE reservation_id = $1 AND status = 'booked'
	`
	_, err := r.db.Conn(ctx).Exec(ctx, deleteQ, reservationID)
	if err != nil {
		return fmt.Errorf("release reservation nights: %w", err)
	}
	return nil
}
