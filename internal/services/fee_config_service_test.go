package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/config"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/clock"
)

type memFeeConfigRepo struct {
	rows map[uuid.UUID]models.FeeConfig
}

func newMemFeeConfigRepo() *memFeeConfigRepo {
	return &memFeeConfigRepo{rows: make(map[uuid.UUID]models.FeeConfig)}
}

func (r *memFeeConfigRepo) GetActive(_ context.Context) (*models.FeeConfig, error) {
	for _, fc := range r.rows {
		if fc.Status == enums.FeeConfigStatusActive {
			cp := fc
			return &cp, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (r *memFeeConfigRepo) FindByID(_ context.Context, id uuid.UUID) (*models.FeeConfig, error) {
	fc, ok := r.rows[id]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	cp := fc
	return &cp, nil
}

func (r *memFeeConfigRepo) FindByIDForUpdate(ctx context.Context, id uuid.UUID) (*models.FeeConfig, error) {
	return r.FindByID(ctx, id)
}

func (r *memFeeConfigRepo) List(_ context.Context) ([]models.FeeConfig, error) {
	var out []models.FeeConfig
	for _, fc := range r.rows {
		out = append(out, fc)
	}
	return out, nil
}

func (r *memFeeConfigRepo) Create(_ context.Context, cfg models.FeeConfig) (*models.FeeConfig, error) {
	if cfg.ID == uuid.Nil {
		cfg.ID = uuid.New()
	}
	r.rows[cfg.ID] = cfg
	cp := cfg
	return &cp, nil
}

func (r *memFeeConfigRepo) Update(_ context.Context, cfg models.FeeConfig) (*models.FeeConfig, error) {
	r.rows[cfg.ID] = cfg
	cp := cfg
	return &cp, nil
}

func (r *memFeeConfigRepo) Activate(_ context.Context, id uuid.UUID, approverID uuid.UUID, now time.Time) (*models.FeeConfig, error) {
	for k, v := range r.rows {
		if v.Status == enums.FeeConfigStatusActive {
			v.Status = enums.FeeConfigStatusSuperseded
			v.UpdatedAt = now
			r.rows[k] = v
		}
	}
	target, ok := r.rows[id]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	target.Status = enums.FeeConfigStatusActive
	target.ApprovedBy = &approverID
	target.ApprovedAt = &now
	target.UpdatedAt = now
	r.rows[id] = target
	cp := target
	return &cp, nil
}

func TestFeeConfigService(t *testing.T) {
	cfg := &config.Config{GuestFeeBPS: 1400, HostFeeBPS: 300}
	repo := newMemFeeConfigRepo()
	clk := clock.NewMockClock(time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC))
	svc := NewFeeConfigService(cfg, repo, nil, clk)

	ctx := context.Background()
	admin1 := uuid.New()
	admin2 := uuid.New()

	// 1. Fallback to env if empty
	active, err := svc.GetActive(ctx)
	if err != nil {
		t.Fatalf("unexpected error getting active: %v", err)
	}
	if active.GuestFeeBPS != 1400 || active.HostFeeBPS != 300 {
		t.Errorf("expected 1400/300, got %d/%d", active.GuestFeeBPS, active.HostFeeBPS)
	}

	// 2. Propose new config
	proposed, err := svc.Propose(ctx, admin1, 1200, 250, "Seasonal reduction")
	if err != nil {
		t.Fatalf("unexpected error proposing: %v", err)
	}
	if proposed.Status != enums.FeeConfigStatusPendingApproval {
		t.Errorf("expected status pending_approval, got %s", proposed.Status)
	}

	// Propose validation failure: empty reason
	if _, err := svc.Propose(ctx, admin1, 1000, 200, "  "); !errors.Is(err, apperrors.ErrValidation) {
		t.Errorf("expected validation error on empty reason, got %v", err)
	}

	// 3. Proposer cannot approve their own proposal
	if _, err := svc.Approve(ctx, admin1, proposed.ID); !errors.Is(err, ErrProposerCannotApprove) {
		t.Errorf("expected ErrProposerCannotApprove, got %v", err)
	}

	// 4. Second admin approves successfully
	approved, err := svc.Approve(ctx, admin2, proposed.ID)
	if err != nil {
		t.Fatalf("unexpected error on second admin approval: %v", err)
	}
	if approved.Status != enums.FeeConfigStatusActive {
		t.Errorf("expected active, got %s", approved.Status)
	}

	// 5. Active is now the newly approved row
	activeNow, err := svc.GetActive(ctx)
	if err != nil {
		t.Fatalf("unexpected error getting active: %v", err)
	}
	if activeNow.ID != proposed.ID || activeNow.GuestFeeBPS != 1200 {
		t.Errorf("expected new active config with 1200 bps, got %d bps", activeNow.GuestFeeBPS)
	}

	// 6. Propose and Reject
	prop2, err := svc.Propose(ctx, admin1, 1500, 400, "Increase fees")
	if err != nil {
		t.Fatalf("unexpected error proposing prop2: %v", err)
	}
	rejected, err := svc.Reject(ctx, admin2, prop2.ID, "Unjustified hike")
	if err != nil {
		t.Fatalf("unexpected error rejecting: %v", err)
	}
	if rejected.Status != enums.FeeConfigStatusRejected {
		t.Errorf("expected rejected status, got %s", rejected.Status)
	}
}
