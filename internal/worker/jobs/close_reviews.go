package jobs

import (
	"context"
	"time"

	"github.com/stayhub/backend/internal/domain/interfaces"
)

type CloseReviewWindowsJob struct {
	reviewService interfaces.ReviewService
}

func NewCloseReviewWindowsJob(reviewService interfaces.ReviewService) *CloseReviewWindowsJob {
	return &CloseReviewWindowsJob{reviewService: reviewService}
}

func (j *CloseReviewWindowsJob) Name() string {
	return "close_review_windows"
}

func (j *CloseReviewWindowsJob) Run(ctx context.Context, _ time.Time, _ int) (int, error) {
	return j.reviewService.CloseReviewWindows(ctx)
}
