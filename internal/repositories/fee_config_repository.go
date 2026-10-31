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

type FeeConfigRepository struct {
	db *db.DB
}

func NewFeeConfigRepository(database *db.DB) interfaces.FeeConfigRepository {
	return &FeeConfigRepository{db: database}
}

const feeConfigSelectCols = `
	id, guest_fee_bps, host_fee_bps, status,
	proposed_by, proposed_reason, approved_by, approved_at,
	rejection_reason, created_at, updated_at
`

func scanFeeConfig(row pgx.Row) (*models.FeeConfig, error) {
	var fc models.FeeConfig
	err := row.Scan(
		&fc.ID,
		&fc.GuestFeeBPS,
		&fc.HostFeeBPS,
		&fc.Status,
		&fc.ProposedBy,
		&fc.ProposedReason,
		&fc.ApprovedBy,
		&fc.ApprovedAt,
		&fc.RejectionReason,
		&fc.CreatedAt,
		&fc.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &fc, nil
}

func (r *FeeConfigRepository) GetActive(ctx context.Context) (*models.FeeConfig, error) {
	q := fmt.Sprintf(`SELECT %s FROM fee_configs WHERE status = 'active' LIMIT 1`, feeConfigSelectCols)
	row := r.db.Conn(ctx).QueryRow(ctx, q)
	return scanFeeConfig(row)
}

func (r *FeeConfigRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.FeeConfig, error) {
	q := fmt.Sprintf(`SELECT %s FROM fee_configs WHERE id = $1`, feeConfigSelectCols)
	row := r.db.Conn(ctx).QueryRow(ctx, q, id)
	return scanFeeConfig(row)
}

func (r *FeeConfigRepository) FindByIDForUpdate(ctx context.Context, id uuid.UUID) (*models.FeeConfig, error) {
	q := fmt.Sprintf(`SELECT %s FROM fee_configs WHERE id = $1 FOR UPDATE`, feeConfigSelectCols)
	row := r.db.Conn(ctx).QueryRow(ctx, q, id)
	return scanFeeConfig(row)
}

func (r *FeeConfigRepository) List(ctx context.Context) ([]models.FeeConfig, error) {
	q := fmt.Sprintf(`SELECT %s FROM fee_configs ORDER BY created_at DESC`, feeConfigSelectCols)
	rows, err := r.db.Conn(ctx).Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list fee_configs: %w", err)
	}
	defer rows.Close()

	var out []models.FeeConfig
	for rows.Next() {
		fc, err := scanFeeConfig(rows)
		if err != nil {
			return nil, fmt.Errorf("scan fee_config: %w", err)
		}
		out = append(out, *fc)
	}
	return out, rows.Err()
}

func (r *FeeConfigRepository) Create(ctx context.Context, cfg models.FeeConfig) (*models.FeeConfig, error) {
	q := fmt.Sprintf(`
		INSERT INTO fee_configs (
			id, guest_fee_bps, host_fee_bps, status,
			proposed_by, proposed_reason, approved_by, approved_at,
			rejection_reason, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING %s
	`, feeConfigSelectCols)
	row := r.db.Conn(ctx).QueryRow(
		ctx,
		q,
		cfg.ID,
		cfg.GuestFeeBPS,
		cfg.HostFeeBPS,
		cfg.Status,
		cfg.ProposedBy,
		cfg.ProposedReason,
		cfg.ApprovedBy,
		cfg.ApprovedAt,
		cfg.RejectionReason,
		cfg.CreatedAt,
		cfg.UpdatedAt,
	)
	return scanFeeConfig(row)
}

func (r *FeeConfigRepository) Update(ctx context.Context, cfg models.FeeConfig) (*models.FeeConfig, error) {
	q := fmt.Sprintf(`
		UPDATE fee_configs
		SET guest_fee_bps = $2,
		    host_fee_bps = $3,
		    status = $4,
		    approved_by = $5,
		    approved_at = $6,
		    rejection_reason = $7,
		    updated_at = $8
		WHERE id = $1
		RETURNING %s
	`, feeConfigSelectCols)
	row := r.db.Conn(ctx).QueryRow(
		ctx,
		q,
		cfg.ID,
		cfg.GuestFeeBPS,
		cfg.HostFeeBPS,
		cfg.Status,
		cfg.ApprovedBy,
		cfg.ApprovedAt,
		cfg.RejectionReason,
		cfg.UpdatedAt,
	)
	return scanFeeConfig(row)
}

func (r *FeeConfigRepository) Activate(
	ctx context.Context,
	id uuid.UUID,
	approverID uuid.UUID,
	now time.Time,
) (*models.FeeConfig, error) {
	var activated *models.FeeConfig
	err := r.db.WithinTx(ctx, func(txCtx context.Context) error {
		// Supersede currently active fee configs
		_, err := r.db.Conn(txCtx).Exec(txCtx, `
			UPDATE fee_configs
			SET status = $1, updated_at = $2
			WHERE status = $3
		`, enums.FeeConfigStatusSuperseded, now, enums.FeeConfigStatusActive)
		if err != nil {
			return fmt.Errorf("supersede active fee configs: %w", err)
		}

		// Activate target fee config
		q := fmt.Sprintf(`
			UPDATE fee_configs
			SET status = $2,
			    approved_by = $3,
			    approved_at = $4,
			    updated_at = $4
			WHERE id = $1
			RETURNING %s
		`, feeConfigSelectCols)
		row := r.db.Conn(txCtx).QueryRow(txCtx, q, id, enums.FeeConfigStatusActive, approverID, now)
		activated, err = scanFeeConfig(row)
		if err != nil {
			return fmt.Errorf("activate fee config: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return activated, nil
}
