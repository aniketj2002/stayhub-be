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

type ReservationRepository struct {
	db *db.DB
}

func NewReservationRepository(database *db.DB) interfaces.ReservationRepository {
	return &ReservationRepository{db: database}
}

const reservationSelectCols = `
	id, code, listing_id, listing_version_id, guest_id, host_id,
	status, is_instant_book, check_in, check_out,
	to_char(check_in_time, 'HH24:MI'), to_char(check_out_time, 'HH24:MI'),
	timezone, check_in_at, check_out_at, guests, guest_message,
	TRIM(currency), nightly_total_minor, cleaning_fee_minor,
	guest_fee_bps, guest_fee_minor, guest_total_minor,
	host_fee_bps, host_fee_minor, host_net_minor,
	cancellation_policy, request_expires_at, decided_at, decline_reason,
	cancelled_at, cancelled_by, cancel_reason, checked_in_at, completed_at,
	version, created_at, updated_at
`

func scanReservation(row pgx.Row) (*models.Reservation, error) {
	var r models.Reservation
	err := row.Scan(
		&r.ID,
		&r.Code,
		&r.ListingID,
		&r.ListingVersionID,
		&r.GuestID,
		&r.HostID,
		&r.Status,
		&r.IsInstantBook,
		&r.CheckIn,
		&r.CheckOut,
		&r.CheckInTime,
		&r.CheckOutTime,
		&r.Timezone,
		&r.CheckInAt,
		&r.CheckOutAt,
		&r.Guests,
		&r.GuestMessage,
		&r.Currency,
		&r.NightlyTotalMinor,
		&r.CleaningFeeMinor,
		&r.GuestFeeBPS,
		&r.GuestFeeMinor,
		&r.GuestTotalMinor,
		&r.HostFeeBPS,
		&r.HostFeeMinor,
		&r.HostNetMinor,
		&r.CancellationPolicy,
		&r.RequestExpiresAt,
		&r.DecidedAt,
		&r.DeclineReason,
		&r.CancelledAt,
		&r.CancelledBy,
		&r.CancelReason,
		&r.CheckedInAt,
		&r.CompletedAt,
		&r.Version,
		&r.CreatedAt,
		&r.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &r, nil
}

func scanReservations(rows pgx.Rows) ([]models.Reservation, error) {
	out := make([]models.Reservation, 0)
	for rows.Next() {
		r, err := scanReservation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan reservation row: %w", err)
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

func (r *ReservationRepository) Create(ctx context.Context, res models.Reservation) (*models.Reservation, error) {
	if res.ID == uuid.Nil {
		res.ID = uuid.New()
	}
	q := fmt.Sprintf(`
		INSERT INTO reservations (
			id, code, listing_id, listing_version_id, guest_id, host_id,
			status, is_instant_book, check_in, check_out,
			check_in_time, check_out_time, timezone, check_in_at, check_out_at,
			guests, guest_message, currency,
			nightly_total_minor, cleaning_fee_minor,
			guest_fee_bps, guest_fee_minor, guest_total_minor,
			host_fee_bps, host_fee_minor, host_net_minor,
			cancellation_policy, request_expires_at, decided_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11::time, $12::time, $13, $14, $15,
			$16, $17, $18,
			$19, $20,
			$21, $22, $23,
			$24, $25, $26,
			$27, $28, $29
		)
		RETURNING %s
	`, reservationSelectCols)

	row := r.db.Conn(ctx).QueryRow(
		ctx,
		q,
		res.ID,
		res.Code,
		res.ListingID,
		res.ListingVersionID,
		res.GuestID,
		res.HostID,
		res.Status,
		res.IsInstantBook,
		res.CheckIn.Format("2006-01-02"),
		res.CheckOut.Format("2006-01-02"),
		res.CheckInTime,
		res.CheckOutTime,
		res.Timezone,
		res.CheckInAt,
		res.CheckOutAt,
		res.Guests,
		res.GuestMessage,
		res.Currency,
		res.NightlyTotalMinor,
		res.CleaningFeeMinor,
		res.GuestFeeBPS,
		res.GuestFeeMinor,
		res.GuestTotalMinor,
		res.HostFeeBPS,
		res.HostFeeMinor,
		res.HostNetMinor,
		res.CancellationPolicy,
		res.RequestExpiresAt,
		res.DecidedAt,
	)

	created, err := scanReservation(row)
	if err != nil {
		return nil, fmt.Errorf("create reservation: %w", err)
	}
	return created, nil
}

func (r *ReservationRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.Reservation, error) {
	q := fmt.Sprintf(`
		SELECT %s
		FROM reservations
		WHERE id = $1
	`, reservationSelectCols)
	row := r.db.Conn(ctx).QueryRow(ctx, q, id)
	return scanReservation(row)
}

func (r *ReservationRepository) FindByIDForUpdate(ctx context.Context, id uuid.UUID) (*models.Reservation, error) {
	q := fmt.Sprintf(`
		SELECT %s
		FROM reservations
		WHERE id = $1
		FOR UPDATE
	`, reservationSelectCols)
	row := r.db.Conn(ctx).QueryRow(ctx, q, id)
	return scanReservation(row)
}

func (r *ReservationRepository) Update(ctx context.Context, res models.Reservation) (*models.Reservation, error) {
	q := fmt.Sprintf(`
		UPDATE reservations
		SET status = $2,
		    request_expires_at = $3,
		    decided_at = $4,
		    decline_reason = $5,
		    cancelled_at = $6,
		    cancelled_by = $7,
		    cancel_reason = $8,
		    checked_in_at = $9,
		    completed_at = $10,
		    updated_at = $11,
		    version = version + 1
		WHERE id = $1 AND version = $12
		RETURNING %s
	`, reservationSelectCols)

	row := r.db.Conn(ctx).QueryRow(
		ctx,
		q,
		res.ID,
		res.Status,
		res.RequestExpiresAt,
		res.DecidedAt,
		res.DeclineReason,
		res.CancelledAt,
		res.CancelledBy,
		res.CancelReason,
		res.CheckedInAt,
		res.CompletedAt,
		res.UpdatedAt,
		res.Version,
	)

	updated, err := scanReservation(row)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, apperrors.ErrConflict
		}
		return nil, fmt.Errorf("update reservation: %w", err)
	}
	return updated, nil
}

func (r *ReservationRepository) ListByGuest(ctx context.Context, guestID uuid.UUID) ([]models.Reservation, error) {
	q := fmt.Sprintf(`
		SELECT %s
		FROM reservations
		WHERE guest_id = $1
		ORDER BY check_in DESC, created_at DESC
	`, reservationSelectCols)
	rows, err := r.db.Conn(ctx).Query(ctx, q, guestID)
	if err != nil {
		return nil, fmt.Errorf("list reservations by guest: %w", err)
	}
	defer rows.Close()
	return scanReservations(rows)
}

func (r *ReservationRepository) ListByListingIDs(ctx context.Context, listingIDs []uuid.UUID) ([]models.Reservation, error) {
	if len(listingIDs) == 0 {
		return []models.Reservation{}, nil
	}
	q := fmt.Sprintf(`
		SELECT %s
		FROM reservations
		WHERE listing_id = ANY($1)
		ORDER BY check_in ASC, created_at DESC
	`, reservationSelectCols)
	rows, err := r.db.Conn(ctx).Query(ctx, q, listingIDs)
	if err != nil {
		return nil, fmt.Errorf("list reservations by listing_ids: %w", err)
	}
	defer rows.Close()
	return scanReservations(rows)
}

func (r *ReservationRepository) ListDuePendingExpirations(ctx context.Context, now time.Time, limit int) ([]models.Reservation, error) {
	q := fmt.Sprintf(`
		SELECT %s
		FROM reservations
		WHERE status = 'pending'
		  AND request_expires_at IS NOT NULL
		  AND request_expires_at <= $1
		ORDER BY request_expires_at ASC
		LIMIT $2
	`, reservationSelectCols)
	rows, err := r.db.Conn(ctx).Query(ctx, q, now, limit)
	if err != nil {
		return nil, fmt.Errorf("list due pending expirations: %w", err)
	}
	defer rows.Close()
	return scanReservations(rows)
}

func (r *ReservationRepository) ListDueCheckIns(ctx context.Context, now time.Time, limit int) ([]models.Reservation, error) {
	q := fmt.Sprintf(`
		SELECT %s
		FROM reservations
		WHERE status = 'confirmed'
		  AND check_in_at <= $1
		ORDER BY check_in_at ASC
		LIMIT $2
	`, reservationSelectCols)
	rows, err := r.db.Conn(ctx).Query(ctx, q, now, limit)
	if err != nil {
		return nil, fmt.Errorf("list due check-ins: %w", err)
	}
	defer rows.Close()
	return scanReservations(rows)
}

func (r *ReservationRepository) ListDueCheckOuts(ctx context.Context, now time.Time, limit int) ([]models.Reservation, error) {
	q := fmt.Sprintf(`
		SELECT %s
		FROM reservations
		WHERE status = 'checked_in'
		  AND check_out_at <= $1
		ORDER BY check_out_at ASC
		LIMIT $2
	`, reservationSelectCols)
	rows, err := r.db.Conn(ctx).Query(ctx, q, now, limit)
	if err != nil {
		return nil, fmt.Errorf("list due check-outs: %w", err)
	}
	defer rows.Close()
	return scanReservations(rows)
}
