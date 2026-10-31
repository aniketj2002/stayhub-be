package services

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/db"
)

type ReviewService struct {
	db              *db.DB
	reviewRepo      interfaces.ReviewRepository
	reservationRepo interfaces.ReservationRepository
	listingRepo     interfaces.ListingRepository
	userRepo        interfaces.UserRepository
	clock           clock.Clock
}

func NewReviewService(
	database *db.DB,
	reviewRepo interfaces.ReviewRepository,
	reservationRepo interfaces.ReservationRepository,
	listingRepo interfaces.ListingRepository,
	userRepo interfaces.UserRepository,
	clk clock.Clock,
) interfaces.ReviewService {
	return &ReviewService{
		db:              database,
		reviewRepo:      reviewRepo,
		reservationRepo: reservationRepo,
		listingRepo:     listingRepo,
		userRepo:        userRepo,
		clock:           clk,
	}
}

func (s *ReviewService) SubmitReview(
	ctx context.Context,
	author models.User,
	reservationID uuid.UUID,
	in dtos.CreateReviewInput,
) (*dtos.ReviewResponse, error) {
	if in.Rating < 1 || in.Rating > 5 {
		return nil, fmt.Errorf("%w: rating must be between 1 and 5", apperrors.ErrValidation)
	}
	publicReview := strings.TrimSpace(in.PublicReview)
	if publicReview == "" {
		return nil, fmt.Errorf("%w: public review text is required", apperrors.ErrValidation)
	}

	for name, val := range map[string]*int{
		"cleanliness":   in.CleanlinessRating,
		"accuracy":      in.AccuracyRating,
		"check_in":      in.CheckInRating,
		"communication": in.CommunicationRating,
		"location":      in.LocationRating,
		"value":         in.ValueRating,
	} {
		if val != nil && (*val < 1 || *val > 5) {
			return nil, fmt.Errorf("%w: %s rating must be between 1 and 5", apperrors.ErrValidation, name)
		}
	}

	res, err := s.reservationRepo.FindByID(ctx, reservationID)
	if err != nil {
		return nil, err
	}

	// Must be completed or already reviewed
	if res.Status != enums.ReservationStatusCompleted && res.Status != enums.ReservationStatusReviewed {
		return nil, fmt.Errorf("%w: reviews can only be submitted for completed stays", apperrors.ErrValidation)
	}

	now := s.clock.Now()
	if res.ReviewDeadline != nil && now.After(*res.ReviewDeadline) {
		return nil, fmt.Errorf("%w: review window has closed (14-day limit exceeded)", apperrors.ErrValidation)
	}

	var targetType enums.ReviewTargetType
	var targetUserID *uuid.UUID
	if author.ID == res.GuestID {
		targetType = enums.ReviewTargetListing
		targetUserID = &res.HostID
	} else if author.ID == res.HostID {
		targetType = enums.ReviewTargetGuest
		targetUserID = &res.GuestID
	} else {
		return nil, fmt.Errorf("%w: only the guest or primary host can review this stay", apperrors.ErrForbidden)
	}

	// Check if already submitted
	existing, err := s.reviewRepo.FindReviewByReservationAndAuthor(ctx, reservationID, author.ID)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("%w: you have already submitted a review for this stay", apperrors.ErrValidation)
	}

	var createdReview *models.Review
	var shouldPublishBoth bool
	var counterpartReview *models.Review

	err = s.db.WithinTx(ctx, func(txCtx context.Context) error {
		// 1. Insert new review
		rev, err := s.reviewRepo.CreateReview(txCtx, models.Review{
			ReservationID:        reservationID,
			AuthorID:             author.ID,
			TargetType:           targetType,
			ListingID:            res.ListingID,
			TargetUserID:         targetUserID,
			Rating:               in.Rating,
			CleanlinessRating:    in.CleanlinessRating,
			AccuracyRating:       in.AccuracyRating,
			CheckInRating:        in.CheckInRating,
			CommunicationRating:  in.CommunicationRating,
			LocationRating:       in.LocationRating,
			ValueRating:          in.ValueRating,
			PublicReview:         publicReview,
			PrivateNote:          in.PrivateNote,
			PublishedAt:          nil,
		})
		if err != nil {
			return err
		}
		createdReview = rev

		// 2. Check if counterpart review exists
		reviews, err := s.reviewRepo.ListReviewsByReservation(txCtx, reservationID)
		if err != nil {
			return err
		}

		for _, r := range reviews {
			if r.AuthorID != author.ID {
				cp := r
				counterpartReview = &cp
				shouldPublishBoth = true
				break
			}
		}

		// 3. If both reviews are submitted, publish BOTH immediately!
		if shouldPublishBoth && counterpartReview != nil {
			publishTime := now
			if err := s.reviewRepo.PublishReviews(txCtx, []uuid.UUID{createdReview.ID, counterpartReview.ID}, publishTime); err != nil {
				return err
			}
			createdReview.PublishedAt = &publishTime

			// Transition reservation status to 'reviewed'
			res.Status = enums.ReservationStatusReviewed
			res.UpdatedAt = now
			if _, err := s.reservationRepo.Update(txCtx, *res); err != nil {
				return err
			}

			// Recompute listing ratings
			if err := s.refreshListingRatings(txCtx, res.ListingID); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.toReviewResponse(createdReview, author.FullName), nil
}

func (s *ReviewService) ListListingReviews(ctx context.Context, listingID uuid.UUID) (*dtos.ListingReviewSummaryResponse, error) {
	reviews, err := s.reviewRepo.ListPublishedReviewsByListing(ctx, listingID)
	if err != nil {
		return nil, err
	}

	listing, err := s.listingRepo.FindByID(ctx, listingID)
	if err != nil {
		return nil, err
	}

	reviewResponses := make([]dtos.ReviewResponse, 0, len(reviews))
	for _, r := range reviews {
		authorName := "Guest"
		if u, err := s.userRepo.FindByID(ctx, r.AuthorID); err == nil && u != nil {
			authorName = u.FullName
		}
		reviewResponses = append(reviewResponses, *s.toReviewResponse(&r, authorName))
	}

	return &dtos.ListingReviewSummaryResponse{
		RatingAvg:           listing.RatingAvg,
		ReviewCount:         listing.ReviewCount,
		RatingCleanliness:   listing.RatingCleanliness,
		RatingAccuracy:      listing.RatingAccuracy,
		RatingCheckin:       listing.RatingCheckin,
		RatingCommunication: listing.RatingCommunication,
		RatingLocation:      listing.RatingLocation,
		RatingValue:         listing.RatingValue,
		Reviews:             reviewResponses,
	}, nil
}

func (s *ReviewService) GetReservationReviews(ctx context.Context, userID, reservationID uuid.UUID) ([]dtos.ReviewResponse, error) {
	reviews, err := s.reviewRepo.ListReviewsByReservation(ctx, reservationID)
	if err != nil {
		return nil, err
	}

	var out []dtos.ReviewResponse
	for _, r := range reviews {
		// Double-blind rule: If published, anyone can view. If unpublished, only author can view.
		if r.PublishedAt != nil || r.AuthorID == userID {
			authorName := "User"
			if u, err := s.userRepo.FindByID(ctx, r.AuthorID); err == nil && u != nil {
				authorName = u.FullName
			}
			out = append(out, *s.toReviewResponse(&r, authorName))
		}
	}
	return out, nil
}

func (s *ReviewService) CloseReviewWindows(ctx context.Context) (int, error) {
	now := s.clock.Now()
	expiredReviews, err := s.reviewRepo.ListUnpublishedReviewsPastDeadline(ctx, now)
	if err != nil {
		return 0, err
	}

	if len(expiredReviews) == 0 {
		return 0, nil
	}

	var reviewIDs []uuid.UUID
	listingIDs := make(map[uuid.UUID]bool)
	for _, r := range expiredReviews {
		reviewIDs = append(reviewIDs, r.ID)
		listingIDs[r.ListingID] = true
	}

	err = s.db.WithinTx(ctx, func(txCtx context.Context) error {
		if err := s.reviewRepo.PublishReviews(txCtx, reviewIDs, now); err != nil {
			return err
		}

		for _, r := range expiredReviews {
			res, err := s.reservationRepo.FindByID(txCtx, r.ReservationID)
			if err == nil && res != nil && res.Status == enums.ReservationStatusCompleted {
				res.Status = enums.ReservationStatusReviewed
				res.UpdatedAt = now
				_, _ = s.reservationRepo.Update(txCtx, *res)
			}
		}

		for lID := range listingIDs {
			_ = s.refreshListingRatings(txCtx, lID)
		}

		return nil
	})
	if err != nil {
		return 0, err
	}

	return len(reviewIDs), nil
}

func (s *ReviewService) refreshListingRatings(ctx context.Context, listingID uuid.UUID) error {
	reviews, err := s.reviewRepo.ListPublishedReviewsByListing(ctx, listingID)
	if err != nil {
		return err
	}

	if len(reviews) == 0 {
		return s.reviewRepo.UpdateListingRatingStats(ctx, listingID, models.ListingRatingStats{
			RatingAvg:   0.00,
			ReviewCount: 0,
		})
	}

	count := len(reviews)
	var sumRating, sumClean, sumAcc, sumCheck, sumComm, sumLoc, sumVal float64
	var countClean, countAcc, countCheck, countComm, countLoc, countVal int

	for _, r := range reviews {
		sumRating += float64(r.Rating)
		if r.CleanlinessRating != nil {
			sumClean += float64(*r.CleanlinessRating)
			countClean++
		}
		if r.AccuracyRating != nil {
			sumAcc += float64(*r.AccuracyRating)
			countAcc++
		}
		if r.CheckInRating != nil {
			sumCheck += float64(*r.CheckInRating)
			countCheck++
		}
		if r.CommunicationRating != nil {
			sumComm += float64(*r.CommunicationRating)
			countComm++
		}
		if r.LocationRating != nil {
			sumLoc += float64(*r.LocationRating)
			countLoc++
		}
		if r.ValueRating != nil {
			sumVal += float64(*r.ValueRating)
			countVal++
		}
	}

	round := func(val float64) float64 {
		return math.Round(val*100) / 100
	}

	stats := models.ListingRatingStats{
		RatingAvg:   round(sumRating / float64(count)),
		ReviewCount: count,
	}
	if countClean > 0 {
		c := round(sumClean / float64(countClean))
		stats.RatingCleanliness = &c
	}
	if countAcc > 0 {
		a := round(sumAcc / float64(countAcc))
		stats.RatingAccuracy = &a
	}
	if countCheck > 0 {
		ch := round(sumCheck / float64(countCheck))
		stats.RatingCheckin = &ch
	}
	if countComm > 0 {
		cm := round(sumComm / float64(countComm))
		stats.RatingCommunication = &cm
	}
	if countLoc > 0 {
		l := round(sumLoc / float64(countLoc))
		stats.RatingLocation = &l
	}
	if countVal > 0 {
		v := round(sumVal / float64(countVal))
		stats.RatingValue = &v
	}

	return s.reviewRepo.UpdateListingRatingStats(ctx, listingID, stats)
}

func (s *ReviewService) toReviewResponse(r *models.Review, authorName string) *dtos.ReviewResponse {
	return &dtos.ReviewResponse{
		ID:                   r.ID,
		ReservationID:        r.ReservationID,
		AuthorID:             r.AuthorID,
		AuthorName:           authorName,
		TargetType:           r.TargetType,
		ListingID:            r.ListingID,
		TargetUserID:         r.TargetUserID,
		Rating:               r.Rating,
		CleanlinessRating:    r.CleanlinessRating,
		AccuracyRating:       r.AccuracyRating,
		CheckInRating:        r.CheckInRating,
		CommunicationRating:  r.CommunicationRating,
		LocationRating:       r.LocationRating,
		ValueRating:          r.ValueRating,
		PublicReview:         r.PublicReview,
		PrivateNote:          r.PrivateNote,
		PublishedAt:          r.PublishedAt,
		CreatedAt:            r.CreatedAt,
	}
}
