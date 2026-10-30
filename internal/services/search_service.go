package services

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/config"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/storage"
)

type SearchService struct {
	cfg            *config.Config
	listingRepo    interfaces.ListingRepository
	calendarRepo   interfaces.CalendarRepository
	userRepo       interfaces.UserRepository
	mediaRepo      interfaces.MediaRepository
	catalogService interfaces.CatalogService
	storage        *storage.Storage
	clock          clock.Clock
}

func NewSearchService(
	cfg *config.Config,
	listingRepo interfaces.ListingRepository,
	calendarRepo interfaces.CalendarRepository,
	userRepo interfaces.UserRepository,
	mediaRepo interfaces.MediaRepository,
	catalogService interfaces.CatalogService,
	store *storage.Storage,
	clk clock.Clock,
) interfaces.SearchService {
	return &SearchService{
		cfg:            cfg,
		listingRepo:    listingRepo,
		calendarRepo:   calendarRepo,
		userRepo:       userRepo,
		mediaRepo:      mediaRepo,
		catalogService: catalogService,
		storage:        store,
		clock:          clk,
	}
}

func (s *SearchService) SearchListings(ctx context.Context, filter dtos.SearchListingsFilter) ([]dtos.SearchListingCard, error) {
	rows, err := s.listingRepo.SearchLiveListings(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Collect all media IDs for a single batch query
	mediaIDSet := make(map[uuid.UUID]bool)
	var allMediaIDs []uuid.UUID
	for _, r := range rows {
		for _, id := range r.LiveVersion.MediaIDs {
			if !mediaIDSet[id] {
				mediaIDSet[id] = true
				allMediaIDs = append(allMediaIDs, id)
			}
		}
	}

	mediaByID := make(map[uuid.UUID]models.Media, len(allMediaIDs))
	if len(allMediaIDs) > 0 && s.mediaRepo != nil {
		if mediaList, err := s.mediaRepo.FindByIDs(ctx, allMediaIDs); err == nil {
			for _, m := range mediaList {
				mediaByID[m.ID] = m
			}
		}
	}

	var nightsCount int
	if filter.CheckIn != nil && filter.CheckOut != nil {
		nightsCount = len(models.EnumerateNights(*filter.CheckIn, *filter.CheckOut))
	}

	cards := make([]dtos.SearchListingCard, 0, len(rows))
	for _, r := range rows {
		lv := r.LiveVersion
		photoURLs := make([]string, 0, len(lv.MediaIDs))
		for _, id := range lv.MediaIDs {
			if m, ok := mediaByID[id]; ok && s.storage != nil {
				photoURLs = append(photoURLs, s.storage.PublicURL(m.S3Key))
			}
		}
		var coverURL *string
		if len(photoURLs) > 0 {
			first := photoURLs[0]
			coverURL = &first
		}

		basePrice := int64(0)
		if lv.BasePriceMinor != nil {
			basePrice = *lv.BasePriceMinor
		}
		maxGuests := 1
		if lv.MaxGuests != nil {
			maxGuests = *lv.MaxGuests
		}

		card := dtos.SearchListingCard{
			ID:                r.Listing.ID,
			HostID:            r.Listing.HostID,
			LiveVersionID:     lv.ID,
			PropertyType:      derefString(lv.PropertyType),
			Title:             derefString(lv.Title),
			City:              derefString(lv.City),
			Region:            lv.Region,
			Country:           derefString(lv.Country),
			MaxGuests:         maxGuests,
			InstantBook:       lv.InstantBook,
			Currency:          lv.Currency,
			NightlyPriceMinor: basePrice,
			CleaningFeeMinor:  lv.CleaningFeeMinor,
			CoverPhotoURL:     coverURL,
			PhotoURLs:         photoURLs,
			Rating:            nil, // V1: rating is always null
			ReviewCount:       0,
		}

		if nightsCount > 0 {
			q := models.ComputeStayQuote(basePrice, lv.CleaningFeeMinor, nightsCount, s.cfg.GuestFeeBPS, s.cfg.HostFeeBPS)
			n := q.Nights
			nt := q.NightlyTotalMinor
			gf := q.GuestFeeMinor
			st := q.GuestTotalMinor
			card.Nights = &n
			card.NightlyTotalMinor = &nt
			card.GuestFeeMinor = &gf
			card.StayTotalMinor = &st
		}

		cards = append(cards, card)
	}

	return cards, nil
}

func (s *SearchService) GetPublicListingDetail(ctx context.Context, listingID uuid.UUID) (*dtos.PublicListingDetailResponse, error) {
	l, err := s.listingRepo.FindByID(ctx, listingID)
	if err != nil {
		return nil, err
	}
	if l.Status != enums.ListingStatusLive || l.LiveVersionID == nil {
		return nil, apperrors.ErrNotFound
	}

	lv, err := s.listingRepo.FindVersionByID(ctx, *l.LiveVersionID)
	if err != nil {
		return nil, err
	}

	// Resolve ordered photos
	photos := make([]dtos.MediaResponse, 0, len(lv.MediaIDs))
	if len(lv.MediaIDs) > 0 && s.mediaRepo != nil && s.storage != nil {
		if mediaRecords, err := s.mediaRepo.FindByIDs(ctx, lv.MediaIDs); err == nil {
			byID := make(map[uuid.UUID]models.Media, len(mediaRecords))
			for _, m := range mediaRecords {
				byID[m.ID] = m
			}
			for _, id := range lv.MediaIDs {
				if m, ok := byID[id]; ok {
					photos = append(photos, dtos.ToMediaResponse(m, s.storage.PublicURL(m.S3Key)))
				}
			}
		}
	}

	// Resolve amenities with labels & categories
	resolvedAmenities := make([]models.Amenity, 0, len(lv.Amenities))
	if allAmenities, err := s.catalogService.ListAmenities(ctx); err == nil {
		byCode := make(map[string]models.Amenity, len(allAmenities))
		for _, a := range allAmenities {
			byCode[a.Code] = a
		}
		for _, code := range lv.Amenities {
			if a, ok := byCode[code]; ok {
				resolvedAmenities = append(resolvedAmenities, a)
			} else {
				resolvedAmenities = append(resolvedAmenities, models.Amenity{
					Code:     code,
					Label:    code,
					Category: "general",
				})
			}
		}
	}

	// Resolve cancellation policy
	policyCode := "flexible"
	if lv.CancellationPolicy != nil && strings.TrimSpace(*lv.CancellationPolicy) != "" {
		policyCode = strings.ToLower(strings.TrimSpace(*lv.CancellationPolicy))
	}
	var policy models.CancellationPolicy
	if p, err := s.catalogService.GetCancellationPolicy(ctx, policyCode); err == nil && p != nil {
		policy = *p
	} else {
		policy = models.CancellationPolicy{Code: policyCode, Name: policyCode}
	}

	// Resolve host profile
	hostProfile := dtos.PublicHostProfile{
		ID: l.HostID,
	}
	if s.userRepo != nil {
		if hostUser, err := s.userRepo.FindByID(ctx, l.HostID); err == nil {
			hostProfile.FullName = hostUser.FullName
			hostProfile.JoinedAt = hostUser.CreatedAt
			if hostUser.AvatarMediaID != nil && s.mediaRepo != nil && s.storage != nil {
				if m, err := s.mediaRepo.FindByID(ctx, *hostUser.AvatarMediaID); err == nil && m.Status == "uploaded" {
					url := s.storage.PublicURL(m.S3Key)
					hostProfile.AvatarURL = &url
				}
			}
		}
	}

	// Load upcoming 365 days of unavailable calendar dates
	now := s.clock.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	unavailable := make([]dtos.CalendarNightEntry, 0)
	if calRows, err := s.calendarRepo.ListRange(ctx, listingID, today, today.AddDate(1, 0, 0)); err == nil {
		for _, r := range calRows {
			unavailable = append(unavailable, dtos.CalendarNightEntry{
				Night:  r.Night.Format("2006-01-02"),
				Status: string(r.Status),
			})
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

	return &dtos.PublicListingDetailResponse{
		ID:                 l.ID,
		Status:             l.Status,
		LiveVersionID:      lv.ID,
		VersionNo:          lv.VersionNo,
		PropertyType:       derefString(lv.PropertyType),
		Title:              derefString(lv.Title),
		Description:        derefString(lv.Description),
		City:               derefString(lv.City),
		Region:             lv.Region,
		Country:            derefString(lv.Country),
		Timezone:           lv.Timezone,
		MaxGuests:          maxGuests,
		BasePriceMinor:     basePrice,
		CleaningFeeMinor:   lv.CleaningFeeMinor,
		Currency:           lv.Currency,
		InstantBook:        lv.InstantBook,
		CheckInTime:        lv.CheckInTime,
		CheckOutTime:       lv.CheckOutTime,
		HouseRules:         derefString(lv.HouseRules),
		Photos:             photos,
		Amenities:          resolvedAmenities,
		CancellationPolicy: policy,
		Host:               hostProfile,
		Rating:             nil,
		ReviewCount:        0,
		UnavailableDates:   unavailable,
	}, nil
}

func (s *SearchService) ComputeQuote(ctx context.Context, listingID uuid.UUID, in dtos.QuoteRequestInput) (*dtos.QuoteResponse, error) {
	l, err := s.listingRepo.FindByID(ctx, listingID)
	if err != nil {
		return nil, err
	}
	if l.Status != enums.ListingStatusLive || l.LiveVersionID == nil {
		return nil, apperrors.ErrListingNotLive
	}

	lv, err := s.listingRepo.FindVersionByID(ctx, *l.LiveVersionID)
	if err != nil {
		return nil, err
	}

	maxGuests := 1
	if lv.MaxGuests != nil {
		maxGuests = *lv.MaxGuests
	}
	if in.Guests > maxGuests {
		return nil, apperrors.ErrGuestLimitExceeded
	}

	// Check availability across [CheckIn, CheckOut)
	existingCal, err := s.calendarRepo.ListRange(ctx, listingID, in.CheckIn, in.CheckOut)
	if err != nil {
		return nil, err
	}
	if len(existingCal) > 0 {
		return nil, apperrors.ErrDatesUnavailable
	}

	nights := models.EnumerateNights(in.CheckIn, in.CheckOut)
	basePrice := int64(0)
	if lv.BasePriceMinor != nil {
		basePrice = *lv.BasePriceMinor
	}

	breakdown := models.ComputeStayQuote(
		basePrice,
		lv.CleaningFeeMinor,
		len(nights),
		s.cfg.GuestFeeBPS,
		s.cfg.HostFeeBPS,
	)

	checkInAt, err := models.CombineLocalDateAndTime(in.CheckIn, lv.CheckInTime, lv.Timezone)
	if err != nil {
		checkInAt, _ = models.CombineLocalDateAndTime(in.CheckIn, "15:00", "UTC")
	}
	checkOutAt, err := models.CombineLocalDateAndTime(in.CheckOut, lv.CheckOutTime, lv.Timezone)
	if err != nil {
		checkOutAt, _ = models.CombineLocalDateAndTime(in.CheckOut, "11:00", "UTC")
	}

	now := s.clock.Now()
	loc := time.UTC
	if lv.Timezone != "" {
		if l, err := time.LoadLocation(lv.Timezone); err == nil {
			loc = l
		}
	}
	nowInLoc := now.In(loc)
	todayInLoc := time.Date(nowInLoc.Year(), nowInLoc.Month(), nowInLoc.Day(), 0, 0, 0, 0, time.UTC)
	if in.CheckIn.Before(todayInLoc) || checkInAt.Before(now) {
		return nil, apperrors.ErrPastDate
	}

	policyCode := "flexible"
	if lv.CancellationPolicy != nil && strings.TrimSpace(*lv.CancellationPolicy) != "" {
		policyCode = strings.ToLower(strings.TrimSpace(*lv.CancellationPolicy))
	}
	var policy models.CancellationPolicy
	if p, err := s.catalogService.GetCancellationPolicy(ctx, policyCode); err == nil && p != nil {
		policy = *p
	} else {
		policy = models.CancellationPolicy{Code: policyCode, Name: policyCode}
	}

	milestones := make([]dtos.QuoteMilestone, 0, len(policy.Rules))
	for _, rule := range policy.Rules {
		cutoff := checkInAt.Add(-time.Duration(rule.MinHoursBefore) * time.Hour)
		milestones = append(milestones, dtos.QuoteMilestone{
			MinHoursBefore:    rule.MinHoursBefore,
			CutoffAt:          cutoff,
			RefundPct:         rule.RefundPct,
			RefundsServiceFee: rule.RefundsServiceFee,
		})
	}

	return &dtos.QuoteResponse{
		ListingID:          l.ID,
		ListingVersionID:   lv.ID,
		CheckIn:            in.CheckIn.Format("2006-01-02"),
		CheckOut:           in.CheckOut.Format("2006-01-02"),
		CheckInTime:        lv.CheckInTime,
		CheckOutTime:       lv.CheckOutTime,
		Timezone:           lv.Timezone,
		CheckInAt:          checkInAt,
		CheckOutAt:         checkOutAt,
		Guests:             in.Guests,
		InstantBook:        lv.InstantBook,
		Currency:           lv.Currency,
		Breakdown:          breakdown,
		CancellationPolicy: policy,
		RefundMilestones:   milestones,
	}, nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
