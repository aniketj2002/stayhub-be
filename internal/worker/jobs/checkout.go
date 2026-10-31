package jobs

import (
	"context"
	"time"

	"github.com/stayhub/backend/internal/domain/interfaces"
)

type CheckOutJob struct {
	resService interfaces.ReservationService
}

func NewCheckOutJob(resService interfaces.ReservationService) *CheckOutJob {
	return &CheckOutJob{resService: resService}
}

func (j *CheckOutJob) Name() string {
	return "process_due_checkouts"
}

func (j *CheckOutJob) Run(ctx context.Context, _ time.Time, limit int) (int, error) {
	return j.resService.ProcessDueCheckOuts(ctx, limit)
}
