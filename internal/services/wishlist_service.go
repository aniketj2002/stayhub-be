package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/storage"
)

type WishlistService struct {
	wishlistRepo interfaces.WishlistRepository
	listingRepo  interfaces.ListingRepository
	mediaRepo    interfaces.MediaRepository
	storage      *storage.Storage
	clock        clock.Clock
}

func NewWishlistService(
	wishlistRepo interfaces.WishlistRepository,
	listingRepo interfaces.ListingRepository,
	mediaRepo interfaces.MediaRepository,
	store *storage.Storage,
	clk clock.Clock,
) interfaces.WishlistService {
	return &WishlistService{
		wishlistRepo: wishlistRepo,
		listingRepo:  listingRepo,
		mediaRepo:    mediaRepo,
		storage:      store,
		clock:        clk,
	}
}

func (s *WishlistService) CreateWishlist(ctx context.Context, userID uuid.UUID, req dtos.CreateWishlistRequest) (*dtos.WishlistSummaryResponse, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("%w: wishlist name is required", apperrors.ErrValidation)
	}

	now := s.clock.Now().UTC()
	w := &models.Wishlist{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      name,
		IsShared:  false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.wishlistRepo.Create(ctx, w); err != nil {
		return nil, err
	}

	return &dtos.WishlistSummaryResponse{
		ID:         w.ID,
		UserID:     w.UserID,
		Name:       w.Name,
		IsShared:   w.IsShared,
		ShareToken: w.ShareToken,
		ItemsCount: 0,
		CreatedAt:  w.CreatedAt,
		UpdatedAt:  w.UpdatedAt,
	}, nil
}

func (s *WishlistService) GetUserWishlists(ctx context.Context, userID uuid.UUID) ([]dtos.WishlistSummaryResponse, error) {
	wishlists, err := s.wishlistRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	counts, err := s.wishlistRepo.GetItemCountsByUser(ctx, userID)
	if err != nil {
		counts = make(map[uuid.UUID]int)
	}

	out := make([]dtos.WishlistSummaryResponse, 0, len(wishlists))
	for _, w := range wishlists {
		out = append(out, dtos.WishlistSummaryResponse{
			ID:         w.ID,
			UserID:     w.UserID,
			Name:       w.Name,
			IsShared:   w.IsShared,
			ShareToken: w.ShareToken,
			ItemsCount: counts[w.ID],
			CreatedAt:  w.CreatedAt,
			UpdatedAt:  w.UpdatedAt,
		})
	}
	return out, nil
}

func (s *WishlistService) GetWishlist(ctx context.Context, userID, wishlistID uuid.UUID) (*dtos.WishlistDetailResponse, error) {
	w, err := s.wishlistRepo.FindByID(ctx, wishlistID)
	if err != nil {
		return nil, err
	}
	if w.UserID != userID {
		return nil, apperrors.ErrForbidden
	}

	cards, err := s.getListingCardsForWishlist(ctx, w.ID)
	if err != nil {
		return nil, err
	}

	return &dtos.WishlistDetailResponse{
		ID:         w.ID,
		UserID:     w.UserID,
		Name:       w.Name,
		IsShared:   w.IsShared,
		ShareToken: w.ShareToken,
		Listings:   cards,
		CreatedAt:  w.CreatedAt,
		UpdatedAt:  w.UpdatedAt,
	}, nil
}

