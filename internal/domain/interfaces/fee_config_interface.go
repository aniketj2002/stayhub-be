package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/models"
)

type FeeConfigRepository interface {
	GetActive(ctx context.Context) (*models.FeeConfig, error)
	FindByID(ctx context.Context, id uuid.UUID) (*models.FeeConfig, error)
	FindByIDForUpdate(ctx context.Context, id uuid.UUID) (*models.FeeConfig, error)
	List(ctx context.Context) ([]models.FeeConfig, error)
	Create(ctx context.Context, cfg models.FeeConfig) (*models.FeeConfig, error)
	Update(ctx context.Context, cfg models.FeeConfig) (*models.FeeConfig, error)
	Activate(ctx context.Context, id uuid.UUID, approverID uuid.UUID, now time.Time) (*models.FeeConfig, error)
}

type FeeConfigService interface {
	GetActive(ctx context.Context) (*models.FeeConfig, error)
	List(ctx context.Context) ([]models.FeeConfig, error)
	Propose(ctx context.Context, adminID uuid.UUID, guestFeeBPS, hostFeeBPS int, reason string) (*models.FeeConfig, error)
	Approve(ctx context.Context, adminID uuid.UUID, id uuid.UUID) (*models.FeeConfig, error)
	Reject(ctx context.Context, adminID uuid.UUID, id uuid.UUID, reason string) (*models.FeeConfig, error)
}
