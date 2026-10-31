package interfaces

import (
	"context"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/models"
)

type PayoutService interface {
	SchedulePayout(ctx context.Context, res models.Reservation, idemKey string) (*models.Payout, error)
	HoldPayout(ctx context.Context, adminID uuid.UUID, payoutID uuid.UUID, reason string) (*models.Payout, error)
	ReleasePayoutHold(ctx context.Context, adminID uuid.UUID, payoutID uuid.UUID, reason string) (*models.Payout, error)
	ReleaseDuePayouts(ctx context.Context, limit int) (int, error)
	AdjustPayoutOnRefund(ctx context.Context, res models.Reservation, refundedHostNetMinor int64) error
	ProcessPayoutRelease(ctx context.Context, payoutID uuid.UUID) error
}
