package jobs

import (
	"context"
	"time"

	"github.com/stayhub/backend/internal/domain/interfaces"
)

type DispatchOutboxJob struct {
	outboxRepo interfaces.OutboxRepository
}

func NewDispatchOutboxJob(outboxRepo interfaces.OutboxRepository) *DispatchOutboxJob {
	return &DispatchOutboxJob{outboxRepo: outboxRepo}
}

func (j *DispatchOutboxJob) Name() string {
	return "dispatch_outbox"
}

func (j *DispatchOutboxJob) Run(ctx context.Context, now time.Time, limit int) (int, error) {
	if j.outboxRepo == nil {
		return 0, nil
	}

	events, err := j.outboxRepo.ClaimPending(ctx, limit, now)
	if err != nil {
		return 0, err
	}

	processed := 0
	for _, e := range events {
		// In Phase 0 / Phase 1: mark as published
		if err := j.outboxRepo.MarkPublished(ctx, e.ID, now); err == nil {
			processed++
		} else {
			_ = j.outboxRepo.MarkFailed(ctx, e.ID, err.Error(), now.Add(5*time.Minute))
		}
	}
	return processed, nil
}
