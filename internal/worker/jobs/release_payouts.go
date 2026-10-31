package jobs

import (
	"context"
	"time"

	"github.com/stayhub/backend/internal/domain/interfaces"
)

type ReleasePayoutsJob struct {
	payoutService interfaces.PayoutService
}

func NewReleasePayoutsJob(payoutService interfaces.PayoutService) *ReleasePayoutsJob {
	return &ReleasePayoutsJob{payoutService: payoutService}
}

func (j *ReleasePayoutsJob) Name() string {
	return "release_due_payouts"
}

func (j *ReleasePayoutsJob) Run(ctx context.Context, _ time.Time, limit int) (int, error) {
	return j.payoutService.ReleaseDuePayouts(ctx, limit)
}
