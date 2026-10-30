package interfaces

import (
	"context"

	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/models"
)

type AuditRepository interface {
	Insert(ctx context.Context, p dtos.AuditInsertParams) error
	FindMany(ctx context.Context, f dtos.EventListFilter) ([]models.EventLog, error)
}

type AuditService interface {
	Record(ctx context.Context, in dtos.AuditEntryInput) error
	ListEvents(ctx context.Context, f dtos.EventListFilter) ([]models.EventLog, error)
}
