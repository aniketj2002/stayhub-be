package worker

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/clock"
)

type stubReservationService struct {
	expired  int
	checkIn  int
	checkOut int
	payouts  int
}

func (s *stubReservationService) CreateReservation(_ context.Context, _ models.AuthActor, _ string, _ dtos.CreateReservationInput) (*dtos.ReservationDetailResponse, error) {
	return nil, nil
}
func (s *stubReservationService) ListGuestTrips(_ context.Context, _ uuid.UUID, _ string) ([]dtos.ReservationDetailResponse, error) {
	return nil, nil
}
func (s *stubReservationService) GetReservationDetail(_ context.Context, _ models.AuthActor, _ uuid.UUID) (*dtos.ReservationDetailResponse, error) {
	return nil, nil
}
func (s *stubReservationService) GetCancelPreview(_ context.Context, _, _ uuid.UUID) (*dtos.CancelPreviewResponse, error) {
	return nil, nil
}
func (s *stubReservationService) CancelByGuest(_ context.Context, _, _ uuid.UUID, _ string, _ dtos.CancelReservationInput) (*dtos.ReservationDetailResponse, error) {
	return nil, nil
}
func (s *stubReservationService) GetHostToday(_ context.Context, _ uuid.UUID) (*dtos.HostTodayResponse, error) {
	return nil, nil
}
func (s *stubReservationService) AcceptByHost(_ context.Context, _, _ uuid.UUID, _ string) (*dtos.ReservationDetailResponse, error) {
	return nil, nil
}
func (s *stubReservationService) DeclineByHost(_ context.Context, _, _ uuid.UUID, _ string, _ dtos.DeclineReservationInput) (*dtos.ReservationDetailResponse, error) {
	return nil, nil
}
func (s *stubReservationService) CancelByHost(_ context.Context, _, _ uuid.UUID, _ string, _ dtos.CancelReservationInput) (*dtos.ReservationDetailResponse, error) {
	return nil, nil
}
func (s *stubReservationService) CheckInByHost(_ context.Context, _, _ uuid.UUID, _ string) (*dtos.ReservationDetailResponse, error) {
	return nil, nil
}
func (s *stubReservationService) GetHostEarnings(_ context.Context, _ uuid.UUID) (*dtos.HostEarningsResponse, error) {
	return nil, nil
}
func (s *stubReservationService) ExpireDuePendingReservations(_ context.Context, _ int) (int, error) {
	return s.expired, nil
}
func (s *stubReservationService) ProcessDueCheckIns(_ context.Context, _ int) (int, error) {
	return s.checkIn, nil
}
func (s *stubReservationService) ProcessDueCheckOuts(_ context.Context, _ int) (int, error) {
	return s.checkOut, nil
}
func (s *stubReservationService) ReleaseDuePayouts(_ context.Context, _ int) (int, error) {
	return s.payouts, nil
}

func TestSchedulerTick(t *testing.T) {
	stub := &stubReservationService{
		expired:  2,
		checkIn:  3,
		checkOut: 1,
		payouts:  4,
	}
	scheduler := NewScheduler(stub, nil, clock.NewMockClock(time.Now().UTC()), time.Second, 50)

	res, err := scheduler.Tick(context.Background())
	if err != nil {
		t.Fatalf("unexpected tick error: %v", err)
	}
	if res.ExpiredPending != 2 || res.CheckedIn != 3 || res.CheckedOut != 1 {
		t.Fatalf("unexpected tick counts: %+v", res)
	}
}
