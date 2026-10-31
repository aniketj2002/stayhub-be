package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	platformIcal "github.com/stayhub/backend/internal/platform/ical"
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/db"
)

type ICalService struct {
	db           *db.DB
	icalRepo     interfaces.ICalRepository
	calendarRepo interfaces.CalendarRepository
	listingRepo  interfaces.ListingRepository
	clock        clock.Clock
}

func NewICalService(
	database *db.DB,
	icalRepo interfaces.ICalRepository,
	calendarRepo interfaces.CalendarRepository,
	listingRepo interfaces.ListingRepository,
	clk clock.Clock,
) interfaces.ICalService {
	return &ICalService{
		db:           database,
		icalRepo:     icalRepo,
		calendarRepo: calendarRepo,
		listingRepo:  listingRepo,
		clock:        clk,
	}
}

func (s *ICalService) CreateFeed(ctx context.Context, listingID uuid.UUID, in dtos.CreateICalFeedInput) (*dtos.ICalFeedResponse, error) {
	if in.Name == "" {
		return nil, fmt.Errorf("%w: feed name is required", apperrors.ErrValidation)
	}
	if err := platformIcal.ValidateSSRF(in.URL); err != nil {
		return nil, fmt.Errorf("%w: invalid feed URL: %v", apperrors.ErrValidation, err)
	}

	feed, err := s.icalRepo.CreateFeed(ctx, models.ICalFeed{
		ListingID: listingID,
		Name:      in.Name,
		URL:       in.URL,
	})
	if err != nil {
		return nil, err
	}

	// Trigger initial sync asynchronously or synchronously
	go func(id uuid.UUID) {
		_ = s.SyncFeed(context.Background(), id)
	}(feed.ID)

	return &dtos.ICalFeedResponse{
		ID:            feed.ID,
		ListingID:     feed.ListingID,
		Name:          feed.Name,
		URL:           feed.URL,
		LastSyncedAt:  feed.LastSyncedAt,
		LastSyncError: feed.LastSyncError,
		CreatedAt:     feed.CreatedAt,
	}, nil
}

func (s *ICalService) ListFeeds(ctx context.Context, listingID uuid.UUID) ([]dtos.ICalFeedResponse, error) {
	feeds, err := s.icalRepo.ListFeedsByListing(ctx, listingID)
	if err != nil {
		return nil, err
	}

	out := make([]dtos.ICalFeedResponse, 0, len(feeds))
	for _, f := range feeds {
		out = append(out, dtos.ICalFeedResponse{
			ID:            f.ID,
			ListingID:     f.ListingID,
			Name:          f.Name,
			URL:           f.URL,
			LastSyncedAt:  f.LastSyncedAt,
			LastSyncError: f.LastSyncError,
			CreatedAt:     f.CreatedAt,
		})
	}
	return out, nil
}

func (s *ICalService) DeleteFeed(ctx context.Context, listingID uuid.UUID, feedID uuid.UUID) error {
	feed, err := s.icalRepo.GetFeedByID(ctx, feedID)
	if err != nil {
		return err
	}
	if feed.ListingID != listingID {
		return apperrors.ErrForbidden
	}

	// Remove blocked nights first
	_ = s.calendarRepo.RemoveICalBlockedNights(ctx, feedID)
	return s.icalRepo.DeleteFeed(ctx, feedID)
}

func (s *ICalService) GetExportFeed(ctx context.Context, token string) (string, error) {
	listing, err := s.icalRepo.GetListingByExportToken(ctx, token)
	if err != nil {
		return "", err
	}

	now := s.clock.Now().Truncate(24 * time.Hour)
	entries, err := s.calendarRepo.ListAllFutureByListing(ctx, listing.ID, now)
	if err != nil {
		return "", err
	}

	listingTitle := ""
	if listing.LiveVersionID != nil {
		if ver, err := s.listingRepo.FindVersionByID(ctx, *listing.LiveVersionID); err == nil && ver != nil && ver.Title != nil {
			listingTitle = *ver.Title
		}
	}

	ics := platformIcal.SerializeCalendar(listing.ID, listingTitle, entries)
	return ics, nil
}

func (s *ICalService) RegenerateExportToken(ctx context.Context, listingID uuid.UUID) (*dtos.ICalExportResponse, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return nil, fmt.Errorf("generate random token: %w", err)
	}
	token := hex.EncodeToString(bytes)

	if err := s.icalRepo.SetExportToken(ctx, listingID, token); err != nil {
		return nil, err
	}

	exportURL := fmt.Sprintf("/api/v1/ical/%s.ics", token)
	return &dtos.ICalExportResponse{
		ExportURL: exportURL,
		Token:     token,
	}, nil
}

func (s *ICalService) SyncFeed(ctx context.Context, feedID uuid.UUID) error {
	feed, err := s.icalRepo.GetFeedByID(ctx, feedID)
	if err != nil {
		return err
	}

	body, err := platformIcal.FetchFeedWithGuard(ctx, feed.URL, 10*time.Second)
	now := s.clock.Now()
	if err != nil {
		errMsg := err.Error()
		_ = s.icalRepo.UpdateFeedSyncStatus(ctx, feedID, now, &errMsg)
		return err
	}

	events, err := platformIcal.ParseCalendar(body)
	if err != nil {
		errMsg := fmt.Sprintf("parse ical: %v", err)
		_ = s.icalRepo.UpdateFeedSyncStatus(ctx, feedID, now, &errMsg)
		return err
	}

	// Extract unique nights across all events
	nightsMap := make(map[string]time.Time)
	for _, ev := range events {
		cur := ev.Start
		for cur.Before(ev.End) {
			key := cur.Format("2006-01-02")
			nightsMap[key] = cur
			cur = cur.AddDate(0, 0, 1)
		}
	}

	nights := make([]time.Time, 0, len(nightsMap))
	for _, t := range nightsMap {
		nights = append(nights, t)
	}

	if err := s.calendarRepo.SyncICalBlockedNights(ctx, feed.ListingID, feed.ID, nights); err != nil {
		errMsg := fmt.Sprintf("sync calendar: %v", err)
		_ = s.icalRepo.UpdateFeedSyncStatus(ctx, feedID, now, &errMsg)
		return err
	}

	return s.icalRepo.UpdateFeedSyncStatus(ctx, feedID, now, nil)
}

func (s *ICalService) SyncAllFeeds(ctx context.Context) (int, error) {
	feeds, err := s.icalRepo.ListAllFeeds(ctx)
	if err != nil {
		return 0, err
	}

	synced := 0
	for _, f := range feeds {
		if err := s.SyncFeed(ctx, f.ID); err == nil {
			synced++
		}
	}
	return synced, nil
}
