package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/config"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/clock"
)

var (
	ErrProposerCannotApprove = errors.New("the proposer cannot approve their own fee change")
	ErrFeeConfigNotPending   = errors.New("fee configuration is not pending approval")
)

type FeeConfigService struct {
	cfg   *config.Config
	repo  interfaces.FeeConfigRepository
	audit interfaces.AuditService
	clock clock.Clock
}

func NewFeeConfigService(
	cfg *config.Config,
	repo interfaces.FeeConfigRepository,
	audit interfaces.AuditService,
	clk clock.Clock,
) interfaces.FeeConfigService {
	return &FeeConfigService{
		cfg:   cfg,
		repo:  repo,
		audit: audit,
		clock: clk,
	}
}

func (s *FeeConfigService) GetActive(ctx context.Context) (*models.FeeConfig, error) {
	fc, err := s.repo.GetActive(ctx)
	if err == nil && fc != nil {
		return fc, nil
	}
	// Fallback to config environment variables if table has no active row
	now := s.clock.Now()
	fallback := &models.FeeConfig{
		ID:             uuid.Nil,
		GuestFeeBPS:    s.cfg.GuestFeeBPS,
		HostFeeBPS:     s.cfg.HostFeeBPS,
		Status:         enums.FeeConfigStatusActive,
		ProposedReason: "Environment fallback",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	return fallback, nil
}

func (s *FeeConfigService) List(ctx context.Context) ([]models.FeeConfig, error) {
	return s.repo.List(ctx)
}

func (s *FeeConfigService) Propose(
	ctx context.Context,
	adminID uuid.UUID,
	guestFeeBPS, hostFeeBPS int,
	reason string,
) (*models.FeeConfig, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, fmt.Errorf("%w: proposal reason is required", apperrors.ErrValidation)
	}
	if guestFeeBPS < 0 || hostFeeBPS < 0 {
		return nil, fmt.Errorf("%w: fees must not be negative", apperrors.ErrValidation)
	}

	now := s.clock.Now()
	newFC := models.FeeConfig{
		ID:             uuid.New(),
		GuestFeeBPS:    guestFeeBPS,
		HostFeeBPS:     hostFeeBPS,
		Status:         enums.FeeConfigStatusPendingApproval,
		ProposedBy:     adminID,
		ProposedReason: reason,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	created, err := s.repo.Create(ctx, newFC)
	if err != nil {
		return nil, err
	}

	if s.audit != nil {
		_ = s.audit.Record(ctx, dtos.AuditEntryInput{
			ActorType:  enums.ActorAdmin,
			ActorID:    &adminID,
			Action:     "fee_config.propose",
			ObjectType: "fee_config",
			ObjectID:   created.ID,
			AfterState: map[string]any{
				"guest_fee_bps":   created.GuestFeeBPS,
				"host_fee_bps":    created.HostFeeBPS,
				"status":          created.Status,
				"proposed_reason": created.ProposedReason,
			},
			Reason: &reason,
		})
	}

	return created, nil
}

func (s *FeeConfigService) Approve(
	ctx context.Context,
	adminID uuid.UUID,
	id uuid.UUID,
) (*models.FeeConfig, error) {
	fc, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if fc.Status != enums.FeeConfigStatusPendingApproval {
		return nil, ErrFeeConfigNotPending
	}
	// Two-person rule enforced in service as well as DB constraint
	if fc.ProposedBy == adminID {
		return nil, ErrProposerCannotApprove
	}

	now := s.clock.Now()
	activated, err := s.repo.Activate(ctx, id, adminID, now)
	if err != nil {
		return nil, err
	}

	if s.audit != nil {
		reason := "Fee configuration approved"
		_ = s.audit.Record(ctx, dtos.AuditEntryInput{
			ActorType:   enums.ActorAdmin,
			ActorID:     &adminID,
			Action:      "fee_config.approve",
			ObjectType:  "fee_config",
			ObjectID:    activated.ID,
			BeforeState: map[string]any{"status": fc.Status},
			AfterState: map[string]any{
				"status":        activated.Status,
				"approved_by":   activated.ApprovedBy,
				"approved_at":   activated.ApprovedAt,
				"guest_fee_bps": activated.GuestFeeBPS,
				"host_fee_bps":  activated.HostFeeBPS,
			},
			Reason: &reason,
		})
	}

	return activated, nil
}

func (s *FeeConfigService) Reject(
	ctx context.Context,
	adminID uuid.UUID,
	id uuid.UUID,
	reason string,
) (*models.FeeConfig, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, fmt.Errorf("%w: rejection reason is required", apperrors.ErrValidation)
	}

	fc, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if fc.Status != enums.FeeConfigStatusPendingApproval {
		return nil, ErrFeeConfigNotPending
	}

	now := s.clock.Now()
	fc.Status = enums.FeeConfigStatusRejected
	fc.RejectionReason = &reason
	fc.UpdatedAt = now

	updated, err := s.repo.Update(ctx, *fc)
	if err != nil {
		return nil, err
	}

	if s.audit != nil {
		_ = s.audit.Record(ctx, dtos.AuditEntryInput{
			ActorType:   enums.ActorAdmin,
			ActorID:     &adminID,
			Action:      "fee_config.reject",
			ObjectType:  "fee_config",
			ObjectID:    updated.ID,
			BeforeState: map[string]any{"status": enums.FeeConfigStatusPendingApproval},
			AfterState: map[string]any{
				"status":           updated.Status,
				"rejection_reason": updated.RejectionReason,
			},
			Reason: &reason,
		})
	}

	return updated, nil
}
