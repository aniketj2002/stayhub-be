package services

import (
	"context"
	"strings"
	"time"

	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/redis"
)

const (
	amenitiesCacheKey            = "stayhub:catalog:amenities"
	cancellationPoliciesCacheKey = "stayhub:catalog:cancellation_policies"
	catalogCacheTTL              = 1 * time.Hour
)

type CatalogService struct {
	repo interfaces.CatalogRepository
	rdb  *redis.Client
}

func NewCatalogService(repo interfaces.CatalogRepository, rdb *redis.Client) interfaces.CatalogService {
	return &CatalogService{
		repo: repo,
		rdb:  rdb,
	}
}

func (s *CatalogService) ListAmenities(ctx context.Context) ([]models.Amenity, error) {
	if s.rdb != nil {
		var cached []models.Amenity
		if ok, err := s.rdb.GetJSON(ctx, amenitiesCacheKey, &cached); err == nil && ok {
			return cached, nil
		}
	}

	amenities, err := s.repo.ListAmenities(ctx)
	if err != nil {
		return nil, err
	}

	if s.rdb != nil {
		_ = s.rdb.SetJSON(ctx, amenitiesCacheKey, amenities, catalogCacheTTL)
	}
	return amenities, nil
}

func (s *CatalogService) ListCancellationPolicies(ctx context.Context) ([]models.CancellationPolicy, error) {
	if s.rdb != nil {
		var cached []models.CancellationPolicy
		if ok, err := s.rdb.GetJSON(ctx, cancellationPoliciesCacheKey, &cached); err == nil && ok {
			return cached, nil
		}
	}

	policies, err := s.repo.ListCancellationPolicies(ctx)
	if err != nil {
		return nil, err
	}

	if s.rdb != nil {
		_ = s.rdb.SetJSON(ctx, cancellationPoliciesCacheKey, policies, catalogCacheTTL)
	}
	return policies, nil
}

func (s *CatalogService) GetCancellationPolicy(ctx context.Context, code string) (*models.CancellationPolicy, error) {
	cleanCode := strings.ToLower(strings.TrimSpace(code))
	return s.repo.FindCancellationPolicyByCode(ctx, cleanCode)
}
