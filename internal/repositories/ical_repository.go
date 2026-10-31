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

type ICalRepository struct {
	db *db.DB
}

func NewICalRepository(database *db.DB) interfaces.ICalRepository {
	return &ICalRepository{db: database}
}

func (r *ICalRepository) CreateFeed(ctx context.Context, feed models.ICalFeed) (*models.ICalFeed, error) {
	const q = `
		INSERT INTO ical_feeds (listing_id, name, url, created_at, updated_at)
		VALUES ($1, $2, $3, now(), now())
		RETURNING id, listing_id, name, url, last_synced_at, last_sync_error, created_at, updated_at
	`
	var out models.ICalFeed
	err := r.db.Conn(ctx).QueryRow(ctx, q, feed.ListingID, feed.Name, feed.URL).Scan(
		&out.ID,
		&out.ListingID,
		&out.Name,
		&out.URL,
		&out.LastSyncedAt,
		&out.LastSyncError,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create ical feed: %w", err)
	}
	return &out, nil
}

func (r *ICalRepository) GetFeedByID(ctx context.Context, feedID uuid.UUID) (*models.ICalFeed, error) {
	const q = `
		SELECT id, listing_id, name, url, last_synced_at, last_sync_error, created_at, updated_at
		FROM ical_feeds
		WHERE id = $1
	`
	var out models.ICalFeed
	err := r.db.Conn(ctx).QueryRow(ctx, q, feedID).Scan(
		&out.ID,
		&out.ListingID,
		&out.Name,
		&out.URL,
		&out.LastSyncedAt,
		&out.LastSyncError,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("get ical feed: %w", err)
	}
	return &out, nil
}

func (r *ICalRepository) ListFeedsByListing(ctx context.Context, listingID uuid.UUID) ([]models.ICalFeed, error) {
	const q = `
		SELECT id, listing_id, name, url, last_synced_at, last_sync_error, created_at, updated_at
		FROM ical_feeds
		WHERE listing_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.Conn(ctx).Query(ctx, q, listingID)
	if err != nil {
		return nil, fmt.Errorf("list ical feeds: %w", err)
	}
	defer rows.Close()

	var list []models.ICalFeed
	for rows.Next() {
		var f models.ICalFeed
		if err := rows.Scan(
			&f.ID,
			&f.ListingID,
			&f.Name,
			&f.URL,
			&f.LastSyncedAt,
			&f.LastSyncError,
			&f.CreatedAt,
			&f.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ical feed: %w", err)
		}
		list = append(list, f)
	}
	return list, rows.Err()
}

func (r *ICalRepository) DeleteFeed(ctx context.Context, feedID uuid.UUID) error {
	const q = `DELETE FROM ical_feeds WHERE id = $1`
	cmd, err := r.db.Conn(ctx).Exec(ctx, q, feedID)
	if err != nil {
		return fmt.Errorf("delete ical feed: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *ICalRepository) ListAllFeeds(ctx context.Context) ([]models.ICalFeed, error) {
	const q = `
		SELECT id, listing_id, name, url, last_synced_at, last_sync_error, created_at, updated_at
		FROM ical_feeds
		ORDER BY last_synced_at ASC NULLS FIRST
	`
	rows, err := r.db.Conn(ctx).Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list all ical feeds: %w", err)
	}
	defer rows.Close()

	var list []models.ICalFeed
	for rows.Next() {
		var f models.ICalFeed
		if err := rows.Scan(
			&f.ID,
			&f.ListingID,
			&f.Name,
			&f.URL,
			&f.LastSyncedAt,
			&f.LastSyncError,
			&f.CreatedAt,
			&f.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ical feed: %w", err)
		}
		list = append(list, f)
	}
	return list, rows.Err()
}

func (r *ICalRepository) UpdateFeedSyncStatus(ctx context.Context, feedID uuid.UUID, lastSyncedAt time.Time, syncError *string) error {
	const q = `
		UPDATE ical_feeds
		SET last_synced_at = $2,
		    last_sync_error = $3,
		    updated_at = now()
		WHERE id = $1
	`
	_, err := r.db.Conn(ctx).Exec(ctx, q, feedID, lastSyncedAt, syncError)
	if err != nil {
		return fmt.Errorf("update feed sync status: %w", err)
	}
	return nil
}

func (r *ICalRepository) GetListingByExportToken(ctx context.Context, token string) (*models.Listing, error) {
	const q = `
		SELECT id, host_id, status, live_version_id, working_version_id, ical_export_token, created_at, updated_at
		FROM listings
		WHERE ical_export_token = $1
	`
	var l models.Listing
	err := r.db.Conn(ctx).QueryRow(ctx, q, token).Scan(
		&l.ID,
		&l.HostID,
		&l.Status,
		&l.LiveVersionID,
		&l.WorkingVersionID,
		&l.ICalExportToken,
		&l.CreatedAt,
		&l.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("get listing by export token: %w", err)
	}
	return &l, nil
}

func (r *ICalRepository) SetExportToken(ctx context.Context, listingID uuid.UUID, token string) error {
	const q = `
		UPDATE listings
		SET ical_export_token = $2, updated_at = now()
		WHERE id = $1
	`
	_, err := r.db.Conn(ctx).Exec(ctx, q, listingID, token)
	if err != nil {
		return fmt.Errorf("set export token: %w", err)
	}
	return nil
}
