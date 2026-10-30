package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/clock"
)

type AuditService struct {
	repo  interfaces.AuditRepository
	clock clock.Clock
}

func NewAuditService(repo interfaces.AuditRepository, clk clock.Clock) interfaces.AuditService {
	return &AuditService{
		repo:  repo,
		clock: clk,
	}
}

func (s *AuditService) Record(ctx context.Context, in dtos.AuditEntryInput) error {
	var beforeBytes []byte
	var err error
	if in.BeforeState != nil {
		beforeBytes, err = json.Marshal(in.BeforeState)
		if err != nil {
			return fmt.Errorf("marshal before_state: %w", err)
		}
	}

	var afterBytes []byte
	if in.AfterState != nil {
		afterBytes, err = json.Marshal(in.AfterState)
		if err != nil {
			return fmt.Errorf("marshal after_state: %w", err)
		}
	}

	reqID := in.RequestID
	if reqID == "" {
		reqID = models.RequestIDFromContext(ctx)
	}
	var reqIDPtr *string
	if reqID != "" {
		reqIDPtr = &reqID
	}

	return s.repo.Insert(ctx, dtos.AuditInsertParams{
		OccurredAt:  s.clock.Now(),
		ActorType:   in.ActorType,
		ActorID:     in.ActorID,
		Action:      in.Action,
		ObjectType:  in.ObjectType,
		ObjectID:    in.ObjectID,
		BeforeBytes: beforeBytes,
		AfterBytes:  afterBytes,
		Reason:      in.Reason,
		RequestID:   reqIDPtr,
	})
}

func (s *AuditService) ListEvents(ctx context.Context, f dtos.EventListFilter) ([]models.EventLog, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	return s.repo.FindMany(ctx, f)
}
