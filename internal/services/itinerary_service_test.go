package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/models"
)

func TestItineraryService(t *testing.T) {
	guestID := uuid.New()
	hostID := uuid.New()
	unrelatedUserID := uuid.New()
	listingID := uuid.New()
	versionID := uuid.New()
	resID := uuid.New()

	checkIn := time.Date(2026, 11, 10, 15, 0, 0, 0, time.UTC)
	checkOut := time.Date(2026, 11, 13, 11, 0, 0, 0, time.UTC) // 3 nights

	title := "Cozy Alpine Chalet"
	streetAddr := "123 Mountain View Way"
	city := "Zermatt"
	country := "Switzerland"

	listingVersion := models.ListingVersion{
		ID:           versionID,
		ListingID:    listingID,
		Title:        &title,
		AddressLine1: &streetAddr,
		City:         &city,
		Country:      &country,
		Timezone:     "Europe/Zurich",
	}

	listing := models.Listing{
		ID:            listingID,
		HostID:        hostID,
		LiveVersionID: &versionID,
	}

	res := models.Reservation{
		ID:                resID,
		Code:              "RES-ALPINE-101",
		ListingID:         listingID,
		ListingVersionID:  versionID,
		GuestID:           guestID,
		HostID:            hostID,
		Status:            enums.ReservationStatusPending, // pending initially
		CheckInAt:         checkIn,
		CheckOutAt:        checkOut,
		CheckInTime:       "15:00:00",
		CheckOutTime:      "11:00:00",
		Timezone:          "Europe/Zurich",
		Guests:            2,
		Currency:          "EUR",
		NightlyTotalMinor: 30000,
		CleaningFeeMinor:  5000,
		GuestFeeMinor:     4200,
		GuestTotalMinor:   39200,
	}

	resRepo := newMemReservationRepo()
	resRepo.rows[resID] = res

	listingRepo := &memListingRepo{
		listings: map[uuid.UUID]models.Listing{listingID: listing},
		versions: map[uuid.UUID]models.ListingVersion{versionID: listingVersion},
	}

	listingHostRepo := &memListingHostRepo{
		members: map[string]models.ListingHost{
			listingID.String() + ":" + hostID.String(): {
				ListingID: listingID,
				UserID:    hostID,
				Role:      enums.ListingHostRoleOwner,
			},
		},
	}

	userRepo := &memUserRepo{
		users: map[uuid.UUID]models.User{
			guestID: {ID: guestID, FullName: "Alice Guest", Email: "alice@example.com"},
			hostID:  {ID: hostID, FullName: "Bob Alpine Host", Email: "bob@example.com"},
		},
	}

	paymentRepo := newMemPaymentRepo()
	simRef := "sim_chg_12345678"
	_, _ = paymentRepo.CreatePayment(context.Background(), models.Payment{
		ID:             uuid.New(),
		ReservationID:  resID,
		Kind:           enums.PaymentKindCharge,
		AmountMinor:    39200,
		Currency:       "EUR",
		Status:         enums.PaymentStatusSucceeded,
		SimulatedRef:   &simRef,
	})

	svc := NewItineraryService(resRepo, listingRepo, listingHostRepo, userRepo, paymentRepo)

	t.Run("Address is hidden when reservation is pending", func(t *testing.T) {
		itin, err := svc.GetItinerary(context.Background(), guestID, resID)
		if err != nil {
			t.Fatalf("expected no error getting itinerary: %v", err)
		}
		if itin.Address != nil {
			t.Errorf("expected street address to be hidden for pending status, got: %v", *itin.Address)
		}
		if itin.City != "Zermatt" || itin.Country != "Switzerland" {
			t.Errorf("expected city/country to be Zermatt/Switzerland, got %s, %s", itin.City, itin.Country)
		}
	})

	t.Run("Address is revealed once reservation is confirmed", func(t *testing.T) {
		confirmedRes := res
		confirmedRes.Status = enums.ReservationStatusConfirmed
		resRepo.rows[resID] = confirmedRes

		itin, err := svc.GetItinerary(context.Background(), guestID, resID)
		if err != nil {
			t.Fatalf("expected no error getting itinerary: %v", err)
		}
		if itin.Address == nil || *itin.Address != streetAddr {
			t.Fatalf("expected street address %s once confirmed, got %v", streetAddr, itin.Address)
		}
	})

	t.Run("Day-by-day timeline generated properly", func(t *testing.T) {
		itin, err := svc.GetItinerary(context.Background(), guestID, resID)
		if err != nil {
			t.Fatalf("expected no error: %v", err)
		}
		if len(itin.Timeline) != 4 { // checkin, night 1, night 2, checkout
			t.Fatalf("expected 4 timeline days for 3-night stay, got %d", len(itin.Timeline))
		}
		if !itin.Timeline[0].IsCheckIn {
			t.Errorf("expected first timeline entry to be check-in")
		}
		if !itin.Timeline[len(itin.Timeline)-1].IsCheckOut {
			t.Errorf("expected last timeline entry to be check-out")
		}
	})

	t.Run("Receipts and total paid match payments", func(t *testing.T) {
		itin, err := svc.GetItinerary(context.Background(), guestID, resID)
		if err != nil {
			t.Fatalf("expected no error: %v", err)
		}
		if len(itin.Receipts) != 1 {
			t.Fatalf("expected 1 receipt entry, got %d", len(itin.Receipts))
		}
		if itin.Receipts[0].AmountMinor != 39200 {
			t.Errorf("expected receipt amount 39200, got %d", itin.Receipts[0].AmountMinor)
		}
	})

	t.Run("Unrelated user is forbidden to view itinerary", func(t *testing.T) {
		_, err := svc.GetItinerary(context.Background(), unrelatedUserID, resID)
		if err == nil {
			t.Fatal("expected forbidden error for unrelated user")
		}
	})
}
