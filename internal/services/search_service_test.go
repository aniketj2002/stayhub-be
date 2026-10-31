package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/config"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/clock"
)

type searchMockListingRepo struct {
	*memListingRepo
	callCount int
	rows      []dtos.SearchListingRow
}

func (m *searchMockListingRepo) SearchLiveListings(_ context.Context, f dtos.SearchListingsFilter) ([]dtos.SearchListingRow, error) {
	m.callCount++
	var filtered []dtos.SearchListingRow
	for _, r := range m.rows {
		// BBox filter check
		if f.NeLat != nil && f.NeLng != nil && f.SwLat != nil && f.SwLng != nil {
			if r.LiveVersion.Latitude == nil || r.LiveVersion.Longitude == nil {
				continue
			}
			lat := *r.LiveVersion.Latitude
			lng := *r.LiveVersion.Longitude
			if lat < *f.SwLat || lat > *f.NeLat || lng < *f.SwLng || lng > *f.NeLng {
				continue
			}
		}
		// Instant book filter check
		if f.InstantBook != nil && r.LiveVersion.InstantBook != *f.InstantBook {
			continue
		}
		// Guest favorite check
		if f.GuestFavorite != nil && *f.GuestFavorite && !r.Listing.IsGuestFavorite {
			continue
		}
		// Superhost check
		if f.Superhost != nil && *f.Superhost && !r.HostIsSuperhost {
			continue
		}
		// Bedrooms check
		if f.Bedrooms != nil {
			beds := 1
			if r.LiveVersion.Bedrooms != nil {
				beds = *r.LiveVersion.Bedrooms
			}
			if beds < *f.Bedrooms {
				continue
			}
		}
		filtered = append(filtered, r)
	}
	return filtered, nil
}

func TestSearchService_FiltersAndMapBoundingBox(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMockClock(time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC))
	cfg := &config.Config{GuestFeeBPS: 1400, HostFeeBPS: 300}

	lisbonLat, lisbonLng := 38.7223, -9.1393
	portoLat, portoLng := 41.1579, -8.6291
	price1 := int64(10000)
	price2 := int64(25000)
	beds2 := 2
	beds4 := 4

	row1 := dtos.SearchListingRow{
		Listing: models.Listing{
			ID:              uuid.New(),
			HostID:          uuid.New(),
			Status:          enums.ListingStatusLive,
			RatingAvg:       4.95,
			ReviewCount:     42,
			IsGuestFavorite: true,
		},
		LiveVersion: models.ListingVersion{
			ID:               uuid.New(),
			Latitude:         &lisbonLat,
			Longitude:        &lisbonLng,
			InstantBook:      true,
			Bedrooms:         &beds2,
			BasePriceMinor:   &price1,
			CleaningFeeMinor: 2000,
		},
		HostIsSuperhost: true,
	}

	row2 := dtos.SearchListingRow{
		Listing: models.Listing{
			ID:              uuid.New(),
			HostID:          uuid.New(),
			Status:          enums.ListingStatusLive,
			RatingAvg:       4.50,
			ReviewCount:     8,
			IsGuestFavorite: false,
		},
		LiveVersion: models.ListingVersion{
			ID:               uuid.New(),
			Latitude:         &portoLat,
			Longitude:        &portoLng,
			InstantBook:      false,
			Bedrooms:         &beds4,
			BasePriceMinor:   &price2,
			CleaningFeeMinor: 3000,
		},
		HostIsSuperhost: false,
	}

	mockRepo := &searchMockListingRepo{
		memListingRepo: newMemListingRepo(),
		rows:           []dtos.SearchListingRow{row1, row2},
	}

	svc := NewSearchService(cfg, mockRepo, nil, nil, nil, nil, nil, clk, nil, nil)

	// Test 1: Bounding Box around Lisbon only
	neLat, neLng := 39.0, -9.0
	swLat, swLng := 38.5, -9.5
	res, err := svc.SearchListings(ctx, dtos.SearchListingsFilter{
		NeLat: &neLat,
		NeLng: &neLng,
		SwLat: &swLat,
		SwLng: &swLng,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(res) != 1 || res[0].ID != row1.Listing.ID {
		t.Fatalf("expected only Lisbon listing in bbox, got %d results", len(res))
	}
	if res[0].Lat == nil || *res[0].Lat != lisbonLat {
		t.Errorf("expected card to have pin latitude = %f", lisbonLat)
	}
	if !res[0].IsGuestFavorite {
		t.Errorf("expected card to have is_guest_favorite = true")
	}
	if !res[0].IsSuperhost {
		t.Errorf("expected card to have is_superhost = true")
	}

	// Test 2: Instant Book filter
	instant := true
	res, err = svc.SearchListings(ctx, dtos.SearchListingsFilter{
		InstantBook: &instant,
	})
	if err != nil || len(res) != 1 || res[0].ID != row1.Listing.ID {
		t.Fatalf("expected only instant book listing, got %d results", len(res))
	}

	// Test 3: Guest favorite filter
	gf := true
	res, err = svc.SearchListings(ctx, dtos.SearchListingsFilter{
		GuestFavorite: &gf,
	})
	if err != nil || len(res) != 1 || res[0].ID != row1.Listing.ID {
		t.Fatalf("expected only guest favorite listing, got %d results", len(res))
	}

	// Test 4: Superhost filter
	sh := true
	res, err = svc.SearchListings(ctx, dtos.SearchListingsFilter{
		Superhost: &sh,
	})
	if err != nil || len(res) != 1 || res[0].ID != row1.Listing.ID {
		t.Fatalf("expected only superhost listing, got %d results", len(res))
	}

	// Test 5: Bedrooms filter (min 3 bedrooms)
	minBedrooms := 3
	res, err = svc.SearchListings(ctx, dtos.SearchListingsFilter{
		Bedrooms: &minBedrooms,
	})
	if err != nil || len(res) != 1 || res[0].ID != row2.Listing.ID {
		t.Fatalf("expected only Porto listing with 4 bedrooms, got %d results", len(res))
	}
}
