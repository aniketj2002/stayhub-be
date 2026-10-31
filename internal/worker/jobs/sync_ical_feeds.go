package jobs

import (
	"context"
	"time"

	"github.com/stayhub/backend/internal/domain/interfaces"
)

type SyncICalFeedsJob struct {
	icalService interfaces.ICalService
}

func NewSyncICalFeedsJob(icalService interfaces.ICalService) *SyncICalFeedsJob {
	return &SyncICalFeedsJob{icalService: icalService}
}

func (j *SyncICalFeedsJob) Name() string {
	return "sync_ical_feeds"
}

func (j *SyncICalFeedsJob) Run(ctx context.Context, _ time.Time, _ int) (int, error) {
	return j.icalService.SyncAllFeeds(ctx)
}
