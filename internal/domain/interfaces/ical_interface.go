package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/models"
)

type ICalRepository interface {
	CreateFeed(ctx context.Context, feed models.ICalFeed) (*models.ICalFeed, error)
	GetFeedByID(ctx context.Context, feedID uuid.UUID) (*models.ICalFeed, error)
	ListFeedsByListing(ctx context.Context, listingID uuid.UUID) ([]models.ICalFeed, error)
	DeleteFeed(ctx context.Context, feedID uuid.UUID) error
	ListAllFeeds(ctx context.Context) ([]models.ICalFeed, error)
	UpdateFeedSyncStatus(ctx context.Context, feedID uuid.UUID, lastSyncedAt time.Time, syncError *string) error
	GetListingByExportToken(ctx context.Context, token string) (*models.Listing, error)
	SetExportToken(ctx context.Context, listingID uuid.UUID, token string) error
}

type ICalService interface {
	CreateFeed(ctx context.Context, listingID uuid.UUID, in dtos.CreateICalFeedInput) (*dtos.ICalFeedResponse, error)
	ListFeeds(ctx context.Context, listingID uuid.UUID) ([]dtos.ICalFeedResponse, error)
	DeleteFeed(ctx context.Context, listingID uuid.UUID, feedID uuid.UUID) error
	GetExportFeed(ctx context.Context, token string) (string, error)
	RegenerateExportToken(ctx context.Context, listingID uuid.UUID) (*dtos.ICalExportResponse, error)
	SyncFeed(ctx context.Context, feedID uuid.UUID) error
	SyncAllFeeds(ctx context.Context) (int, error)
}
