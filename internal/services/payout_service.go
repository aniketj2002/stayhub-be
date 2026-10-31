package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/config"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/domain/statemachine"
	platformaws "github.com/stayhub/backend/internal/platform/aws"
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/db"
	"github.com/stayhub/backend/internal/platform/redis"
)

var (
	ErrPayoutNotHeld = errors.New("payout is not currently held")
)

type PayoutService struct {
	cfg             *config.Config
	db              *db.DB
	rdb             *redis.Client
	paymentRepo     interfaces.PaymentRepository
	resRepo         interfaces.ReservationRepository
	ledger          interfaces.LedgerService
	audit           interfaces.AuditService
	clock           clock.Clock
	payoutScheduler platformaws.PayoutScheduler
	hostRepo        interfaces.ListingHostRepository
	cohostRepo      interfaces.CohostRepository
}

func NewPayoutService(
	cfg *config.Config,
	database *db.DB,
	rdb *redis.Client,
	paymentRepo interfaces.PaymentRepository,
	resRepo interfaces.ReservationRepository,
	ledger interfaces.LedgerService,
	audit interfaces.AuditService,
	clk clock.Clock,
	payoutScheduler ...platformaws.PayoutScheduler,
) interfaces.PayoutService {
	var ps platformaws.PayoutScheduler
	if len(payoutScheduler) > 0 {
		ps = payoutScheduler[0]
	}
	return &PayoutService{
		cfg:             cfg,
		db:              database,
		rdb:             rdb,
		paymentRepo:     paymentRepo,
		resRepo:         resRepo,
		ledger:          ledger,
		audit:           audit,
		clock:           clk,
		payoutScheduler: ps,
	}
}

func (s *PayoutService) SetCohostDependencies(hostRepo interfaces.ListingHostRepository, cohostRepo interfaces.CohostRepository) {
	s.hostRepo = hostRepo
	s.cohostRepo = cohostRepo
}

func (s *PayoutService) SchedulePayout(
	ctx context.Context,
	res models.Reservation,
	idemKey string,
) (*models.Payout, error) {
	grossMinor := res.NightlyTotalMinor + res.CleaningFeeMinor
	payout := models.Payout{
		ReservationID:  res.ID,
		HostID:         res.HostID,
		Status:         enums.PayoutStatusScheduled,
		GrossMinor:     grossMinor,
		HostFeeMinor:   res.HostFeeMinor,
		NetMinor:       res.HostNetMinor,
		Currency:       res.Currency,
		IdempotencyKey: fmt.Sprintf("payout:%s:%s", res.ID.String(), idemKey),
	}

	created, err := s.paymentRepo.CreatePayout(ctx, payout)
	if err != nil {
		return nil, err
	}

	// Schedule co-host splits if any
	if s.hostRepo != nil && s.cohostRepo != nil && res.ListingID != uuid.Nil {
		members, err := s.hostRepo.ListByListing(ctx, res.ListingID)
		if err == nil {
			var cohostSplits []models.PayoutSplit
			var totalCohostShareBPS int
			var totalCohostAmount int64

			for _, m := range members {
				if m.Role == enums.ListingHostRoleCohost && m.PayoutShareBPS > 0 {
					splitAmount := models.MulBPS(res.HostNetMinor, m.PayoutShareBPS)
					totalCohostAmount += splitAmount
					totalCohostShareBPS += m.PayoutShareBPS
					cohostSplits = append(cohostSplits, models.PayoutSplit{
						PayoutID:    created.ID,
						UserID:      m.UserID,
						ShareBPS:    m.PayoutShareBPS,
						AmountMinor: splitAmount,
						Status:      enums.PayoutSplitScheduled,
					})
				}
			}

			if len(cohostSplits) > 0 {
				ownerShareBPS := 10000 - totalCohostShareBPS
				if ownerShareBPS < 0 {
					ownerShareBPS = 0
				}
				ownerAmount := res.HostNetMinor - totalCohostAmount
				if ownerAmount < 0 {
					ownerAmount = 0
				}
				allSplits := append(cohostSplits, models.PayoutSplit{
					PayoutID:    created.ID,
					UserID:      res.HostID,
					ShareBPS:    ownerShareBPS,
					AmountMinor: ownerAmount,
					Status:      enums.PayoutSplitScheduled,
				})
				_ = s.cohostRepo.CreatePayoutSplits(ctx, allSplits)
			}
		}
	}

	if s.audit != nil {
		_ = s.audit.Record(ctx, dtos.AuditEntryInput{
			ActorType:  enums.ActorSystem,
			ActorID:    nil,
			Action:     "payout.schedule",
			ObjectType: "payout",
			ObjectID:   created.ID,
			AfterState: map[string]any{
				"reservation_id": created.ReservationID,
				"host_id":        created.HostID,
				"net_minor":      created.NetMinor,
				"currency":       created.Currency,
				"status":         created.Status,
			},
		})
	}

	return created, nil
}

