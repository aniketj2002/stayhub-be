package interfaces

import (
	"context"
	"time"

	"github.com/stayhub/backend/internal/domain/models"
)

type OutboxRepository interface {
	Insert(ctx context.Context, event models.OutboxEvent) error
	ClaimPending(ctx context.Context, limit int, now time.Time) ([]models.OutboxEvent, error)
	MarkPublished(ctx context.Context, id int64, publishedAt time.Time) error
	MarkFailed(ctx context.Context, id int64, errDetail string, nextRetry time.Time) error
}

type NotificationRepository interface {
	Create(ctx context.Context, n models.Notification) (*models.Notification, error)
	ListByUser(ctx context.Context, userID string, limit, offset int) ([]models.Notification, error)
	MarkAsRead(ctx context.Context, id string, readAt time.Time) error
}
