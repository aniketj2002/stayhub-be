package models

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/enums"
)

type requestIDCtxKey struct{}

func WithRequestID(ctx context.Context, reqID string) context.Context {
	return context.WithValue(ctx, requestIDCtxKey{}, reqID)
}

func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDCtxKey{}).(string); ok {
		return v
	}
	return ""
}

// EventLog maps to the `event_log` table.
type EventLog struct {
	ID          int64           `db:"id" json:"id"`
	OccurredAt  time.Time       `db:"occurred_at" json:"occurred_at"`
	ActorType   enums.ActorType `db:"actor_type" json:"actor_type"`
	ActorID     *uuid.UUID      `db:"actor_id" json:"actor_id,omitempty"`
	Action      string          `db:"action" json:"action"`
	ObjectType  string          `db:"object_type" json:"object_type"`
	ObjectID    uuid.UUID       `db:"object_id" json:"object_id"`
	BeforeState json.RawMessage `db:"before_state" json:"before_state,omitempty"`
	AfterState  json.RawMessage `db:"after_state" json:"after_state,omitempty"`
	Reason      *string         `db:"reason" json:"reason,omitempty"`
	RequestID   *string         `db:"request_id" json:"request_id,omitempty"`
}
