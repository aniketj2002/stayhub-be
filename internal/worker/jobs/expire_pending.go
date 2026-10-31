package jobs

import (
	"context"
	"time"

	"github.com/stayhub/backend/internal/domain/interfaces"
)

type ExpirePendingJob struct {
	resService interfaces.ReservationService
}

func NewExpirePendingJob(resService interfaces.ReservationService) *ExpirePendingJob {
	return &ExpirePendingJob{resService: resService}
}

func (j *ExpirePendingJob) Name() string {
	return "expire_pending_reservations"
}

func (j *ExpirePendingJob) Run(ctx context.Context, _ time.Time, limit int) (int, error) {
	return j.resService.ExpireDuePendingReservations(ctx, limit)
}
