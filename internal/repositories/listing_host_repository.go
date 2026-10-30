package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/db"
)

type ListingHostRepository struct {
	db *db.DB
}

func NewListingHostRepository(database *db.DB) interfaces.ListingHostRepository {
	return &ListingHostRepository{db: database}
}

func (r *ListingHostRepository) FindByListingAndUser(ctx context.Context, listingID, userID uuid.UUID) (*models.ListingHost, error) {
	const q = `
		SELECT id, listing_id, user_id, role,
		       can_manage_listing, can_manage_calendar, can_manage_reservations,
		       can_message_guests, can_view_insights,
		       payout_share_bps, config, invited_by, created_at, updated_at
		FROM listing_hosts
		WHERE listing_id = $1 AND user_id = $2
	`
	var lh models.ListingHost
	var configRaw []byte
	err := r.db.Conn(ctx).QueryRow(ctx, q, listingID, userID).Scan(
		&lh.ID,
		&lh.ListingID,
		&lh.UserID,
		&lh.Role,
		&lh.CanManageListing,
		&lh.CanManageCalendar,
		&lh.CanManageReservations,
		&lh.CanMessageGuests,
		&lh.CanViewInsights,
		&lh.PayoutShareBPS,
		&configRaw,
		&lh.InvitedBy,
		&lh.CreatedAt,
		&lh.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("find listing_host: %w", err)
	}
	if len(configRaw) > 0 {
		lh.Config = json.RawMessage(configRaw)
	} else {
		lh.Config = json.RawMessage(`{}`)
	}
	return &lh, nil
}

func (r *ListingHostRepository) ListByListing(ctx context.Context, listingID uuid.UUID) ([]models.ListingHost, error) {
	const q = `
		SELECT id, listing_id, user_id, role,
		       can_manage_listing, can_manage_calendar, can_manage_reservations,
		       can_message_guests, can_view_insights,
		       payout_share_bps, config, invited_by, created_at, updated_at
		FROM listing_hosts
		WHERE listing_id = $1
		ORDER BY role ASC, created_at ASC
	`
	rows, err := r.db.Conn(ctx).Query(ctx, q, listingID)
	if err != nil {
		return nil, fmt.Errorf("list listing_hosts by listing: %w", err)
	}
	defer rows.Close()

	return scanListingHosts(rows)
}

func (r *ListingHostRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]models.ListingHost, error) {
	const q = `
		SELECT id, listing_id, user_id, role,
		       can_manage_listing, can_manage_calendar, can_manage_reservations,
		       can_message_guests, can_view_insights,
		       payout_share_bps, config, invited_by, created_at, updated_at
		FROM listing_hosts
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Conn(ctx).Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list listing_hosts by user: %w", err)
	}
	defer rows.Close()

	return scanListingHosts(rows)
}

func (r *ListingHostRepository) CreateOwner(ctx context.Context, listingID, userID uuid.UUID, configBytes []byte) (*models.ListingHost, error) {
	if len(configBytes) == 0 {
		configBytes = []byte(`{}`)
	}
	const q = `
		INSERT INTO listing_hosts (
			listing_id, user_id, role,
			can_manage_listing, can_manage_calendar, can_manage_reservations,
			can_message_guests, can_view_insights,
			payout_share_bps, config
		) VALUES (
			$1, $2, 'owner',
			true, true, true,
			true, true,
			10000, $3
		)
		RETURNING id, listing_id, user_id, role,
		          can_manage_listing, can_manage_calendar, can_manage_reservations,
		          can_message_guests, can_view_insights,
		          payout_share_bps, config, invited_by, created_at, updated_at
	`
	var lh models.ListingHost
	var configRaw []byte
	err := r.db.Conn(ctx).QueryRow(ctx, q, listingID, userID, configBytes).Scan(
		&lh.ID,
		&lh.ListingID,
		&lh.UserID,
		&lh.Role,
		&lh.CanManageListing,
		&lh.CanManageCalendar,
		&lh.CanManageReservations,
		&lh.CanMessageGuests,
		&lh.CanViewInsights,
		&lh.PayoutShareBPS,
		&configRaw,
		&lh.InvitedBy,
		&lh.CreatedAt,
		&lh.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert owner listing_host: %w", err)
	}
	lh.Config = json.RawMessage(configRaw)
	return &lh, nil
}

func (r *ListingHostRepository) UpdateConfig(ctx context.Context, listingID, userID uuid.UUID, configBytes []byte) (*models.ListingHost, error) {
	const q = `
		UPDATE listing_hosts
		SET config = $3, updated_at = now()
		WHERE listing_id = $1 AND user_id = $2
		RETURNING id, listing_id, user_id, role,
		          can_manage_listing, can_manage_calendar, can_manage_reservations,
		          can_message_guests, can_view_insights,
		          payout_share_bps, config, invited_by, created_at, updated_at
	`
	var lh models.ListingHost
	var configRaw []byte
	err := r.db.Conn(ctx).QueryRow(ctx, q, listingID, userID, configBytes).Scan(
		&lh.ID,
		&lh.ListingID,
		&lh.UserID,
		&lh.Role,
		&lh.CanManageListing,
		&lh.CanManageCalendar,
		&lh.CanManageReservations,
		&lh.CanMessageGuests,
		&lh.CanViewInsights,
		&lh.PayoutShareBPS,
		&configRaw,
		&lh.InvitedBy,
		&lh.CreatedAt,
		&lh.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("update listing_host config: %w", err)
	}
	lh.Config = json.RawMessage(configRaw)
	return &lh, nil
}

func scanListingHosts(rows pgx.Rows) ([]models.ListingHost, error) {
	out := make([]models.ListingHost, 0)
	for rows.Next() {
		var lh models.ListingHost
		var configRaw []byte
		if err := rows.Scan(
			&lh.ID,
			&lh.ListingID,
			&lh.UserID,
			&lh.Role,
			&lh.CanManageListing,
			&lh.CanManageCalendar,
			&lh.CanManageReservations,
			&lh.CanMessageGuests,
			&lh.CanViewInsights,
			&lh.PayoutShareBPS,
			&configRaw,
			&lh.InvitedBy,
			&lh.CreatedAt,
			&lh.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan listing_host: %w", err)
		}
		if len(configRaw) > 0 {
			lh.Config = json.RawMessage(configRaw)
		} else {
			lh.Config = json.RawMessage(`{}`)
		}
		out = append(out, lh)
	}
	return out, rows.Err()
}
