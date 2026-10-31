package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
)

type ItineraryService struct {
	reservationRepo interfaces.ReservationRepository
	listingRepo     interfaces.ListingRepository
	listingHostRepo interfaces.ListingHostRepository
	userRepo        interfaces.UserRepository
	paymentRepo     interfaces.PaymentRepository
}

func NewItineraryService(
	reservationRepo interfaces.ReservationRepository,
	listingRepo interfaces.ListingRepository,
	listingHostRepo interfaces.ListingHostRepository,
	userRepo interfaces.UserRepository,
	paymentRepo interfaces.PaymentRepository,
) interfaces.ItineraryService {
	return &ItineraryService{
		reservationRepo: reservationRepo,
		listingRepo:     listingRepo,
		listingHostRepo: listingHostRepo,
		userRepo:        userRepo,
		paymentRepo:     paymentRepo,
	}
}

func (s *ItineraryService) GetItinerary(
	ctx context.Context,
	userID, reservationID uuid.UUID,
) (*dtos.TripItineraryResponse, error) {
	res, err := s.reservationRepo.FindByID(ctx, reservationID)
	if err != nil {
		return nil, err
	}

	// Authorize: guest or host on listing
	isAuthorized := (res.GuestID == userID)
	if !isAuthorized {
		lh, err := s.listingHostRepo.FindByListingAndUser(ctx, res.ListingID, userID)
		if err == nil && lh != nil {
			isAuthorized = true
		}
	}
	if !isAuthorized {
		return nil, apperrors.ErrForbidden
	}

	listing, err := s.listingRepo.FindByID(ctx, res.ListingID)
	if err != nil {
		return nil, err
	}

	var title string
	var city, country string
	var streetAddress *string
	var coverPhotoURL *string

	if listing.LiveVersionID != nil {
		ver, err := s.listingRepo.FindVersionByID(ctx, *listing.LiveVersionID)
		if err == nil && ver != nil {
			if ver.Title != nil {
				title = *ver.Title
			}
			if ver.City != nil {
				city = *ver.City
			}
			if ver.Country != nil {
				country = *ver.Country
			}
			// Check-in card address: shown only once confirmed, checked_in, completed, or reviewed
			isConfirmed := res.Status == enums.ReservationStatusConfirmed ||
				res.Status == enums.ReservationStatusCheckedIn ||
				res.Status == enums.ReservationStatusCompleted ||
				res.Status == enums.ReservationStatusReviewed
			if isConfirmed && ver.AddressLine1 != nil {
				streetAddress = ver.AddressLine1
			}
		}
	}

	hostName := "Host"
	hostEmail := ""
	if hostUser, err := s.userRepo.FindByID(ctx, res.HostID); err == nil && hostUser != nil {
		hostName = hostUser.FullName
		hostEmail = hostUser.Email
	}

	// Build day-by-day timeline
	timeline := s.buildTimeline(*res)

	// Build receipts
	payments, _ := s.paymentRepo.ListPaymentsByReservation(ctx, res.ID)
	receipts := make([]dtos.ReceiptPayment, 0, len(payments))
	var totalPaidMinor int64
	for _, p := range payments {
		if p.Status == enums.PaymentStatusSucceeded {
			if p.Kind == enums.PaymentKindCharge {
				totalPaidMinor += p.AmountMinor
			} else if p.Kind == enums.PaymentKindRefund {
				totalPaidMinor -= p.AmountMinor
			}
		}
		receipts = append(receipts, dtos.ReceiptPayment{
			ID:          p.ID,
			Type:        string(p.Kind),
			AmountEUR:   float64(p.AmountMinor) / 100.0,
			AmountMinor: p.AmountMinor,
			Status:      p.Status,
			CreatedAt:   p.CreatedAt,
		})
	}

	checkInDateStr := res.CheckIn.Format("2006-01-02")
	if res.CheckIn.IsZero() {
		checkInDateStr = res.CheckInAt.Format("2006-01-02")
	}
	checkOutDateStr := res.CheckOut.Format("2006-01-02")
	if res.CheckOut.IsZero() {
		checkOutDateStr = res.CheckOutAt.Format("2006-01-02")
	}

	return &dtos.TripItineraryResponse{
		ReservationID:     res.ID,
		Code:              res.Code,
		Status:            res.Status,
		ListingID:         res.ListingID,
		ListingTitle:      title,
		CoverPhotoURL:     coverPhotoURL,
		Address:           streetAddress,
		City:              city,
		Country:           country,
		CheckInDate:       checkInDateStr,
		CheckOutDate:      checkOutDateStr,
		CheckInTime:       res.CheckInTime,
		CheckOutTime:      res.CheckOutTime,
		Timezone:          res.Timezone,
		CheckInInstant:    res.CheckInAt,
		CheckOutInstant:   res.CheckOutAt,
		HostName:          hostName,
		HostEmail:         hostEmail,
		Guests:            res.Guests,
		NightlyTotalMinor: res.NightlyTotalMinor,
		CleaningFeeMinor:  res.CleaningFeeMinor,
		DiscountMinor:     res.DiscountMinor,
		GuestFeeMinor:     res.GuestFeeMinor,
		GuestTotalMinor:   res.GuestTotalMinor,
		Currency:          res.Currency,
		Timeline:          timeline,
		Receipts:          receipts,
	}, nil
}

func (s *ItineraryService) buildTimeline(res models.Reservation) []dtos.TimelineDay {
	var timeline []dtos.TimelineDay
	startDate := res.CheckIn
	if startDate.IsZero() {
		startDate = res.CheckInAt
	}
	endDate := res.CheckOut
	if endDate.IsZero() {
		endDate = res.CheckOutAt
	}

	loc, err := time.LoadLocation(res.Timezone)
	if err == nil && loc != nil {
		startDate = startDate.In(loc)
		endDate = endDate.In(loc)
	}

	cur := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, time.UTC)
	last := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, time.UTC)

	checkInStr := cur.Format("2006-01-02")
	checkOutStr := last.Format("2006-01-02")

	for !cur.After(last) {
		dateStr := cur.Format("2006-01-02")
		isCheckIn := (dateStr == checkInStr)
		isCheckOut := (dateStr == checkOutStr)

		var title, desc string
		if isCheckIn {
			title = "Check-in Day"
			desc = fmt.Sprintf("Arrive after %s in %s timezone", res.CheckInTime, res.Timezone)
		} else if isCheckOut {
			title = "Check-out Day"
			desc = fmt.Sprintf("Depart before %s in %s timezone", res.CheckOutTime, res.Timezone)
		} else {
			title = "Stay Night"
			desc = "Enjoy your stay"
		}

		timeline = append(timeline, dtos.TimelineDay{
			Date:        dateStr,
			Title:       title,
			Description: desc,
			IsCheckIn:   isCheckIn,
			IsCheckOut:  isCheckOut,
		})
		cur = cur.AddDate(0, 0, 1)
	}
	return timeline
}
