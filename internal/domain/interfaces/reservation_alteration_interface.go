package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/models"
)

type ReservationAlterationRepository interface {
	CreateAlteration(ctx context.Context, alt models.ReservationAlteration) (*models.ReservationAlteration, error)
	FindAlterationByID(ctx context.Context, id uuid.UUID) (*models.ReservationAlteration, error)
	FindAlterationByIDForUpdate(ctx context.Context, id uuid.UUID) (*models.ReservationAlteration, error)
	ListAlterationsByReservation(ctx context.Context, reservationID uuid.UUID) ([]models.ReservationAlteration, error)
	FindPendingByReservation(ctx context.Context, reservationID uuid.UUID) (*models.ReservationAlteration, error)
	ListPendingByReservationIDs(ctx context.Context, reservationIDs []uuid.UUID) ([]models.ReservationAlteration, error)
	UpdateAlterationStatus(ctx context.Context, id uuid.UUID, status enums.AlterationStatus, decidedAt *time.Time, reason *string) error
	ListExpiredPending(ctx context.Context, now time.Time) ([]models.ReservationAlteration, error)
}

type ReservationAlterationService interface {
	RequestAlteration(ctx context.Context, guestID, reservationID uuid.UUID, in dtos.CreateReservationAlterationInput) (*dtos.ReservationAlterationResponse, error)
	ListAlterations(ctx context.Context, userID, reservationID uuid.UUID) ([]dtos.ReservationAlterationResponse, error)
	ListPendingForHost(ctx context.Context, hostUserID uuid.UUID) ([]dtos.HostPendingAlteration, error)
	ApproveAlteration(ctx context.Context, hostActor models.AuthActor, alterationID uuid.UUID) (*dtos.ReservationAlterationResponse, error)
	DeclineAlteration(ctx context.Context, hostActor models.AuthActor, alterationID uuid.UUID, in dtos.DeclineReservationAlterationInput) (*dtos.ReservationAlterationResponse, error)
	ExpireDuePendingAlterations(ctx context.Context) (int, error)
}
