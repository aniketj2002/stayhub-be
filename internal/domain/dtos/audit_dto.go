package dtos

import (
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/enums"
)

type AuditEntryInput struct {
	ActorType   enums.ActorType
	ActorID     *uuid.UUID
	Action      string
	ObjectType  string
	ObjectID    uuid.UUID
	BeforeState any
	AfterState  any
	Reason      *string
	RequestID   string
}

type AuditInsertParams struct {
	OccurredAt  time.Time
	ActorType   enums.ActorType
	ActorID     *uuid.UUID
	Action      string
	ObjectType  string
	ObjectID    uuid.UUID
	BeforeBytes []byte
	AfterBytes  []byte
	Reason      *string
	RequestID   *string
}

type EventListFilter struct {
	ObjectType string
	ObjectID   *uuid.UUID
	ActorType  string
	ActorID    *uuid.UUID
	Action     string
	Query      string
	From       *time.Time
	To         *time.Time
	Limit      int
	Offset     int
}
