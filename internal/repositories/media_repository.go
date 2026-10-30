package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/db"
)

type MediaRepository struct {
	db *db.DB
}

func NewMediaRepository(database *db.DB) interfaces.MediaRepository {
	return &MediaRepository{db: database}
}

func (r *MediaRepository) CreatePending(ctx context.Context, p dtos.CreateMediaParams) (*models.Media, error) {
	const q = `
		INSERT INTO media (id, s3_key, content_type, size_bytes, width, height, status, uploaded_by)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending', $7)
		RETURNING id, s3_key, content_type, size_bytes, width, height, status, uploaded_by, created_at
	`
	var m models.Media
	err := r.db.Conn(ctx).QueryRow(
		ctx,
		q,
		p.ID,
		p.S3Key,
		p.ContentType,
		p.SizeBytes,
		p.Width,
		p.Height,
		p.UploadedBy,
	).Scan(
		&m.ID,
		&m.S3Key,
		&m.ContentType,
		&m.SizeBytes,
		&m.Width,
		&m.Height,
		&m.Status,
		&m.UploadedBy,
		&m.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create pending media: %w", err)
	}
	return &m, nil
}

func (r *MediaRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.Media, error) {
	const q = `
		SELECT id, s3_key, content_type, size_bytes, width, height, status, uploaded_by, created_at
		FROM media
		WHERE id = $1
	`
	var m models.Media
	err := r.db.Conn(ctx).QueryRow(ctx, q, id).Scan(
		&m.ID,
		&m.S3Key,
		&m.ContentType,
		&m.SizeBytes,
		&m.Width,
		&m.Height,
		&m.Status,
		&m.UploadedBy,
		&m.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("find media by id: %w", err)
	}
	return &m, nil
}

func (r *MediaRepository) FindByIDs(ctx context.Context, ids []uuid.UUID) ([]models.Media, error) {
	if len(ids) == 0 {
		return []models.Media{}, nil
	}
	const q = `
		SELECT id, s3_key, content_type, size_bytes, width, height, status, uploaded_by, created_at
		FROM media
		WHERE id = ANY($1)
	`
	rows, err := r.db.Conn(ctx).Query(ctx, q, ids)
	if err != nil {
		return nil, fmt.Errorf("find media by ids: %w", err)
	}
	defer rows.Close()

	out := make([]models.Media, 0, len(ids))
	for rows.Next() {
		var m models.Media
		if err := rows.Scan(
			&m.ID,
			&m.S3Key,
			&m.ContentType,
			&m.SizeBytes,
			&m.Width,
			&m.Height,
			&m.Status,
			&m.UploadedBy,
			&m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan media row: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *MediaRepository) ConfirmUpload(ctx context.Context, id uuid.UUID, width, height *int) (*models.Media, error) {
	const q = `
		UPDATE media
		SET status = 'uploaded',
		    width = COALESCE($2, width),
		    height = COALESCE($3, height)
		WHERE id = $1
		RETURNING id, s3_key, content_type, size_bytes, width, height, status, uploaded_by, created_at
	`
	var m models.Media
	err := r.db.Conn(ctx).QueryRow(ctx, q, id, width, height).Scan(
		&m.ID,
		&m.S3Key,
		&m.ContentType,
		&m.SizeBytes,
		&m.Width,
		&m.Height,
		&m.Status,
		&m.UploadedBy,
		&m.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("confirm media upload: %w", err)
	}
	return &m, nil
}
