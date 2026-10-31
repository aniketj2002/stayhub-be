package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/db"
)

type CohostRepository struct {
	db *db.DB
}

func NewCohostRepository(database *db.DB) interfaces.CohostRepository {
	return &CohostRepository{db: database}
}

func (r *CohostRepository) CreateInvitation(ctx context.Context, inv models.CohostInvitation) (*models.CohostInvitation, error) {
	const q = `
		INSERT INTO cohost_invitations (
			listing_id, inviter_user_id, email, token_hash, permissions,
			payout_share_bps, status, expires_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, now(), now()
		)
		RETURNING id, listing_id, inviter_user_id, email, token_hash, permissions,
		          payout_share_bps, status, expires_at, created_at, updated_at
	`
	var out models.CohostInvitation
	err := r.db.Conn(ctx).QueryRow(
		ctx, q,
		inv.ListingID, inv.InviterUserID, inv.Email, inv.TokenHash, inv.Permissions,
		inv.PayoutShareBPS, inv.Status, inv.ExpiresAt,
	).Scan(
		&out.ID,
		&out.ListingID,
		&out.InviterUserID,
		&out.Email,
		&out.TokenHash,
		&out.Permissions,
		&out.PayoutShareBPS,
		&out.Status,
		&out.ExpiresAt,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create cohost invitation: %w", err)
	}
	return &out, nil
}

func (r *CohostRepository) FindInvitationByTokenHash(ctx context.Context, tokenHash string) (*models.CohostInvitation, error) {
	const q = `
		SELECT id, listing_id, inviter_user_id, email, token_hash, permissions,
		       payout_share_bps, status, expires_at, created_at, updated_at
		FROM cohost_invitations
		WHERE token_hash = $1
	`
	var out models.CohostInvitation
	err := r.db.Conn(ctx).QueryRow(ctx, q, tokenHash).Scan(
		&out.ID,
		&out.ListingID,
		&out.InviterUserID,
		&out.Email,
		&out.TokenHash,
		&out.Permissions,
		&out.PayoutShareBPS,
		&out.Status,
		&out.ExpiresAt,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("find invitation by token hash: %w", err)
	}
	return &out, nil
}

func (r *CohostRepository) FindInvitationByID(ctx context.Context, id uuid.UUID) (*models.CohostInvitation, error) {
	const q = `
		SELECT id, listing_id, inviter_user_id, email, token_hash, permissions,
		       payout_share_bps, status, expires_at, created_at, updated_at
		FROM cohost_invitations
		WHERE id = $1
	`
	var out models.CohostInvitation
	err := r.db.Conn(ctx).QueryRow(ctx, q, id).Scan(
		&out.ID,
		&out.ListingID,
		&out.InviterUserID,
		&out.Email,
		&out.TokenHash,
		&out.Permissions,
		&out.PayoutShareBPS,
		&out.Status,
		&out.ExpiresAt,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("find invitation by id: %w", err)
	}
	return &out, nil
}

func (r *CohostRepository) ListInvitationsByListing(ctx context.Context, listingID uuid.UUID) ([]models.CohostInvitation, error) {
	const q = `
		SELECT id, listing_id, inviter_user_id, email, token_hash, permissions,
		       payout_share_bps, status, expires_at, created_at, updated_at
		FROM cohost_invitations
		WHERE listing_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Conn(ctx).Query(ctx, q, listingID)
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	defer rows.Close()

	var list []models.CohostInvitation
	for rows.Next() {
		var inv models.CohostInvitation
		if err := rows.Scan(
			&inv.ID,
			&inv.ListingID,
			&inv.InviterUserID,
			&inv.Email,
			&inv.TokenHash,
			&inv.Permissions,
			&inv.PayoutShareBPS,
			&inv.Status,
			&inv.ExpiresAt,
			&inv.CreatedAt,
			&inv.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan invitation: %w", err)
		}
		list = append(list, inv)
	}
	return list, rows.Err()
}

func (r *CohostRepository) UpdateInvitationStatus(ctx context.Context, id uuid.UUID, status enums.CohostInviteStatus, now time.Time) error {
	const q = `
		UPDATE cohost_invitations
		SET status = $2, updated_at = $3
		WHERE id = $1
	`
	cmd, err := r.db.Conn(ctx).Exec(ctx, q, id, status, now)
	if err != nil {
		return fmt.Errorf("update invitation status: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *CohostRepository) CreatePayoutSplits(ctx context.Context, splits []models.PayoutSplit) error {
	if len(splits) == 0 {
		return nil
	}
	const q = `
		INSERT INTO payout_splits (
			payout_id, user_id, share_bps, amount_minor, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, now(), now())
		ON CONFLICT (payout_id, user_id) DO UPDATE
		SET share_bps = EXCLUDED.share_bps,
		    amount_minor = EXCLUDED.amount_minor,
		    status = EXCLUDED.status,
		    updated_at = now()
	`
	b := &pgx.Batch{}
	for _, s := range splits {
		b.Queue(q, s.PayoutID, s.UserID, s.ShareBPS, s.AmountMinor, s.Status)
	}
	br := r.db.Conn(ctx).SendBatch(ctx, b)
	defer br.Close()

	for range splits {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("batch insert payout split: %w", err)
		}
	}
	return nil
}

func (r *CohostRepository) ListPayoutSplits(ctx context.Context, payoutID uuid.UUID) ([]models.PayoutSplit, error) {
	const q = `
		SELECT id, payout_id, user_id, share_bps, amount_minor, status, created_at, updated_at
		FROM payout_splits
		WHERE payout_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.Conn(ctx).Query(ctx, q, payoutID)
	if err != nil {
		return nil, fmt.Errorf("list payout splits: %w", err)
	}
	defer rows.Close()

	var list []models.PayoutSplit
	for rows.Next() {
		var s models.PayoutSplit
		if err := rows.Scan(
			&s.ID,
			&s.PayoutID,
			&s.UserID,
			&s.ShareBPS,
			&s.AmountMinor,
			&s.Status,
			&s.CreatedAt,
			&s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan payout split: %w", err)
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

func (r *CohostRepository) UpdatePayoutSplitsStatus(ctx context.Context, payoutID uuid.UUID, status enums.PayoutSplitStatus, now time.Time) error {
	const q = `
		UPDATE payout_splits
		SET status = $2, updated_at = $3
		WHERE payout_id = $1
	`
	_, err := r.db.Conn(ctx).Exec(ctx, q, payoutID, status, now)
	if err != nil {
		return fmt.Errorf("update payout splits status: %w", err)
	}
	return nil
}
