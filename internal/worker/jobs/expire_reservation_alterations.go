package jobs

import (
	"context"
	"time"

	"github.com/stayhub/backend/internal/domain/interfaces"
)

type ExpireReservationAlterationsJob struct {
	alterationService interfaces.ReservationAlterationService
}

func NewExpireReservationAlterationsJob(alterationService interfaces.ReservationAlterationService) *ExpireReservationAlterationsJob {
	return &ExpireReservationAlterationsJob{alterationService: alterationService}
}

func (j *ExpireReservationAlterationsJob) Name() string {
	return "expire_reservation_alterations"
}

func (j *ExpireReservationAlterationsJob) Run(ctx context.Context, _ time.Time, _ int) (int, error) {
	return j.alterationService.ExpireDuePendingAlterations(ctx)
}