func (s *WishlistService) UpdateWishlist(ctx context.Context, userID, wishlistID uuid.UUID, req dtos.UpdateWishlistRequest) (*dtos.WishlistSummaryResponse, error) {
	w, err := s.wishlistRepo.FindByID(ctx, wishlistID)
	if err != nil {
		return nil, err
	}
	if w.UserID != userID {
		return nil, apperrors.ErrForbidden
	}

	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		if trimmed == "" {
			return nil, fmt.Errorf("%w: wishlist name cannot be empty", apperrors.ErrValidation)
		}
		w.Name = trimmed
	}

	if req.IsShared != nil {
		w.IsShared = *req.IsShared
		if !w.IsShared {
			w.ShareToken = nil
		}
	}

	w.UpdatedAt = s.clock.Now().UTC()
	if err := s.wishlistRepo.Update(ctx, w); err != nil {
		return nil, err
	}

	counts, err := s.wishlistRepo.GetItemCountsByUser(ctx, userID)
	itemCount := 0
	if err == nil {
		itemCount = counts[w.ID]
	}

	return &dtos.WishlistSummaryResponse{
		ID:         w.ID,
		UserID:     w.UserID,
		Name:       w.Name,
		IsShared:   w.IsShared,
		ShareToken: w.ShareToken,
		ItemsCount: itemCount,
		CreatedAt:  w.CreatedAt,
		UpdatedAt:  w.UpdatedAt,
	}, nil
}

func (s *WishlistService) DeleteWishlist(ctx context.Context, userID, wishlistID uuid.UUID) error {
	w, err := s.wishlistRepo.FindByID(ctx, wishlistID)
	if err != nil {
		return err
	}
	if w.UserID != userID {
		return apperrors.ErrForbidden
	}
	return s.wishlistRepo.Delete(ctx, wishlistID)
}

func (s *WishlistService) AddListing(ctx context.Context, userID, wishlistID, listingID uuid.UUID) error {
	w, err := s.wishlistRepo.FindByID(ctx, wishlistID)
	if err != nil {
		return err
	}
	if w.UserID != userID {
		return apperrors.ErrForbidden
	}

	// Verify listing exists
	l, err := s.listingRepo.FindByID(ctx, listingID)
	if err != nil {
		return err
	}
	if l.Status != enums.ListingStatusLive {
		return apperrors.ErrNotFound
	}

	return s.wishlistRepo.AddItem(ctx, wishlistID, listingID)
}

func (s *WishlistService) RemoveListing(ctx context.Context, userID, wishlistID, listingID uuid.UUID) error {
	w, err := s.wishlistRepo.FindByID(ctx, wishlistID)
	if err != nil {
		return err
	}
	if w.UserID != userID {
		return apperrors.ErrForbidden
	}
	return s.wishlistRepo.RemoveItem(ctx, wishlistID, listingID)
}

func (s *WishlistService) ShareWishlist(ctx context.Context, userID, wishlistID uuid.UUID) (*dtos.ShareWishlistResponse, error) {
	w, err := s.wishlistRepo.FindByID(ctx, wishlistID)
	if err != nil {
		return nil, err
	}
	if w.UserID != userID {
		return nil, apperrors.ErrForbidden
	}

	if w.ShareToken == nil || *w.ShareToken == "" {
		tokenBytes := make([]byte, 16)
		if _, err := rand.Read(tokenBytes); err != nil {
			return nil, fmt.Errorf("generate share token: %w", err)
		}
		tok := hex.EncodeToString(tokenBytes)
		w.ShareToken = &tok
	}
	w.IsShared = true
	w.UpdatedAt = s.clock.Now().UTC()

	if err := s.wishlistRepo.Update(ctx, w); err != nil {
		return nil, err
	}

	return &dtos.ShareWishlistResponse{
		ShareToken: *w.ShareToken,
		ShareURL:   fmt.Sprintf("/shared/wishlists/%s", *w.ShareToken),
	}, nil
}

func (s *WishlistService) UnshareWishlist(ctx context.Context, userID, wishlistID uuid.UUID) error {
	w, err := s.wishlistRepo.FindByID(ctx, wishlistID)
	if err != nil {
		return err
	}
	if w.UserID != userID {
		return apperrors.ErrForbidden
	}

	w.IsShared = false
	w.ShareToken = nil
	w.UpdatedAt = s.clock.Now().UTC()

	return s.wishlistRepo.Update(ctx, w)
}

