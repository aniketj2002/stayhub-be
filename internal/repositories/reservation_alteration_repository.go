package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/db"
)

type ReservationAlterationRepository struct {
	db *db.DB
}

func NewReservationAlterationRepository(database *db.DB) interfaces.ReservationAlterationRepository {
	return &ReservationAlterationRepository{db: database}
}

func (r *ReservationAlterationRepository) CreateAlteration(ctx context.Context, alt models.ReservationAlteration) (*models.ReservationAlteration, error) {
	if alt.ID == uuid.Nil {
		alt.ID = uuid.New()
	}

	query := `
		INSERT INTO reservation_alterations (
			id, reservation_id, requested_by, current_check_in, current_check_out,
			new_check_in, new_check_out, original_snapshot, requested_changes,
			delta_minor, currency, status, expires_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13, now(), now()
		)
		RETURNING id, reservation_id, requested_by, current_check_in, current_check_out,
		          new_check_in, new_check_out, original_snapshot, requested_changes,
		          delta_minor, currency, status, expires_at, decided_at, decision_reason,
		          created_at, updated_at;
	`
	q := r.db.Conn(ctx)
	var out models.ReservationAlteration
	err := q.QueryRow(
		ctx,
		query,
		alt.ID,
		alt.ReservationID,
		alt.RequestedBy,
		alt.CurrentCheckIn,
		alt.CurrentCheckOut,
		alt.NewCheckIn,
		alt.NewCheckOut,
		alt.OriginalSnapshot,
		alt.RequestedChanges,
		alt.DeltaMinor,
		alt.Currency,
		alt.Status,
		alt.ExpiresAt,
	).Scan(
		&out.ID,
		&out.ReservationID,
		&out.RequestedBy,
		&out.CurrentCheckIn,
		&out.CurrentCheckOut,
		&out.NewCheckIn,
		&out.NewCheckOut,
		&out.OriginalSnapshot,
		&out.RequestedChanges,
		&out.DeltaMinor,
		&out.Currency,
		&out.Status,
		&out.ExpiresAt,
		&out.DecidedAt,
		&out.DecisionReason,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *ReservationAlterationRepository) FindAlterationByID(ctx context.Context, id uuid.UUID) (*models.ReservationAlteration, error) {
	query := `
		SELECT id, reservation_id, requested_by, current_check_in, current_check_out,
		       new_check_in, new_check_out, original_snapshot, requested_changes,
		       delta_minor, currency, status, expires_at, decided_at, decision_reason,
		       created_at, updated_at
		FROM reservation_alterations
		WHERE id = $1;
	`
	q := r.db.Conn(ctx)
	var out models.ReservationAlteration
	err := q.QueryRow(ctx, query, id).Scan(
		&out.ID,
		&out.ReservationID,
		&out.RequestedBy,
		&out.CurrentCheckIn,
		&out.CurrentCheckOut,
		&out.NewCheckIn,
		&out.NewCheckOut,
		&out.OriginalSnapshot,
		&out.RequestedChanges,
		&out.DeltaMinor,
		&out.Currency,
		&out.Status,
		&out.ExpiresAt,
		&out.DecidedAt,
		&out.DecisionReason,
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

func (r *ReservationAlterationRepository) FindAlterationByIDForUpdate(ctx context.Context, id uuid.UUID) (*models.ReservationAlteration, error) {
	query := `
		SELECT id, reservation_id, requested_by, current_check_in, current_check_out,
		       new_check_in, new_check_out, original_snapshot, requested_changes,
		       delta_minor, currency, status, expires_at, decided_at, decision_reason,
		       created_at, updated_at
		FROM reservation_alterations
		WHERE id = $1
		FOR UPDATE;
	`
	q := r.db.Conn(ctx)
	var out models.ReservationAlteration
	err := q.QueryRow(ctx, query, id).Scan(
		&out.ID,
		&out.ReservationID,
		&out.RequestedBy,
		&out.CurrentCheckIn,
		&out.CurrentCheckOut,
		&out.NewCheckIn,
		&out.NewCheckOut,
		&out.OriginalSnapshot,
		&out.RequestedChanges,
		&out.DeltaMinor,
		&out.Currency,
		&out.Status,
		&out.ExpiresAt,
		&out.DecidedAt,
		&out.DecisionReason,
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

func (r *ReservationAlterationRepository) ListAlterationsByReservation(ctx context.Context, reservationID uuid.UUID) ([]models.ReservationAlteration, error) {
	query := `
		SELECT id, reservation_id, requested_by, current_check_in, current_check_out,
		       new_check_in, new_check_out, original_snapshot, requested_changes,
		       delta_minor, currency, status, expires_at, decided_at, decision_reason,
		       created_at, updated_at
		FROM reservation_alterations
		WHERE reservation_id = $1
		ORDER BY created_at DESC;
	`
	q := r.db.Conn(ctx)
	rows, err := q.Query(ctx, query, reservationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.ReservationAlteration
	for rows.Next() {
		var alt models.ReservationAlteration
		if err := rows.Scan(
			&alt.ID,
			&alt.ReservationID,
			&alt.RequestedBy,
			&alt.CurrentCheckIn,
			&alt.CurrentCheckOut,
			&alt.NewCheckIn,
			&alt.NewCheckOut,
			&alt.OriginalSnapshot,
			&alt.RequestedChanges,
			&alt.DeltaMinor,
			&alt.Currency,
			&alt.Status,
			&alt.ExpiresAt,
			&alt.DecidedAt,
			&alt.DecisionReason,
			&alt.CreatedAt,
			&alt.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, alt)
	}
	return out, rows.Err()
}

func (r *ReservationAlterationRepository) FindPendingByReservation(ctx context.Context, reservationID uuid.UUID) (*models.ReservationAlteration, error) {
	query := `
		SELECT id, reservation_id, requested_by, current_check_in, current_check_out,
		       new_check_in, new_check_out, original_snapshot, requested_changes,
		       delta_minor, currency, status, expires_at, decided_at, decision_reason,
		       created_at, updated_at
		FROM reservation_alterations
		WHERE reservation_id = $1 AND status = 'pending'
		LIMIT 1;
	`
	q := r.db.Conn(ctx)
	var out models.ReservationAlteration
	err := q.QueryRow(ctx, query, reservationID).Scan(
		&out.ID,
		&out.ReservationID,
		&out.RequestedBy,
		&out.CurrentCheckIn,
		&out.CurrentCheckOut,
		&out.NewCheckIn,
		&out.NewCheckOut,
		&out.OriginalSnapshot,
		&out.RequestedChanges,
		&out.DeltaMinor,
		&out.Currency,
		&out.Status,
		&out.ExpiresAt,
		&out.DecidedAt,
		&out.DecisionReason,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}

func (r *ReservationAlterationRepository) ListPendingByReservationIDs(ctx context.Context, reservationIDs []uuid.UUID) ([]models.ReservationAlteration, error) {
	if len(reservationIDs) == 0 {
		return []models.ReservationAlteration{}, nil
	}
	query := `
		SELECT id, reservation_id, requested_by, current_check_in, current_check_out,
		       new_check_in, new_check_out, original_snapshot, requested_changes,
		       delta_minor, currency, status, expires_at, decided_at, decision_reason,
		       created_at, updated_at
		FROM reservation_alterations
		WHERE status = 'pending' AND reservation_id = ANY($1)
		ORDER BY expires_at ASC;
	`
	q := r.db.Conn(ctx)
	rows, err := q.Query(ctx, query, reservationIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.ReservationAlteration, 0)
	for rows.Next() {
		var alt models.ReservationAlteration
		if err := rows.Scan(
			&alt.ID,
			&alt.ReservationID,
			&alt.RequestedBy,
			&alt.CurrentCheckIn,
			&alt.CurrentCheckOut,
			&alt.NewCheckIn,
			&alt.NewCheckOut,
			&alt.OriginalSnapshot,
			&alt.RequestedChanges,
			&alt.DeltaMinor,
			&alt.Currency,
			&alt.Status,
			&alt.ExpiresAt,
			&alt.DecidedAt,
			&alt.DecisionReason,
			&alt.CreatedAt,
			&alt.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, alt)
	}
	return out, rows.Err()
}

func (r *ReservationAlterationRepository) UpdateAlterationStatus(
	ctx context.Context,
	id uuid.UUID,
	status enums.AlterationStatus,
	decidedAt *time.Time,
	reason *string,
) error {
	query := `
		UPDATE reservation_alterations
		SET status = $1, decided_at = $2, decision_reason = $3, updated_at = now()
		WHERE id = $4;
	`
	q := r.db.Conn(ctx)
	_, err := q.Exec(ctx, query, status, decidedAt, reason, id)
	return err
}

func (r *ReservationAlterationRepository) ListExpiredPending(ctx context.Context, now time.Time) ([]models.ReservationAlteration, error) {
	query := `
		SELECT id, reservation_id, requested_by, current_check_in, current_check_out,
		       new_check_in, new_check_out, original_snapshot, requested_changes,
		       delta_minor, currency, status, expires_at, decided_at, decision_reason,
		       created_at, updated_at
		FROM reservation_alterations
		WHERE status = 'pending' AND expires_at <= $1;
	`
	q := r.db.Conn(ctx)
	rows, err := q.Query(ctx, query, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.ReservationAlteration
	for rows.Next() {
		var alt models.ReservationAlteration
		if err := rows.Scan(
			&alt.ID,
			&alt.ReservationID,
			&alt.RequestedBy,
			&alt.CurrentCheckIn,
			&alt.CurrentCheckOut,
			&alt.NewCheckIn,
			&alt.NewCheckOut,
			&alt.OriginalSnapshot,
			&alt.RequestedChanges,
			&alt.DeltaMinor,
			&alt.Currency,
			&alt.Status,
			&alt.ExpiresAt,
			&alt.DecidedAt,
			&alt.DecisionReason,
			&alt.CreatedAt,
			&alt.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, alt)
	}
	return out, rows.Err()
}
