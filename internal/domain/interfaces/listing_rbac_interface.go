package interfaces

import (
	"context"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/models"
)

type ListingHostRepository interface {
	FindByListingAndUser(ctx context.Context, listingID, userID uuid.UUID) (*models.ListingHost, error)
	ListByListing(ctx context.Context, listingID uuid.UUID) ([]models.ListingHost, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]models.ListingHost, error)
	CreateOwner(ctx context.Context, listingID, userID uuid.UUID, configBytes []byte) (*models.ListingHost, error)
	UpdateConfig(ctx context.Context, listingID, userID uuid.UUID, configBytes []byte) (*models.ListingHost, error)
	CreateCohost(ctx context.Context, host models.ListingHost) (*models.ListingHost, error)
	UpdateCohost(ctx context.Context, host models.ListingHost) (*models.ListingHost, error)
	DeleteCohost(ctx context.Context, listingID, userID uuid.UUID) error
}