func (s *WishlistService) GetSharedWishlist(ctx context.Context, token string) (*dtos.WishlistDetailResponse, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, apperrors.ErrNotFound
	}

	w, err := s.wishlistRepo.FindByShareToken(ctx, token)
	if err != nil {
		return nil, err
	}

	cards, err := s.getListingCardsForWishlist(ctx, w.ID)
	if err != nil {
		return nil, err
	}

	return &dtos.WishlistDetailResponse{
		ID:         w.ID,
		UserID:     w.UserID,
		Name:       w.Name,
		IsShared:   w.IsShared,
		ShareToken: w.ShareToken,
		Listings:   cards,
		CreatedAt:  w.CreatedAt,
		UpdatedAt:  w.UpdatedAt,
	}, nil
}

func (s *WishlistService) getListingCardsForWishlist(ctx context.Context, wishlistID uuid.UUID) ([]dtos.SearchListingCard, error) {
	listingIDs, err := s.wishlistRepo.GetItemListingIDs(ctx, wishlistID)
	if err != nil {
		return nil, err
	}

	cards := make([]dtos.SearchListingCard, 0, len(listingIDs))
	for _, id := range listingIDs {
		l, err := s.listingRepo.FindByID(ctx, id)
		if err != nil || l.Status != enums.ListingStatusLive || l.LiveVersionID == nil {
			continue // Only live listings are displayed in wishlists
		}

		lv, err := s.listingRepo.FindVersionByID(ctx, *l.LiveVersionID)
		if err != nil {
			continue
		}

		var coverPhotoURL *string
		photoURLs := make([]string, 0)
		if len(lv.MediaIDs) > 0 && s.mediaRepo != nil && s.storage != nil {
			if mediaList, err := s.mediaRepo.FindByIDs(ctx, lv.MediaIDs); err == nil {
				byID := make(map[uuid.UUID]models.Media, len(mediaList))
				for _, m := range mediaList {
					byID[m.ID] = m
				}
				for _, mid := range lv.MediaIDs {
					if m, ok := byID[mid]; ok {
						url := s.storage.PublicURL(m.S3Key)
						photoURLs = append(photoURLs, url)
					}
				}
				if len(photoURLs) > 0 {
					coverPhotoURL = &photoURLs[0]
				}
			}
		}

		basePrice := int64(0)
		if lv.BasePriceMinor != nil {
			basePrice = *lv.BasePriceMinor
		}
		maxGuests := 1
		if lv.MaxGuests != nil {
			maxGuests = *lv.MaxGuests
		}

		var rating *float64
		if l.ReviewCount > 0 {
			rate := l.RatingAvg
			rating = &rate
		}

		cards = append(cards, dtos.SearchListingCard{
			ID:                l.ID,
			HostID:            l.HostID,
			LiveVersionID:     lv.ID,
			PropertyType:      derefString(lv.PropertyType),
			Title:             derefString(lv.Title),
			Description:       lv.Description,
			City:              derefString(lv.City),
			Region:            lv.Region,
			Country:           derefString(lv.Country),
			Latitude:          lv.Latitude,
			Longitude:         lv.Longitude,
			Lat:               lv.Latitude,
			Lng:               lv.Longitude,
			MaxGuests:         maxGuests,
			Bedrooms:          lv.Bedrooms,
			Beds:              lv.Beds,
			Bathrooms:         lv.Bathrooms,
			InstantBook:       lv.InstantBook,
			Currency:          lv.Currency,
			NightlyPriceMinor: basePrice,
			CleaningFeeMinor:  lv.CleaningFeeMinor,
			CoverPhotoURL:     coverPhotoURL,
			PhotoURLs:         photoURLs,
			Rating:            rating,
			ReviewCount:       l.ReviewCount,
			IsGuestFavorite:   l.IsGuestFavorite,
		})
	}

	return cards, nil
}
