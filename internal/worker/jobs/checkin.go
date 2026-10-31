package jobs

import (
	"context"
	"time"

	"github.com/stayhub/backend/internal/domain/interfaces"
)

type CheckInJob struct {
	resService interfaces.ReservationService
}

func NewCheckInJob(resService interfaces.ReservationService) *CheckInJob {
	return &CheckInJob{resService: resService}
}

func (j *CheckInJob) Name() string {
	return "process_due_checkins"
}

func (j *CheckInJob) Run(ctx context.Context, _ time.Time, limit int) (int, error) {
	return j.resService.ProcessDueCheckIns(ctx, limit)
}
