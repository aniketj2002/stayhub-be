package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/models"
)

type ReservationRepository interface {
	Create(ctx context.Context, r models.Reservation) (*models.Reservation, error)
	FindByID(ctx context.Context, id uuid.UUID) (*models.Reservation, error)
	FindByIDForUpdate(ctx context.Context, id uuid.UUID) (*models.Reservation, error)
	Update(ctx context.Context, r models.Reservation) (*models.Reservation, error)
	ListByGuest(ctx context.Context, guestID uuid.UUID) ([]models.Reservation, error)
	ListByListingIDs(ctx context.Context, listingIDs []uuid.UUID) ([]models.Reservation, error)
	ListDuePendingExpirations(ctx context.Context, now time.Time, limit int) ([]models.Reservation, error)
	ListDueCheckIns(ctx context.Context, now time.Time, limit int) ([]models.Reservation, error)
	ListDueCheckOuts(ctx context.Context, now time.Time, limit int) ([]models.Reservation, error)
}

type PaymentRepository interface {
	CreatePayment(ctx context.Context, p models.Payment) (*models.Payment, error)
	FindChargeByReservation(ctx context.Context, reservationID uuid.UUID) (*models.Payment, error)
	ListPaymentsByReservation(ctx context.Context, reservationID uuid.UUID) ([]models.Payment, error)

	CreatePayout(ctx context.Context, p models.Payout) (*models.Payout, error)
	FindPayoutByReservation(ctx context.Context, reservationID uuid.UUID) (*models.Payout, error)
	FindPayoutByIDForUpdate(ctx context.Context, id uuid.UUID) (*models.Payout, error)
	UpdatePayout(ctx context.Context, p models.Payout) (*models.Payout, error)
	ListPayoutsByListingIDs(ctx context.Context, listingIDs []uuid.UUID) ([]dtos.EarningsPayoutItem, error)
	ListDuePayouts(ctx context.Context, now time.Time, limit int) ([]models.Payout, error)

	InsertLedgerEntries(ctx context.Context, entries []models.LedgerEntry) error
	ListLedgerByReservation(ctx context.Context, reservationID uuid.UUID) ([]models.LedgerEntry, error)
}

type ReservationService interface {
	CreateReservation(ctx context.Context, actor models.AuthActor, idemKey string, in dtos.CreateReservationInput) (*dtos.ReservationDetailResponse, error)
	ListGuestTrips(ctx context.Context, guestID uuid.UUID, tab string) ([]dtos.ReservationDetailResponse, error)
	GetReservationDetail(ctx context.Context, actor models.AuthActor, reservationID uuid.UUID) (*dtos.ReservationDetailResponse, error)
	GetCancelPreview(ctx context.Context, guestID, reservationID uuid.UUID) (*dtos.CancelPreviewResponse, error)
	CancelByGuest(ctx context.Context, guestID, reservationID uuid.UUID, idemKey string, in dtos.CancelReservationInput) (*dtos.ReservationDetailResponse, error)

	GetHostToday(ctx context.Context, hostUserID uuid.UUID) (*dtos.HostTodayResponse, error)
	AcceptByHost(ctx context.Context, hostUserID, reservationID uuid.UUID, idemKey string) (*dtos.ReservationDetailResponse, error)
	DeclineByHost(ctx context.Context, hostUserID, reservationID uuid.UUID, idemKey string, in dtos.DeclineReservationInput) (*dtos.ReservationDetailResponse, error)
	CancelByHost(ctx context.Context, hostUserID, reservationID uuid.UUID, idemKey string, in dtos.CancelReservationInput) (*dtos.ReservationDetailResponse, error)
	CheckInByHost(ctx context.Context, hostUserID, reservationID uuid.UUID, idemKey string) (*dtos.ReservationDetailResponse, error)
	GetHostEarnings(ctx context.Context, hostUserID uuid.UUID) (*dtos.HostEarningsResponse, error)

	ExpireDuePendingReservations(ctx context.Context, limit int) (int, error)
	ProcessDueCheckIns(ctx context.Context, limit int) (int, error)
	ProcessDueCheckOuts(ctx context.Context, limit int) (int, error)
	ReleaseDuePayouts(ctx context.Context, limit int) (int, error)
}
