package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/domain/statemachine"
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/db"
	"github.com/stayhub/backend/internal/platform/storage"
)

type AdminReservationService struct {
	db             *db.DB
	resRepo        interfaces.ReservationRepository
	paymentRepo    interfaces.PaymentRepository
	listingRepo    interfaces.ListingRepository
	calendarRepo   interfaces.CalendarRepository
	userRepo       interfaces.UserRepository
	mediaRepo      interfaces.MediaRepository
	catalogService interfaces.CatalogService
	ledger         interfaces.LedgerService
	payoutSvc      interfaces.PayoutService
	audit          interfaces.AuditService
	storage        *storage.Storage
	clock          clock.Clock
}

func NewAdminReservationService(
	database *db.DB,
	resRepo interfaces.ReservationRepository,
	paymentRepo interfaces.PaymentRepository,
	listingRepo interfaces.ListingRepository,
	calendarRepo interfaces.CalendarRepository,
	userRepo interfaces.UserRepository,
	mediaRepo interfaces.MediaRepository,
	catalogService interfaces.CatalogService,
	ledger interfaces.LedgerService,
	payoutSvc interfaces.PayoutService,
	audit interfaces.AuditService,
	store *storage.Storage,
	clk clock.Clock,
) interfaces.AdminReservationService {
	return &AdminReservationService{
		db:             database,
		resRepo:        resRepo,
		paymentRepo:    paymentRepo,
		listingRepo:    listingRepo,
		calendarRepo:   calendarRepo,
		userRepo:       userRepo,
		mediaRepo:      mediaRepo,
		catalogService: catalogService,
		ledger:         ledger,
		payoutSvc:      payoutSvc,
		audit:          audit,
		storage:        store,
		clock:          clk,
	}
}

func (s *AdminReservationService) ListReservations(
	ctx context.Context,
	filter dtos.AdminReservationFilter,
) (*dtos.AdminReservationListResponse, error) {
	rows, total, err := s.resRepo.ListAdmin(ctx, filter)
	if err != nil {
		return nil, err
	}

	summaries := make([]dtos.AdminReservationSummary, 0, len(rows))
	for _, r := range rows {
		listingSummary := s.buildListingSummary(ctx, r.ListingID, r.ListingVersionID)
		guestSummary := s.buildUserSummary(ctx, r.GuestID)
		hostSummary := s.buildUserSummary(ctx, r.HostID)

		summaries = append(summaries, dtos.AdminReservationSummary{
			Reservation: r,
			Listing:     listingSummary,
			Guest:       guestSummary,
			Host:        hostSummary,
		})
	}

	return &dtos.AdminReservationListResponse{
		Total:        total,
		Reservations: summaries,
	}, nil
}

func (s *AdminReservationService) GetReservationDetail(
	ctx context.Context,
	id uuid.UUID,
) (*dtos.AdminReservationDetailResponse, error) {
	r, err := s.resRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	listingSummary := s.buildListingSummary(ctx, r.ListingID, r.ListingVersionID)
	guestSummary := s.buildUserSummary(ctx, r.GuestID)
	hostSummary := s.buildUserSummary(ctx, r.HostID)

	payments, _ := s.paymentRepo.ListPaymentsByReservation(ctx, r.ID)
	if payments == nil {
		payments = []models.Payment{}
	}

	payout, _ := s.paymentRepo.FindPayoutByReservation(ctx, r.ID)
	ledger, _ := s.paymentRepo.ListLedgerByReservation(ctx, r.ID)
	if ledger == nil {
		ledger = []models.LedgerEntry{}
	}

	var timeline []models.EventLog
	if s.audit != nil {
		events, _ := s.audit.ListEvents(ctx, dtos.EventListFilter{
			ObjectType: "reservation",
			ObjectID:   &r.ID,
			Limit:      100,
			Offset:     0,
		})
		timeline = events
	}
	if timeline == nil {
		timeline = []models.EventLog{}
	}

	var refundedTotal int64
	for _, p := range payments {
		if p.Kind == enums.PaymentKindRefund && p.Status == enums.PaymentStatusSucceeded {
			refundedTotal += p.AmountMinor
		}
	}
	refundable := r.GuestTotalMinor - refundedTotal
	if refundable < 0 {
		refundable = 0
	}

	return &dtos.AdminReservationDetailResponse{
		Reservation:   *r,
		Listing:       listingSummary,
		Guest:         guestSummary,
		Host:          hostSummary,
		Payments:      payments,
		Payout:        payout,
		LedgerEntries: ledger,
		Timeline:      timeline,
		Refundable:    refundable,
	}, nil
}

