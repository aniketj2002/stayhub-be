package services

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/db"
)

type memICalRepo struct {
	mu           sync.Mutex
	feeds        map[uuid.UUID]models.ICalFeed
	tokens       map[uuid.UUID]string
	byToken      map[string]uuid.UUID
	listingRepo  *memListingRepo
}

func newMemICalRepo(lRepo *memListingRepo) *memICalRepo {
	return &memICalRepo{
		feeds:       make(map[uuid.UUID]models.ICalFeed),
		tokens:      make(map[uuid.UUID]string),
		byToken:     make(map[string]uuid.UUID),
		listingRepo: lRepo,
	}
}

func (m *memICalRepo) CreateFeed(_ context.Context, feed models.ICalFeed) (*models.ICalFeed, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if feed.ID == uuid.Nil {
		feed.ID = uuid.New()
	}
	feed.CreatedAt = time.Now()
	feed.UpdatedAt = time.Now()
	m.feeds[feed.ID] = feed
	return &feed, nil
}

func (m *memICalRepo) GetFeedByID(_ context.Context, feedID uuid.UUID) (*models.ICalFeed, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.feeds[feedID]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	return &f, nil
}

func (m *memICalRepo) ListFeedsByListing(_ context.Context, listingID uuid.UUID) ([]models.ICalFeed, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.ICalFeed
	for _, f := range m.feeds {
		if f.ListingID == listingID {
			out = append(out, f)
		}
	}
	return out, nil
}

func (m *memICalRepo) DeleteFeed(_ context.Context, feedID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.feeds, feedID)
	return nil
}

func (m *memICalRepo) ListAllFeeds(_ context.Context) ([]models.ICalFeed, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.ICalFeed
	for _, f := range m.feeds {
		out = append(out, f)
	}
	return out, nil
}

func (m *memICalRepo) UpdateFeedSyncStatus(_ context.Context, feedID uuid.UUID, lastSyncedAt time.Time, syncError *string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.feeds[feedID]
	if !ok {
		return apperrors.ErrNotFound
	}
	f.LastSyncedAt = &lastSyncedAt
	f.LastSyncError = syncError
	m.feeds[feedID] = f
	return nil
}

func (m *memICalRepo) GetListingByExportToken(ctx context.Context, token string) (*models.Listing, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byToken[token]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	return m.listingRepo.FindByID(ctx, id)
}

func (m *memICalRepo) SetExportToken(_ context.Context, listingID uuid.UUID, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if oldTok, ok := m.tokens[listingID]; ok {
		delete(m.byToken, oldTok)
	}
	m.tokens[listingID] = token
	m.byToken[token] = listingID
	return nil
}

func TestICalService_ExportAndFeedManagement(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	mockClk := clock.NewMockClock(now)

	listingRepo := newMemListingRepo()
	calRepo := newMemCalendarRepo()
	icalRepo := newMemICalRepo(listingRepo)

	svc := NewICalService(&db.DB{}, icalRepo, calRepo, listingRepo, mockClk)
	ctx := context.Background()

	listingID := uuid.New()
	versionID := uuid.New()
	title := "Alpine Chalet"
	listingRepo.listings[listingID] = models.Listing{
		ID:            listingID,
		LiveVersionID: &versionID,
	}
	listingRepo.versions[versionID] = models.ListingVersion{
		ID:        versionID,
		ListingID: listingID,
		Title:     &title,
	}

	// 1. SSRF Guard on CreateFeed
	_, err := svc.CreateFeed(ctx, listingID, dtos.CreateICalFeedInput{
		Name: "Loopback",
		URL:  "https://127.0.0.1/feed.ics",
	})
	if err == nil {
		t.Fatalf("expected error for SSRF loopback URL")
	}

	_, err = svc.CreateFeed(ctx, listingID, dtos.CreateICalFeedInput{
		Name: "HTTP",
		URL:  "http://example.com/feed.ics",
	})
	if err == nil {
		t.Fatalf("expected error for non-https URL")
	}

	// 2. Generate export token
	expResp, err := svc.RegenerateExportToken(ctx, listingID)
	if err != nil {
		t.Fatalf("failed to regenerate export token: %v", err)
	}
	if expResp.Token == "" {
		t.Fatalf("expected non-empty export token")
	}
	if !strings.HasPrefix(expResp.ExportURL, "/api/v1/ical/") {
		t.Errorf("expected export url prefix /api/v1/ical/, got %s", expResp.ExportURL)
	}

	// 3. Export calendar as .ics
	// Add a booked night and a blocked night
	_ = calRepo.BookNights(ctx, listingID, uuid.New(), []time.Time{
		time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC),
	})
	reason := "maintenance"
	_ = calRepo.BlockNights(ctx, listingID, []time.Time{
		time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC),
	}, &reason)

	icsStr, err := svc.GetExportFeed(ctx, expResp.Token)
	if err != nil {
		t.Fatalf("failed to get export feed: %v", err)
	}
	if !strings.Contains(icsStr, "BEGIN:VCALENDAR") {
		t.Errorf("export missing BEGIN:VCALENDAR")
	}
	if !strings.Contains(icsStr, "X-WR-CALNAME:Alpine Chalet") {
		t.Errorf("export missing X-WR-CALNAME")
	}
	if !strings.Contains(icsStr, "20261105") || !strings.Contains(icsStr, "20261110") {
		t.Errorf("export missing exported dates")
	}

	// 4. Delete feed removes associated blocked calendar entries
	feedID := uuid.New()
	icalRepo.feeds[feedID] = models.ICalFeed{
		ID:        feedID,
		ListingID: listingID,
		Name:      "Old Feed",
		URL:       "https://remote.com/cal.ics",
	}
	// Sync nights for this feed
	_ = calRepo.SyncICalBlockedNights(ctx, listingID, feedID, []time.Time{
		time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
	})

	if err := svc.DeleteFeed(ctx, listingID, feedID); err != nil {
		t.Fatalf("failed to delete feed: %v", err)
	}
	if _, ok := icalRepo.feeds[feedID]; ok {
		t.Errorf("feed was not deleted from repository")
	}
	// Verify blocked nights for that feed were removed
	futureNights, _ := calRepo.ListAllFutureByListing(ctx, listingID, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC))
	for _, n := range futureNights {
		if n.ICalFeedID != nil && *n.ICalFeedID == feedID {
			t.Errorf("expected feed nights to be cleaned up")
		}
	}
}
