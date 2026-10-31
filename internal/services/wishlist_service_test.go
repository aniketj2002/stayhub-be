package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/clock"
)

type memWishlistRepo struct {
	wishlists map[uuid.UUID]*models.Wishlist
	items     map[uuid.UUID]map[uuid.UUID]bool // wishlistID -> set of listingIDs
}

func newMemWishlistRepo() *memWishlistRepo {
	return &memWishlistRepo{
		wishlists: make(map[uuid.UUID]*models.Wishlist),
		items:     make(map[uuid.UUID]map[uuid.UUID]bool),
	}
}

func (r *memWishlistRepo) Create(_ context.Context, w *models.Wishlist) error {
	copied := *w
	r.wishlists[w.ID] = &copied
	return nil
}

func (r *memWishlistRepo) FindByID(_ context.Context, id uuid.UUID) (*models.Wishlist, error) {
	w, ok := r.wishlists[id]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	copied := *w
	return &copied, nil
}

func (r *memWishlistRepo) FindByUserID(_ context.Context, userID uuid.UUID) ([]models.Wishlist, error) {
	var out []models.Wishlist
	for _, w := range r.wishlists {
		if w.UserID == userID {
			out = append(out, *w)
		}
	}
	return out, nil
}

func (r *memWishlistRepo) FindByShareToken(_ context.Context, token string) (*models.Wishlist, error) {
	for _, w := range r.wishlists {
		if w.ShareToken != nil && *w.ShareToken == token && w.IsShared {
			copied := *w
			return &copied, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (r *memWishlistRepo) Update(_ context.Context, w *models.Wishlist) error {
	if _, ok := r.wishlists[w.ID]; !ok {
		return apperrors.ErrNotFound
	}
	copied := *w
	r.wishlists[w.ID] = &copied
	return nil
}

func (r *memWishlistRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := r.wishlists[id]; !ok {
		return apperrors.ErrNotFound
	}
	delete(r.wishlists, id)
	delete(r.items, id)
	return nil
}

func (r *memWishlistRepo) AddItem(_ context.Context, wishlistID, listingID uuid.UUID) error {
	if _, ok := r.items[wishlistID]; !ok {
		r.items[wishlistID] = make(map[uuid.UUID]bool)
	}
	r.items[wishlistID][listingID] = true
	return nil
}

func (r *memWishlistRepo) RemoveItem(_ context.Context, wishlistID, listingID uuid.UUID) error {
	if m, ok := r.items[wishlistID]; ok {
		delete(m, listingID)
	}
	return nil
}

func (r *memWishlistRepo) GetItemListingIDs(_ context.Context, wishlistID uuid.UUID) ([]uuid.UUID, error) {
	var out []uuid.UUID
	if m, ok := r.items[wishlistID]; ok {
		for id := range m {
			out = append(out, id)
		}
	}
	return out, nil
}

func (r *memWishlistRepo) GetItemCountsByUser(_ context.Context, userID uuid.UUID) (map[uuid.UUID]int, error) {
	counts := make(map[uuid.UUID]int)
	for wID, w := range r.wishlists {
		if w.UserID == userID {
			counts[wID] = len(r.items[wID])
		}
	}
	return counts, nil
}

func TestWishlistService_LifecycleAndSharing(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMockClock(time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC))
	wishlistRepo := newMemWishlistRepo()
	listingRepo := newMemListingRepo()

	user1 := uuid.New()
	user2 := uuid.New()

	// Seed a live listing
	listingID := uuid.New()
	versionID := uuid.New()
	basePrice := int64(15000)
	title := "Coastal Villa"
	city := "Cascais"
	country := "PT"
	listingRepo.listings[listingID] = models.Listing{
		ID:            listingID,
		HostID:        user2,
		Status:        enums.ListingStatusLive,
		LiveVersionID: &versionID,
	}
	listingRepo.versions[versionID] = models.ListingVersion{
		ID:             versionID,
		ListingID:      listingID,
		Title:          &title,
		City:           &city,
		Country:        &country,
		BasePriceMinor: &basePrice,
	}

	svc := NewWishlistService(wishlistRepo, listingRepo, nil, nil, clk)

	// 1. Create Wishlist
	created, err := svc.CreateWishlist(ctx, user1, dtos.CreateWishlistRequest{Name: "Dream Stays"})
	if err != nil {
		t.Fatalf("CreateWishlist failed: %v", err)
	}
	if created.Name != "Dream Stays" {
		t.Errorf("expected name 'Dream Stays', got %s", created.Name)
	}
	if created.IsShared {
		t.Errorf("expected is_shared to be false")
	}

	// 2. Reject empty name
	_, err = svc.CreateWishlist(ctx, user1, dtos.CreateWishlistRequest{Name: "   "})
	if err == nil {
		t.Fatalf("expected validation error on empty name")
	}

	// 3. Add Item to Wishlist
	err = svc.AddListing(ctx, user1, created.ID, listingID)
	if err != nil {
		t.Fatalf("AddListing failed: %v", err)
	}

	// 4. Verify List Wishlists
	lists, err := svc.GetUserWishlists(ctx, user1)
	if err != nil || len(lists) != 1 {
		t.Fatalf("GetUserWishlists failed or wrong count: %v", err)
	}
	if lists[0].ItemsCount != 1 {
		t.Errorf("expected items_count = 1, got %d", lists[0].ItemsCount)
	}

	// 5. User2 cannot access User1's wishlist
	_, err = svc.GetWishlist(ctx, user2, created.ID)
	if err != apperrors.ErrForbidden {
		t.Errorf("expected ErrForbidden, got %v", err)
	}

	// 6. Share Wishlist generates token
	shareRes, err := svc.ShareWishlist(ctx, user1, created.ID)
	if err != nil {
		t.Fatalf("ShareWishlist failed: %v", err)
	}
	if shareRes.ShareToken == "" {
		t.Fatalf("expected non-empty share token")
	}

	// 7. Public view via share token works
	shared, err := svc.GetSharedWishlist(ctx, shareRes.ShareToken)
	if err != nil {
		t.Fatalf("GetSharedWishlist failed: %v", err)
	}
	if len(shared.Listings) != 1 {
		t.Fatalf("expected 1 listing in shared wishlist, got %d", len(shared.Listings))
	}
	if shared.Listings[0].Title != "Coastal Villa" {
		t.Errorf("expected title 'Coastal Villa', got %s", shared.Listings[0].Title)
	}

	// 8. Unshare revokes the token and makes public link return 404
	err = svc.UnshareWishlist(ctx, user1, created.ID)
	if err != nil {
		t.Fatalf("UnshareWishlist failed: %v", err)
	}
	_, err = svc.GetSharedWishlist(ctx, shareRes.ShareToken)
	if err != apperrors.ErrNotFound {
		t.Errorf("expected ErrNotFound after unsharing, got %v", err)
	}

	// 9. Remove Item
	err = svc.RemoveListing(ctx, user1, created.ID, listingID)
	if err != nil {
		t.Fatalf("RemoveListing failed: %v", err)
	}
	detail, err := svc.GetWishlist(ctx, user1, created.ID)
	if err != nil {
		t.Fatalf("GetWishlist failed: %v", err)
	}
	if len(detail.Listings) != 0 {
		t.Errorf("expected 0 listings after removal, got %d", len(detail.Listings))
	}

	// 10. Delete Wishlist
	err = svc.DeleteWishlist(ctx, user1, created.ID)
	if err != nil {
		t.Fatalf("DeleteWishlist failed: %v", err)
	}
	_, err = svc.GetWishlist(ctx, user1, created.ID)
	if err != apperrors.ErrNotFound {
		t.Errorf("expected ErrNotFound after deletion, got %v", err)
	}
}
