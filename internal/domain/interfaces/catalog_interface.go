package interfaces

import (
	"context"

	"github.com/stayhub/backend/internal/domain/models"
)

type CatalogRepository interface {
	ListAmenities(ctx context.Context) ([]models.Amenity, error)
	ListCancellationPolicies(ctx context.Context) ([]models.CancellationPolicy, error)
	FindCancellationPolicyByCode(ctx context.Context, code string) (*models.CancellationPolicy, error)
}

type CatalogService interface {
	ListAmenities(ctx context.Context) ([]models.Amenity, error)
	ListCancellationPolicies(ctx context.Context) ([]models.CancellationPolicy, error)
	GetCancellationPolicy(ctx context.Context, code string) (*models.CancellationPolicy, error)
}
