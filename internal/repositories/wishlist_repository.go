package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/db"
)

type WishlistRepository struct {
	db *db.DB
}

func NewWishlistRepository(db *db.DB) interfaces.WishlistRepository {
	return &WishlistRepository{db: db}
}

func (r *WishlistRepository) Create(ctx context.Context, w *models.Wishlist) error {
	q := `
		INSERT INTO wishlists (id, user_id, name, is_shared, share_token, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Conn(ctx).Exec(ctx, q,
		w.ID, w.UserID, w.Name, w.IsShared, w.ShareToken, w.CreatedAt, w.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create wishlist: %w", err)
	}
	return nil
}

func (r *WishlistRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.Wishlist, error) {
	q := `
		SELECT id, user_id, name, is_shared, share_token, created_at, updated_at
		FROM wishlists
		WHERE id = $1
	`
	var w models.Wishlist
	err := r.db.Conn(ctx).QueryRow(ctx, q, id).Scan(
		&w.ID, &w.UserID, &w.Name, &w.IsShared, &w.ShareToken, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("find wishlist by id: %w", err)
	}
	return &w, nil
}

func (r *WishlistRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]models.Wishlist, error) {
	q := `
		SELECT id, user_id, name, is_shared, share_token, created_at, updated_at
		FROM wishlists
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Conn(ctx).Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("find wishlists by user: %w", err)
	}
	defer rows.Close()

	var out []models.Wishlist
	for rows.Next() {
		var w models.Wishlist
		if err := rows.Scan(&w.ID, &w.UserID, &w.Name, &w.IsShared, &w.ShareToken, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan wishlist: %w", err)
		}
		out = append(out, w)
	}
	return out, nil
}

func (r *WishlistRepository) FindByShareToken(ctx context.Context, token string) (*models.Wishlist, error) {
	q := `
		SELECT id, user_id, name, is_shared, share_token, created_at, updated_at
		FROM wishlists
		WHERE share_token = $1 AND is_shared = true
	`
	var w models.Wishlist
	err := r.db.Conn(ctx).QueryRow(ctx, q, token).Scan(
		&w.ID, &w.UserID, &w.Name, &w.IsShared, &w.ShareToken, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("find wishlist by share token: %w", err)
	}
	return &w, nil
}

func (r *WishlistRepository) Update(ctx context.Context, w *models.Wishlist) error {
	q := `
		UPDATE wishlists
		SET name = $1, is_shared = $2, share_token = $3, updated_at = $4
		WHERE id = $5
	`
	res, err := r.db.Conn(ctx).Exec(ctx, q, w.Name, w.IsShared, w.ShareToken, w.UpdatedAt, w.ID)
	if err != nil {
		return fmt.Errorf("update wishlist: %w", err)
	}
	if res.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *WishlistRepository) Delete(ctx context.Context, id uuid.UUID) error {
	q := `DELETE FROM wishlists WHERE id = $1`
	res, err := r.db.Conn(ctx).Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete wishlist: %w", err)
	}
	if res.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *WishlistRepository) AddItem(ctx context.Context, wishlistID, listingID uuid.UUID) error {
	q := `
		INSERT INTO wishlist_items (wishlist_id, listing_id)
		VALUES ($1, $2)
		ON CONFLICT (wishlist_id, listing_id) DO NOTHING
	`
	_, err := r.db.Conn(ctx).Exec(ctx, q, wishlistID, listingID)
	if err != nil {
		return fmt.Errorf("add wishlist item: %w", err)
	}
	return nil
}

func (r *WishlistRepository) RemoveItem(ctx context.Context, wishlistID, listingID uuid.UUID) error {
	q := `
		DELETE FROM wishlist_items
		WHERE wishlist_id = $1 AND listing_id = $2
	`
	_, err := r.db.Conn(ctx).Exec(ctx, q, wishlistID, listingID)
	if err != nil {
		return fmt.Errorf("remove wishlist item: %w", err)
	}
	return nil
}

func (r *WishlistRepository) GetItemListingIDs(ctx context.Context, wishlistID uuid.UUID) ([]uuid.UUID, error) {
	q := `
		SELECT listing_id
		FROM wishlist_items
		WHERE wishlist_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Conn(ctx).Query(ctx, q, wishlistID)
	if err != nil {
		return nil, fmt.Errorf("get wishlist items: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan listing id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (r *WishlistRepository) GetItemCountsByUser(ctx context.Context, userID uuid.UUID) (map[uuid.UUID]int, error) {
	q := `
		SELECT w.id, COUNT(wi.id)
		FROM wishlists w
		LEFT JOIN wishlist_items wi ON wi.wishlist_id = w.id
		WHERE w.user_id = $1
		GROUP BY w.id
	`
	rows, err := r.db.Conn(ctx).Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("get item counts by user: %w", err)
	}
	defer rows.Close()

	counts := make(map[uuid.UUID]int)
	for rows.Next() {
		var wID uuid.UUID
		var count int
		if err := rows.Scan(&wID, &count); err != nil {
			return nil, fmt.Errorf("scan wishlist count: %w", err)
		}
		counts[wID] = count
	}
	return counts, nil
}
