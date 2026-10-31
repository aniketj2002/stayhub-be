package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/db"
)

func TestAdminListingReviewService_DiffAndLifecycle(t *testing.T) {
	listingRepo := newMemListingRepo()
	userRepo := newMemUserRepo()
	clk := clock.NewMockClock(time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC))
	svc := NewAdminListingReviewService((*db.DB)(nil), listingRepo, userRepo, nil, clk)

	hostID := uuid.New()
	userRepo.users[hostID] = models.User{
		ID:       hostID,
		Email:    "host@stayhub.dev",
		FullName: "Host User",
	}

	listingID := uuid.New()
	photo1 := uuid.New()
	photo2 := uuid.New()
	photo3 := uuid.New()
	photo4 := uuid.New()

	v1Title := "Cozy Cabin"
	v1Price := int64(10000)
	v1MaxGuests := 2
	liveV1 := models.ListingVersion{
		ID:               uuid.New(),
		ListingID:        listingID,
		VersionNo:        1,
		Status:           enums.VersionStatusApproved,
		Title:            &v1Title,
		BasePriceMinor:   &v1Price,
		CleaningFeeMinor: 2000,
		MaxGuests:        &v1MaxGuests,
		Amenities:        []string{"wifi"},
		MediaIDs:         []uuid.UUID{photo1, photo2, photo3},
	}
	listingRepo.versions[liveV1.ID] = liveV1

	v2Title := "Luxury Cabin"
	v2Price := int64(15000)
	v2MaxGuests := 4
	submittedV2 := models.ListingVersion{
		ID:               uuid.New(),
		ListingID:        listingID,
		VersionNo:        2,
		Status:           enums.VersionStatusSubmitted,
		Title:            &v2Title,
		BasePriceMinor:   &v2Price,
		CleaningFeeMinor: 2500,
		MaxGuests:        &v2MaxGuests,
		Amenities:        []string{"wifi", "pool"},
		MediaIDs:         []uuid.UUID{photo2, photo1, photo4}, // photo3 removed, photo4 added, photo1 & photo2 swapped
	}
	listingRepo.versions[submittedV2.ID] = submittedV2

	listingRepo.listings[listingID] = models.Listing{
		ID:               listingID,
		HostID:           hostID,
		Status:           enums.ListingStatusLive,
		LiveVersionID:    &liveV1.ID,
		WorkingVersionID: &submittedV2.ID,
	}

	ctx := context.Background()

	// 1. Diff
	diff, err := svc.GetDiff(ctx, submittedV2.ID)
	if err != nil {
		t.Fatalf("unexpected diff error: %v", err)
	}

	if diff.VersionNo != 2 || *diff.LiveVersionNo != 1 {
		t.Errorf("expected version 2 vs live version 1, got %d vs %v", diff.VersionNo, diff.LiveVersionNo)
	}

	titleDiff, ok := diff.FieldDiffs["title"]
	if !ok || titleDiff.Old != "Cozy Cabin" || titleDiff.New != "Luxury Cabin" {
		t.Errorf("unexpected title diff: %+v", titleDiff)
	}

	priceDiff, ok := diff.FieldDiffs["base_price_minor"]
	if !ok || priceDiff.Old != int64(10000) || priceDiff.New != int64(15000) {
		t.Errorf("unexpected price diff: %+v", priceDiff)
	}

	if len(diff.PhotosDiff.Added) != 1 || diff.PhotosDiff.Added[0] != photo4 {
		t.Errorf("expected photo4 added, got %+v", diff.PhotosDiff.Added)
	}
	if len(diff.PhotosDiff.Removed) != 1 || diff.PhotosDiff.Removed[0] != photo3 {
		t.Errorf("expected photo3 removed, got %+v", diff.PhotosDiff.Removed)
	}
	if !diff.PhotosDiff.Reordered {
		t.Errorf("expected photos reordered to be true")
	}

	adminActor := models.AuthActor{
		SubjectID:   uuid.New(),
		SubjectType: "admin",
	}

	// 2. Reject empty note
	if err := svc.Approve(ctx, adminActor, submittedV2.ID, "  "); !errors.Is(err, apperrors.ErrValidation) {
		t.Errorf("expected validation error on empty note, got %v", err)
	}

	// 3. Approve
	if err := svc.Approve(ctx, adminActor, submittedV2.ID, "Approved by admin"); err != nil {
		t.Fatalf("unexpected approve error: %v", err)
	}

	upListing := listingRepo.listings[listingID]
	if *upListing.LiveVersionID != submittedV2.ID {
		t.Errorf("expected live version to be updated to %s, got %s", submittedV2.ID, *upListing.LiveVersionID)
	}

	prevLive := listingRepo.versions[liveV1.ID]
	if prevLive.Status != enums.VersionStatusSuperseded {
		t.Errorf("expected previous live version to be superseded, got %s", prevLive.Status)
	}

	newLive := listingRepo.versions[submittedV2.ID]
	if newLive.Status != enums.VersionStatusApproved {
		t.Errorf("expected new version to be approved, got %s", newLive.Status)
	}
	if newLive.DecisionNote == nil || *newLive.DecisionNote != "Approved by admin" {
		t.Errorf("expected decision note recorded, got %v", newLive.DecisionNote)
	}
}
