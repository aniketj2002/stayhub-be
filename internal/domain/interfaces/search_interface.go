package interfaces

import (
	"context"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/dtos"
)

type SearchService interface {
	SearchListings(ctx context.Context, filter dtos.SearchListingsFilter) ([]dtos.SearchListingCard, error)
	GetPublicListingDetail(ctx context.Context, listingID uuid.UUID) (*dtos.PublicListingDetailResponse, error)
	ComputeQuote(ctx context.Context, listingID uuid.UUID, in dtos.QuoteRequestInput) (*dtos.QuoteResponse, error)
}
