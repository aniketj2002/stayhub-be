package dtos

import (
	"time"

	"github.com/google/uuid"
)

type CreateWishlistRequest struct {
	Name string `json:"name"`
}

type UpdateWishlistRequest struct {
	Name     *string `json:"name,omitempty"`
	IsShared *bool   `json:"is_shared,omitempty"`
}

type WishlistSummaryResponse struct {
	ID         uuid.UUID `json:"id"`
	UserID     uuid.UUID `json:"user_id"`
	Name       string    `json:"name"`
	IsShared   bool      `json:"is_shared"`
	ShareToken *string   `json:"share_token,omitempty"`
	ItemsCount int       `json:"items_count"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type WishlistDetailResponse struct {
	ID         uuid.UUID           `json:"id"`
	UserID     uuid.UUID           `json:"user_id"`
	Name       string              `json:"name"`
	IsShared   bool                `json:"is_shared"`
	ShareToken *string             `json:"share_token,omitempty"`
	Listings   []SearchListingCard `json:"listings"`
	CreatedAt  time.Time           `json:"created_at"`
	UpdatedAt  time.Time           `json:"updated_at"`
}

type ShareWishlistResponse struct {
	ShareToken string `json:"share_token"`
	ShareURL   string `json:"share_url"`
}
