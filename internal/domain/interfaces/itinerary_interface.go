package interfaces

import (
	"context"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/dtos"
)

type ItineraryService interface {
	GetItinerary(ctx context.Context, userID, reservationID uuid.UUID) (*dtos.TripItineraryResponse, error)
}
