package interfaces

import (
	"context"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/models"
)

type WishlistRepository interface {
	Create(ctx context.Context, w *models.Wishlist) error
	FindByID(ctx context.Context, id uuid.UUID) (*models.Wishlist, error)
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]models.Wishlist, error)
	FindByShareToken(ctx context.Context, token string) (*models.Wishlist, error)
	Update(ctx context.Context, w *models.Wishlist) error
	Delete(ctx context.Context, id uuid.UUID) error

	AddItem(ctx context.Context, wishlistID, listingID uuid.UUID) error
	RemoveItem(ctx context.Context, wishlistID, listingID uuid.UUID) error
	GetItemListingIDs(ctx context.Context, wishlistID uuid.UUID) ([]uuid.UUID, error)
	GetItemCountsByUser(ctx context.Context, userID uuid.UUID) (map[uuid.UUID]int, error)
}

type WishlistService interface {
	CreateWishlist(ctx context.Context, userID uuid.UUID, req dtos.CreateWishlistRequest) (*dtos.WishlistSummaryResponse, error)
	GetUserWishlists(ctx context.Context, userID uuid.UUID) ([]dtos.WishlistSummaryResponse, error)
	GetWishlist(ctx context.Context, userID, wishlistID uuid.UUID) (*dtos.WishlistDetailResponse, error)
	UpdateWishlist(ctx context.Context, userID, wishlistID uuid.UUID, req dtos.UpdateWishlistRequest) (*dtos.WishlistSummaryResponse, error)
	DeleteWishlist(ctx context.Context, userID, wishlistID uuid.UUID) error

	AddListing(ctx context.Context, userID, wishlistID, listingID uuid.UUID) error
	RemoveListing(ctx context.Context, userID, wishlistID, listingID uuid.UUID) error

	ShareWishlist(ctx context.Context, userID, wishlistID uuid.UUID) (*dtos.ShareWishlistResponse, error)
	UnshareWishlist(ctx context.Context, userID, wishlistID uuid.UUID) error
	GetSharedWishlist(ctx context.Context, token string) (*dtos.WishlistDetailResponse, error)
}