func (s *AdminReservationService) CancelReservation(
	ctx context.Context,
	actor models.AuthActor,
	id uuid.UUID,
	in dtos.AdminCancelReservationInput,
) (*dtos.AdminReservationDetailResponse, error) {
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return nil, fmt.Errorf("%w: cancellation reason is required", apperrors.ErrValidation)
	}

	mode := strings.ToLower(strings.TrimSpace(in.RefundMode))
	if mode != "policy" && mode != "full" && mode != "custom" {
		return nil, fmt.Errorf("%w: refund_mode must be 'policy', 'full', or 'custom'", apperrors.ErrValidation)
	}
	if mode == "custom" && (in.AmountMinor == nil || *in.AmountMinor <= 0) {
		return nil, fmt.Errorf("%w: custom refund mode requires amount_minor > 0", apperrors.ErrValidation)
	}

	now := s.clock.Now()
	err := s.db.WithinTx(ctx, func(txCtx context.Context) error {
		r, err := s.resRepo.FindByIDForUpdate(txCtx, id)
		if err != nil {
			return err
		}

		if statemachine.IsTerminalReservationStatus(r.Status) {
			return fmt.Errorf("%w: cannot cancel reservation in terminal state %s", apperrors.ErrInvalidStateTransition, r.Status)
		}

		beforeStatus := r.Status
		targetStatus := enums.ReservationStatusCancelledByHost
		if err := statemachine.ValidateReservationTransition(r.Status, targetStatus); err != nil {
			// If already disputed or in-progress, force cancellation
			targetStatus = enums.ReservationStatusCancelledByHost
		}

		// Calculate refund breakdown
		var rb models.CancellationRefundBreakdown
		switch mode {
		case "full":
			rb = models.ComputeFullRefund(*r, now)
		case "custom":
			if *in.AmountMinor > r.GuestTotalMinor {
				return fmt.Errorf("%w: refund amount exceeds guest total", apperrors.ErrValidation)
			}
			rb = models.ComputeCustomRefund(*r, *in.AmountMinor)
		case "policy":
			var rules []models.CancellationPolicyRule
			if pol, err := s.catalogService.GetCancellationPolicy(txCtx, r.CancellationPolicy); err == nil && pol != nil {
				rules = pol.Rules
			}
			rb = models.ComputePolicyRefund(*r, rules, now)
		}

		r.Status = targetStatus
		r.CancelledAt = &now
		actorAdmin := enums.ActorAdmin
		r.CancelledBy = &actorAdmin
		r.CancelReason = &reason
		r.UpdatedAt = now

		updatedRes, err := s.resRepo.Update(txCtx, *r)
		if err != nil {
			return err
		}

		// Release calendar nights
		_ = s.calendarRepo.ReleaseReservationNights(txCtx, updatedRes.ID)

		// Record refund payment & ledger if any refund amount
		if rb.RefundAmountMinor > 0 {
			var chargePaymentID *uuid.UUID
			if chg, err := s.paymentRepo.FindChargeByReservation(txCtx, r.ID); err == nil && chg != nil {
				chargePaymentID = &chg.ID
			}

			basisBytes, _ := json.Marshal(rb)
			simRef := fmt.Sprintf("sim_ref_%s", strings.ReplaceAll(r.ID.String()[:12], "-", ""))
			refundPct := rb.RefundPct

			refundPayment, err := s.paymentRepo.CreatePayment(txCtx, models.Payment{
				ReservationID:    r.ID,
				Kind:             enums.PaymentKindRefund,
				Status:           enums.PaymentStatusSucceeded,
				AmountMinor:      rb.RefundAmountMinor,
				Currency:         r.Currency,
				IdempotencyKey:   fmt.Sprintf("admin_cancel_refund:%s:%d", r.ID.String(), now.UnixNano()),
				SimulatedRef:     &simRef,
				RelatedPaymentID: chargePaymentID,
				RefundPct:        &refundPct,
				RefundBasis:      json.RawMessage(basisBytes),
			})
			if err != nil {
				return err
			}

			if s.ledger != nil {
				if err := s.ledger.RecordRefund(txCtx, uuid.New(), *r, *refundPayment, rb, enums.ActorAdmin, &actor.SubjectID, reason); err != nil {
					return err
				}
			}

			if s.payoutSvc != nil {
				_ = s.payoutSvc.AdjustPayoutOnRefund(txCtx, *r, rb.RefundedHostNetMinor)
			}
		}

		if s.audit != nil {
			_ = s.audit.Record(txCtx, dtos.AuditEntryInput{
				ActorType:   enums.ActorAdmin,
				ActorID:     &actor.SubjectID,
				Action:      "admin.reservation.cancel",
				ObjectType:  "reservation",
				ObjectID:    updatedRes.ID,
				BeforeState: map[string]any{"status": beforeStatus},
				AfterState: map[string]any{
					"status":        updatedRes.Status,
					"refund_mode":   mode,
					"refund_amount": rb.RefundAmountMinor,
					"reason":        reason,
				},
				Reason: &reason,
			})
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.GetReservationDetail(ctx, id)
}

func (s *AdminReservationService) RefundReservation(
	ctx context.Context,
	actor models.AuthActor,
	id uuid.UUID,
	in dtos.AdminRefundReservationInput,
) (*dtos.AdminReservationDetailResponse, error) {
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return nil, fmt.Errorf("%w: refund reason is required", apperrors.ErrValidation)
	}
	if in.AmountMinor <= 0 {
		return nil, fmt.Errorf("%w: refund amount must be greater than zero", apperrors.ErrValidation)
	}

	now := s.clock.Now()
	err := s.db.WithinTx(ctx, func(txCtx context.Context) error {
		r, err := s.resRepo.FindByIDForUpdate(txCtx, id)
		if err != nil {
			return err
		}

		payments, err := s.paymentRepo.ListPaymentsByReservation(txCtx, r.ID)
		if err != nil {
			return err
		}

		var alreadyRefunded int64
		for _, p := range payments {
			if p.Kind == enums.PaymentKindRefund && p.Status == enums.PaymentStatusSucceeded {
				alreadyRefunded += p.AmountMinor
			}
		}

		remaining := r.GuestTotalMinor - alreadyRefunded
		if in.AmountMinor > remaining {
			return fmt.Errorf("%w: refund of %d exceeds remaining refundable amount %d", apperrors.ErrValidation, in.AmountMinor, remaining)
		}

		rb := models.ComputeCustomRefund(*r, in.AmountMinor)
		var chargePaymentID *uuid.UUID
		if chg, err := s.paymentRepo.FindChargeByReservation(txCtx, r.ID); err == nil && chg != nil {
			chargePaymentID = &chg.ID
		}

		basisBytes, _ := json.Marshal(rb)
		simRef := fmt.Sprintf("sim_ref_%s", strings.ReplaceAll(r.ID.String()[:12], "-", ""))
		refundPct := rb.RefundPct

		refundPayment, err := s.paymentRepo.CreatePayment(txCtx, models.Payment{
			ReservationID:    r.ID,
			Kind:             enums.PaymentKindRefund,
			Status:           enums.PaymentStatusSucceeded,
			AmountMinor:      in.AmountMinor,
			Currency:         r.Currency,
			IdempotencyKey:   fmt.Sprintf("admin_refund:%s:%d", r.ID.String(), now.UnixNano()),
			SimulatedRef:     &simRef,
			RelatedPaymentID: chargePaymentID,
			RefundPct:        &refundPct,
			RefundBasis:      json.RawMessage(basisBytes),
		})
		if err != nil {
			return err
		}

		if s.ledger != nil {
			if err := s.ledger.RecordRefund(txCtx, uuid.New(), *r, *refundPayment, rb, enums.ActorAdmin, &actor.SubjectID, reason); err != nil {
				return err
			}
		}

		if s.payoutSvc != nil {
			_ = s.payoutSvc.AdjustPayoutOnRefund(txCtx, *r, rb.RefundedHostNetMinor)
		}

		if s.audit != nil {
			_ = s.audit.Record(txCtx, dtos.AuditEntryInput{
				ActorType:  enums.ActorAdmin,
				ActorID:    &actor.SubjectID,
				Action:     "admin.reservation.refund",
				ObjectType: "reservation",
				ObjectID:   r.ID,
				AfterState: map[string]any{
					"refund_payment_id": refundPayment.ID,
					"amount_minor":      in.AmountMinor,
					"reason":            reason,
				},
				Reason: &reason,
			})
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.GetReservationDetail(ctx, id)
}

func (s *AdminReservationService) buildListingSummary(ctx context.Context, listingID, versionID uuid.UUID) dtos.ReservationListingSummary {
	sum := dtos.ReservationListingSummary{ID: listingID}
	if lv, err := s.listingRepo.FindVersionByID(ctx, versionID); err == nil && lv != nil {
		sum.Title = derefString(lv.Title)
		sum.PropertyType = derefString(lv.PropertyType)
		sum.City = derefString(lv.City)
		sum.Country = derefString(lv.Country)
		sum.AddressLine1 = derefString(lv.AddressLine1)
		if len(lv.MediaIDs) > 0 && s.mediaRepo != nil && s.storage != nil {
			if m, err := s.mediaRepo.FindByID(ctx, lv.MediaIDs[0]); err == nil {
				url := s.storage.PublicURL(m.S3Key)
				sum.CoverPhotoURL = &url
			}
		}
	}
	return sum
}

func (s *AdminReservationService) buildUserSummary(ctx context.Context, userID uuid.UUID) dtos.ReservationUserSummary {
	sum := dtos.ReservationUserSummary{ID: userID}
	if u, err := s.userRepo.FindByID(ctx, userID); err == nil && u != nil {
		sum.FullName = u.FullName
		sum.Email = u.Email
		if u.AvatarMediaID != nil && s.mediaRepo != nil && s.storage != nil {
			if m, err := s.mediaRepo.FindByID(ctx, *u.AvatarMediaID); err == nil {
				url := s.storage.PublicURL(m.S3Key)
				sum.AvatarURL = &url
			}
		}
	}
	return sum
}
