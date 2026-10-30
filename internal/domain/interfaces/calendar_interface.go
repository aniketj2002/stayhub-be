package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/models"
)

type CalendarRepository interface {
	ListRange(ctx context.Context, listingID uuid.UUID, startDate, endDate time.Time) ([]models.ListingCalendar, error)
	BlockNights(ctx context.Context, listingID uuid.UUID, nights []time.Time, reason *string) error
	UnblockNights(ctx context.Context, listingID uuid.UUID, startDate, endDate time.Time) error
	BookNights(ctx context.Context, listingID, reservationID uuid.UUID, nights []time.Time) error
	ReleaseReservationNights(ctx context.Context, reservationID uuid.UUID) error
}

type CalendarService interface {
	GetCalendar(ctx context.Context, listingID uuid.UUID, in dtos.CalendarQueryInput) (*dtos.CalendarResponse, error)
	BlockDates(ctx context.Context, listingID uuid.UUID, hostAccess models.ListingHost, in dtos.CalendarBlockInput) (*dtos.CalendarResponse, error)
	UnblockDates(ctx context.Context, listingID uuid.UUID, hostAccess models.ListingHost, in dtos.CalendarUnblockInput) (*dtos.CalendarResponse, error)
}
