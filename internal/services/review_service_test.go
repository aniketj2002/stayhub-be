package services

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/clock"
)

type memReviewRepo struct {
	mu           sync.Mutex
	reviews      map[uuid.UUID]models.Review
	listingStats map[uuid.UUID]models.ListingRatingStats
	listingRepo  *memListingRepo
}

func newMemReviewRepo(listingRepo *memListingRepo) *memReviewRepo {
	return &memReviewRepo{
		reviews:      make(map[uuid.UUID]models.Review),
		listingStats: make(map[uuid.UUID]models.ListingRatingStats),
		listingRepo:  listingRepo,
	}
}

func (m *memReviewRepo) CreateReview(_ context.Context, review models.Review) (*models.Review, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if review.ID == uuid.Nil {
		review.ID = uuid.New()
	}
	m.reviews[review.ID] = review
	return &review, nil
}

func (m *memReviewRepo) FindReviewByReservationAndAuthor(_ context.Context, reservationID, authorID uuid.UUID) (*models.Review, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.reviews {
		if r.ReservationID == reservationID && r.AuthorID == authorID {
			return &r, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (m *memReviewRepo) ListReviewsByReservation(_ context.Context, reservationID uuid.UUID) ([]models.Review, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.Review
	for _, r := range m.reviews {
		if r.ReservationID == reservationID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *memReviewRepo) ListPublishedReviewsByListing(_ context.Context, listingID uuid.UUID) ([]models.Review, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.Review
	for _, r := range m.reviews {
		if r.ListingID == listingID && r.TargetType == enums.ReviewTargetListing && r.PublishedAt != nil {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *memReviewRepo) PublishReviews(_ context.Context, reviewIDs []uuid.UUID, publishedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range reviewIDs {
		if r, ok := m.reviews[id]; ok {
			r.PublishedAt = &publishedAt
			m.reviews[id] = r
		}
	}
	return nil
}

func (m *memReviewRepo) ListUnpublishedReviewsPastDeadline(_ context.Context, deadline time.Time) ([]models.Review, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.Review
	for _, r := range m.reviews {
		if r.PublishedAt == nil {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *memReviewRepo) UpdateListingRatingStats(_ context.Context, listingID uuid.UUID, stats models.ListingRatingStats) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listingStats[listingID] = stats
	if m.listingRepo != nil {
		if l, ok := m.listingRepo.listings[listingID]; ok {
			l.RatingAvg = stats.RatingAvg
			l.ReviewCount = stats.ReviewCount
			l.RatingCleanliness = stats.RatingCleanliness
			l.RatingAccuracy = stats.RatingAccuracy
			l.RatingCheckin = stats.RatingCheckin
			l.RatingCommunication = stats.RatingCommunication
			l.RatingLocation = stats.RatingLocation
			l.RatingValue = stats.RatingValue
			m.listingRepo.listings[listingID] = l
		}
	}
	return nil
}

func TestReviewService(t *testing.T) {
	checkoutTime := time.Date(2026, 10, 15, 11, 0, 0, 0, time.UTC)
	deadline := checkoutTime.Add(14 * 24 * time.Hour)
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMockClock(now)

	guestID := uuid.New()
	hostID := uuid.New()
	listingID := uuid.New()
	resID := uuid.New()

	guestUser := models.User{ID: guestID, FullName: "Alice Guest"}
	hostUser := models.User{ID: hostID, FullName: "Bob Host"}

	resRepo := newMemReservationRepo()
	resRepo.rows[resID] = models.Reservation{
		ID:             resID,
		ListingID:      listingID,
		GuestID:        guestID,
		HostID:         hostID,
		Status:         enums.ReservationStatusCompleted,
		CheckOutAt:     checkoutTime,
		ReviewDeadline: &deadline,
	}

	listingRepo := &memListingRepo{
		listings: map[uuid.UUID]models.Listing{
			listingID: {ID: listingID, HostID: hostID},
		},
	}
	reviewRepo := newMemReviewRepo(listingRepo)
	userRepo := &memUserRepo{
		users: map[uuid.UUID]models.User{
			guestID: guestUser,
			hostID:  hostUser,
		},
	}

	svc := NewReviewService(nil, reviewRepo, resRepo, listingRepo, userRepo, clk)

	t.Run("Guest submits review - unpublished initially", func(t *testing.T) {
		cleanRating := 5
		accRating := 5
		commRating := 4
		locRating := 5
		valRating := 4

		resp, err := svc.SubmitReview(context.Background(), guestUser, resID, dtos.CreateReviewInput{
			Rating:              5,
			CleanlinessRating:   &cleanRating,
			AccuracyRating:      &accRating,
			CommunicationRating: &commRating,
			LocationRating:      &locRating,
			ValueRating:         &valRating,
			PublicReview:        "Incredible stay! Spotlessly clean and wonderful location.",
		})
		if err != nil {
			t.Fatalf("expected no error submitting review: %v", err)
		}
		if resp.PublishedAt != nil {
			t.Errorf("expected review to be unpublished before host review or deadline")
		}

		// Public listing reviews should still be 0 (double-blind)
		pubSummary, err := svc.ListListingReviews(context.Background(), listingID)
		if err != nil {
			t.Fatalf("expected no error listing reviews: %v", err)
		}
		if pubSummary.ReviewCount != 0 {
			t.Errorf("expected 0 published reviews, got %d", pubSummary.ReviewCount)
		}

		// Host querying reservation reviews should not see guest's unpublished review
		hostReviews, err := svc.GetReservationReviews(context.Background(), hostID, resID)
		if err != nil {
			t.Fatalf("expected no error: %v", err)
		}
		if len(hostReviews) != 0 {
			t.Errorf("host should not see guest's unpublished review, got %d", len(hostReviews))
		}

		// Guest can see their own review
		guestReviews, err := svc.GetReservationReviews(context.Background(), guestID, resID)
		if err != nil {
			t.Fatalf("expected no error: %v", err)
		}
		if len(guestReviews) != 1 {
			t.Errorf("guest should see their own review, got %d", len(guestReviews))
		}
	})

	t.Run("Host submits review - triggers double-blind publish and updates reservation status to reviewed", func(t *testing.T) {
		resp, err := svc.SubmitReview(context.Background(), hostUser, resID, dtos.CreateReviewInput{
			Rating:       5,
			PublicReview: "Alice was a courteous, respectful guest. Highly recommended!",
		})
		if err != nil {
			t.Fatalf("expected no error: %v", err)
		}
		if resp.PublishedAt == nil {
			t.Errorf("expected host review to trigger atomic publication of both reviews")
		}

		// Reservation status should transition to 'reviewed'
		res := resRepo.rows[resID]
		if res.Status != enums.ReservationStatusReviewed {
			t.Errorf("expected reservation status 'reviewed', got '%s'", res.Status)
		}

		// Public listing reviews should now show guest's review and listing average
		pubSummary, err := svc.ListListingReviews(context.Background(), listingID)
		if err != nil {
			t.Fatalf("expected no error: %v", err)
		}
		if pubSummary.ReviewCount != 1 {
			t.Errorf("expected 1 published listing review, got %d", pubSummary.ReviewCount)
		}
		if pubSummary.RatingAvg != 5.0 {
			t.Errorf("expected rating avg 5.0, got %f", pubSummary.RatingAvg)
		}

		// Both host and guest can now see published reviews on reservation
		hostReviews, err := svc.GetReservationReviews(context.Background(), hostID, resID)
		if err != nil {
			t.Fatalf("expected no error: %v", err)
		}
		if len(hostReviews) != 2 {
			t.Errorf("expected 2 published reviews visible to host, got %d", len(hostReviews))
		}
	})

	t.Run("Reject duplicate review submission", func(t *testing.T) {
		_, err := svc.SubmitReview(context.Background(), guestUser, resID, dtos.CreateReviewInput{
			Rating:       4,
			PublicReview: "Trying to submit again",
		})
		if err == nil {
			t.Fatal("expected error submitting duplicate review")
		}
	})

	t.Run("Reject review after deadline passed", func(t *testing.T) {
		pastDeadline := checkoutTime.Add(-1 * time.Hour)
		resLateID := uuid.New()
		resRepo.rows[resLateID] = models.Reservation{
			ID:             resLateID,
			ListingID:      listingID,
			GuestID:        guestID,
			HostID:         hostID,
			Status:         enums.ReservationStatusCompleted,
			CheckOutAt:     checkoutTime,
			ReviewDeadline: &pastDeadline,
		}

		_, err := svc.SubmitReview(context.Background(), guestUser, resLateID, dtos.CreateReviewInput{
			Rating:       5,
			PublicReview: "Too late!",
		})
		if err == nil {
			t.Fatal("expected error submitting review past deadline")
		}
	})

	t.Run("CloseReviewWindows job publishes unpaired review after deadline", func(t *testing.T) {
		resSingleID := uuid.New()
		deadlineSingle := now.Add(-1 * time.Hour) // passed 1h ago
		resRepo.rows[resSingleID] = models.Reservation{
			ID:             resSingleID,
			ListingID:      listingID,
			GuestID:        guestID,
			HostID:         hostID,
			Status:         enums.ReservationStatusCompleted,
			CheckOutAt:     checkoutTime,
			ReviewDeadline: &deadlineSingle,
		}

		// Manually create single unpublished review for resSingleID
		revID := uuid.New()
		reviewRepo.reviews[revID] = models.Review{
			ID:            revID,
			ReservationID: resSingleID,
			ListingID:     listingID,
			AuthorID:      guestID,
			TargetUserID:  &hostID,
			TargetType:    enums.ReviewTargetListing,
			Rating:        4,
			PublicReview:  "Single review awaiting deadline sweep",
			CreatedAt:     now.Add(-2 * time.Hour),
		}

		publishedCount, err := svc.CloseReviewWindows(context.Background())
		if err != nil {
			t.Fatalf("expected no error running CloseReviewWindows: %v", err)
		}
		if publishedCount != 1 {
			t.Errorf("expected 1 review published by window closure, got %d", publishedCount)
		}

		rev := reviewRepo.reviews[revID]
		if rev.PublishedAt == nil {
			t.Errorf("expected review to be published after deadline sweep")
		}
	})
}
