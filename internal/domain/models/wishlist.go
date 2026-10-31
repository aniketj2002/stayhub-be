package models

import (
	"time"

	"github.com/google/uuid"
)

// Wishlist maps to the `wishlists` table.
type Wishlist struct {
	ID         uuid.UUID  `db:"id" json:"id"`
	UserID     uuid.UUID  `db:"user_id" json:"user_id"`
	Name       string     `db:"name" json:"name"`
	IsShared   bool       `db:"is_shared" json:"is_shared"`
	ShareToken *string    `db:"share_token" json:"share_token,omitempty"`
	CreatedAt  time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at" json:"updated_at"`
}

// WishlistItem maps to the `wishlist_items` table.
type WishlistItem struct {
	ID         uuid.UUID `db:"id" json:"id"`
	WishlistID uuid.UUID `db:"wishlist_id" json:"wishlist_id"`
	ListingID  uuid.UUID `db:"listing_id" json:"listing_id"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}