func (s *PayoutService) HoldPayout(
	ctx context.Context,
	adminID uuid.UUID,
	payoutID uuid.UUID,
	reason string,
) (*models.Payout, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, fmt.Errorf("%w: reason is required to hold payout", apperrors.ErrValidation)
	}

	var updated *models.Payout
	err := s.db.WithinTx(ctx, func(txCtx context.Context) error {
		p, err := s.paymentRepo.FindPayoutByIDForUpdate(txCtx, payoutID)
		if err != nil {
			return err
		}

		if err := statemachine.ValidatePayoutTransition(p.Status, enums.PayoutStatusHeld); err != nil {
			return err
		}

		beforeStatus := p.Status
		now := s.clock.Now()
		p.Status = enums.PayoutStatusHeld
		p.HeldBy = &adminID
		p.HeldReason = &reason
		p.UpdatedAt = now

		up, err := s.paymentRepo.UpdatePayout(txCtx, *p)
		if err != nil {
			return err
		}
		updated = up

		if s.payoutScheduler != nil {
			_ = s.payoutScheduler.CancelPayoutSchedule(txCtx, payoutID)
		}

		if s.audit != nil {
			_ = s.audit.Record(txCtx, dtos.AuditEntryInput{
				ActorType:   enums.ActorAdmin,
				ActorID:     &adminID,
				Action:      "payout.hold",
				ObjectType:  "payout",
				ObjectID:    up.ID,
				BeforeState: map[string]any{"status": beforeStatus},
				AfterState: map[string]any{
					"status":      up.Status,
					"held_by":     up.HeldBy,
					"held_reason": up.HeldReason,
				},
				Reason: &reason,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return updated, nil
}

func (s *PayoutService) ReleasePayoutHold(
	ctx context.Context,
	adminID uuid.UUID,
	payoutID uuid.UUID,
	reason string,
) (*models.Payout, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, fmt.Errorf("%w: reason is required to release payout hold", apperrors.ErrValidation)
	}

	var updated *models.Payout
	err := s.db.WithinTx(ctx, func(txCtx context.Context) error {
		p, err := s.paymentRepo.FindPayoutByIDForUpdate(txCtx, payoutID)
		if err != nil {
			return err
		}

		if p.Status != enums.PayoutStatusHeld {
			return ErrPayoutNotHeld
		}

		res, err := s.resRepo.FindByID(txCtx, p.ReservationID)
		if err != nil {
			return err
		}

		now := s.clock.Now()
		// Recompute release_at = max(now, checked_in_at + 24h)
		releaseAt := now
		if res.CheckedInAt != nil {
			scheduledRelease := res.CheckedInAt.Add(time.Duration(s.cfg.PayoutReleaseAfterCheckInHours) * time.Hour)
			if scheduledRelease.After(now) {
				releaseAt = scheduledRelease
			}
		}

		beforeStatus := p.Status
		p.Status = enums.PayoutStatusScheduled
		p.HeldBy = nil
		p.HeldReason = nil
		p.ReleaseAt = &releaseAt
		p.UpdatedAt = now

		up, err := s.paymentRepo.UpdatePayout(txCtx, *p)
		if err != nil {
			return err
		}
		updated = up

		if s.payoutScheduler != nil && updated != nil && updated.ReleaseAt != nil {
			if err := s.payoutScheduler.SchedulePayout(txCtx, updated.ID, updated.ReservationID, *updated.ReleaseAt); err != nil {
				return fmt.Errorf("failed to schedule payout in EventBridge: %w", err)
			}
		}

		if s.audit != nil {
			_ = s.audit.Record(txCtx, dtos.AuditEntryInput{
				ActorType:   enums.ActorAdmin,
				ActorID:     &adminID,
				Action:      "payout.release_hold",
				ObjectType:  "payout",
				ObjectID:    up.ID,
				BeforeState: map[string]any{"status": beforeStatus},
				AfterState: map[string]any{
					"status":     up.Status,
					"release_at": up.ReleaseAt,
				},
				Reason: &reason,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return updated, nil
}

// ProcessPayoutRelease executes the release of a single scheduled payout inside a DB transaction,
// validates state machine transitions, issues double-entry ledger records, and produces audit logs.
// It is idempotent: if the payout is already released, cancelled, or held, it returns nil cleanly.
func (s *PayoutService) ProcessPayoutRelease(ctx context.Context, payoutID uuid.UUID) error {
	now := s.clock.Now()
	return s.db.WithinTx(ctx, func(txCtx context.Context) error {
		p, err := s.paymentRepo.FindPayoutByIDForUpdate(txCtx, payoutID)
		if err != nil {
			if errors.Is(err, apperrors.ErrNotFound) {
				return nil
			}
			return err
		}

		// Idempotency: if already released, simply acknowledge
		if p.Status == enums.PayoutStatusReleased {
			return nil
		}

		// If cancelled or held by an admin, do not release
		if p.Status == enums.PayoutStatusCancelled || p.Status == enums.PayoutStatusHeld {
			return nil
		}

		if err := statemachine.ValidatePayoutTransition(p.Status, enums.PayoutStatusReleased); err != nil {
			return nil
		}

		simRef := fmt.Sprintf("sim_po_%s", strings.ReplaceAll(p.ID.String()[:12], "-", ""))
		p.Status = enums.PayoutStatusReleased
		p.ReleasedAt = &now
		p.SimulatedRef = &simRef
		p.UpdatedAt = now

		updatedPayout, err := s.paymentRepo.UpdatePayout(txCtx, *p)
		if err != nil {
			return err
		}

		if s.ledger != nil {
			if err := s.ledger.RecordPayoutRelease(txCtx, uuid.New(), *updatedPayout, enums.ActorSystem, nil); err != nil {
				return err
			}
		}

		if s.cohostRepo != nil {
			_ = s.cohostRepo.UpdatePayoutSplitsStatus(txCtx, p.ID, enums.PayoutSplitReleased, now)
		}

		if s.audit != nil {
			_ = s.audit.Record(txCtx, dtos.AuditEntryInput{
				ActorType:  enums.ActorSystem,
				ActorID:    nil,
				Action:     "payout.release",
				ObjectType: "payout",
				ObjectID:   updatedPayout.ID,
				BeforeState: map[string]any{"status": enums.PayoutStatusScheduled},
				AfterState: map[string]any{
					"status":        updatedPayout.Status,
					"released_at":   updatedPayout.ReleasedAt,
					"simulated_ref": updatedPayout.SimulatedRef,
				},
			})
		}

		return nil
	})
}

func (s *PayoutService) ReleaseDuePayouts(ctx context.Context, limit int) (int, error) {
	now := s.clock.Now()
	dueList, err := s.paymentRepo.ListDuePayouts(ctx, now, limit)
	if err != nil {
		return 0, err
	}

	processed := 0
	for _, item := range dueList {
		err := s.ProcessPayoutRelease(ctx, item.ID)
		if err == nil {
			processed++
		}
	}
	return processed, nil
}

func (s *PayoutService) AdjustPayoutOnRefund(
	ctx context.Context,
	res models.Reservation,
	refundedHostNetMinor int64,
) error {
	p, err := s.paymentRepo.FindPayoutByReservation(ctx, res.ID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil
		}
		return err
	}

	now := s.clock.Now()
	newNet := p.NetMinor - refundedHostNetMinor
	if newNet < 0 {
		newNet = 0
	}
	p.NetMinor = newNet
	p.GrossMinor = newNet + p.HostFeeMinor
	p.UpdatedAt = now

	if newNet == 0 {
		p.Status = enums.PayoutStatusCancelled
		if s.cohostRepo != nil {
			_ = s.cohostRepo.UpdatePayoutSplitsStatus(ctx, p.ID, enums.PayoutSplitCancelled, now)
		}
	}

	_, err = s.paymentRepo.UpdatePayout(ctx, *p)
	return err
}
