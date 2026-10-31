package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/db"
)

type OutboxRepository struct {
	db *db.DB
}

func NewOutboxRepository(database *db.DB) interfaces.OutboxRepository {
	return &OutboxRepository{db: database}
}

func (r *OutboxRepository) Insert(ctx context.Context, e models.OutboxEvent) error {
	q := `
		INSERT INTO outbox_events (event_name, aggregate_type, aggregate_id, payload, status, retry_count, next_retry_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	now := time.Now().UTC()
	if e.CreatedAt.IsZero() {
		e.CreatedAt = now
	}
	if e.NextRetryAt.IsZero() {
		e.NextRetryAt = now
	}
	if e.Status == "" {
		e.Status = "pending"
	}
	_, err := r.db.Conn(ctx).Exec(ctx, q, e.EventName, e.AggregateType, e.AggregateID, e.Payload, e.Status, e.RetryCount, e.NextRetryAt, e.CreatedAt)
	return err
}

func (r *OutboxRepository) ClaimPending(ctx context.Context, limit int, now time.Time) ([]models.OutboxEvent, error) {
	q := `
		UPDATE outbox_events
		SET status = 'processing'
		WHERE id IN (
			SELECT id
			FROM outbox_events
			WHERE status IN ('pending', 'processing')
			  AND next_retry_at <= $1
			ORDER BY id ASC
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, event_name, aggregate_type, aggregate_id, payload, status, retry_count, next_retry_at, published_at, error_detail, created_at
	`
	rows, err := r.db.Conn(ctx).Query(ctx, q, now, limit)
	if err != nil {
		return nil, fmt.Errorf("claim pending outbox events: %w", err)
	}
	defer rows.Close()

	var events []models.OutboxEvent
	for rows.Next() {
		var e models.OutboxEvent
		var payloadBytes []byte
		if err := rows.Scan(
			&e.ID,
			&e.EventName,
			&e.AggregateType,
			&e.AggregateID,
			&payloadBytes,
			&e.Status,
			&e.RetryCount,
			&e.NextRetryAt,
			&e.PublishedAt,
			&e.ErrorDetail,
			&e.CreatedAt,
		); err != nil {
			return nil, err
		}
		e.Payload = payloadBytes
		events = append(events, e)
	}
	return events, rows.Err()
}

func (r *OutboxRepository) MarkPublished(ctx context.Context, id int64, publishedAt time.Time) error {
	q := `
		UPDATE outbox_events
		SET status = 'published', published_at = $2
		WHERE id = $1
	`
	_, err := r.db.Conn(ctx).Exec(ctx, q, id, publishedAt)
	return err
}

func (r *OutboxRepository) MarkFailed(ctx context.Context, id int64, errDetail string, nextRetry time.Time) error {
	q := `
		UPDATE outbox_events
		SET status = CASE WHEN retry_count >= 5 THEN 'failed' ELSE 'pending' END,
		    retry_count = retry_count + 1,
		    error_detail = $2,
		    next_retry_at = $3
		WHERE id = $1
	`
	_, err := r.db.Conn(ctx).Exec(ctx, q, id, errDetail, nextRetry)
	return err
}
