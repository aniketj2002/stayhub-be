package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/models"
)

type ListingRepository interface {
	CreateListing(ctx context.Context, hostID uuid.UUID) (*models.Listing, error)
	FindByID(ctx context.Context, id uuid.UUID) (*models.Listing, error)
	UpdatePointersAndStatus(
		ctx context.Context,
		id uuid.UUID,
		status enums.ListingStatus,
		liveVersionID, workingVersionID *uuid.UUID,
		now time.Time,
	) (*models.Listing, error)

	CreateVersion(ctx context.Context, v models.ListingVersion) (*models.ListingVersion, error)
	FindVersionByID(ctx context.Context, id uuid.UUID) (*models.ListingVersion, error)
	UpdateVersion(ctx context.Context, v models.ListingVersion) (*models.ListingVersion, error)
	ListVersionsByStatus(ctx context.Context, status enums.VersionStatus) ([]models.ListingVersion, error)

	ReplaceCheckResults(ctx context.Context, versionID uuid.UUID, results []models.ListingCheckResult) ([]models.ListingCheckResult, error)
	ListCheckResultsByVersion(ctx context.Context, versionID uuid.UUID) ([]models.ListingCheckResult, error)

	SearchLiveListings(ctx context.Context, filter dtos.SearchListingsFilter) ([]dtos.SearchListingRow, error)
}

type ListingService interface {
	CreateListing(ctx context.Context, actor models.AuthActor, in dtos.CreateListingInput) (*dtos.HostListingDetailResponse, error)
	ListHostListings(ctx context.Context, userID uuid.UUID) ([]dtos.HostListingSummaryResponse, error)
	GetHostListing(ctx context.Context, listingID uuid.UUID, hostAccess models.ListingHost) (*dtos.HostListingDetailResponse, error)
	UpdateWorkingVersion(ctx context.Context, listingID uuid.UUID, hostAccess models.ListingHost, in dtos.UpdateWorkingVersionInput) (*dtos.HostListingDetailResponse, error)
	SubmitListing(ctx context.Context, listingID uuid.UUID, hostAccess models.ListingHost) (*dtos.SubmitListingResponse, error)
	UpdateHostConfig(ctx context.Context, listingID uuid.UUID, hostAccess models.ListingHost, in dtos.UpdateHostConfigInput) (*models.ListingHost, error)
}
