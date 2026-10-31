package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
)

type LedgerService struct {
	paymentRepo interfaces.PaymentRepository
	audit       interfaces.AuditService
}

func NewLedgerService(
	paymentRepo interfaces.PaymentRepository,
	audit interfaces.AuditService,
) interfaces.LedgerService {
	return &LedgerService{
		paymentRepo: paymentRepo,
		audit:       audit,
	}
}

// AssertBalanced asserts that the sum of debit entries equals the sum of credit entries.
func (s *LedgerService) AssertBalanced(entries []models.LedgerEntry) error {
	var debit, credit int64
	for _, e := range entries {
		switch e.Direction {
		case enums.LedgerDirectionDebit:
			debit += e.AmountMinor
		case enums.LedgerDirectionCredit:
			credit += e.AmountMinor
		default:
			return fmt.Errorf("unexpected ledger direction %q", e.Direction)
		}
	}
	if debit != credit {
		return fmt.Errorf("%w: unbalanced ledger entries (debit=%d, credit=%d)", apperrors.ErrValidation, debit, credit)
	}
	return nil
}

func (s *LedgerService) RecordCharge(
	ctx context.Context,
	txnID uuid.UUID,
	res models.Reservation,
	payment models.Payment,
	guestTotal, guestFee, hostFee, hostNet int64,
	actorType enums.ActorType,
	actorID *uuid.UUID,
) error {
	legs := BuildChargeLedgerEntries(
		txnID,
		res.ID,
		payment.ID,
		res.Currency,
		guestTotal,
		guestFee,
		hostFee,
		hostNet,
	)
	if err := s.AssertBalanced(legs); err != nil {
		return err
	}
	if err := s.paymentRepo.InsertLedgerEntries(ctx, legs); err != nil {
		return err
	}

	if s.audit != nil {
		_ = s.audit.Record(ctx, dtos.AuditEntryInput{
			ActorType:  actorType,
			ActorID:    actorID,
			Action:     "payment.charge",
			ObjectType: "payment",
			ObjectID:   payment.ID,
			AfterState: map[string]any{
				"reservation_id": payment.ReservationID,
				"amount_minor":   payment.AmountMinor,
				"status":         payment.Status,
				"currency":       payment.Currency,
				"simulated_ref":  payment.SimulatedRef,
			},
		})
	}
	return nil
}

func (s *LedgerService) RecordRefund(
	ctx context.Context,
	txnID uuid.UUID,
	res models.Reservation,
	refundPayment models.Payment,
	rb models.CancellationRefundBreakdown,
	actorType enums.ActorType,
	actorID *uuid.UUID,
	reason string,
) error {
	legs := BuildRefundLedgerEntries(
		txnID,
		res.ID,
		refundPayment.ID,
		res.Currency,
		rb,
	)
	if err := s.AssertBalanced(legs); err != nil {
		return err
	}
	if err := s.paymentRepo.InsertLedgerEntries(ctx, legs); err != nil {
		return err
	}

	if s.audit != nil {
		auditReason := reason
		_ = s.audit.Record(ctx, dtos.AuditEntryInput{
			ActorType:  actorType,
			ActorID:    actorID,
			Action:     "payment.refund",
			ObjectType: "payment",
			ObjectID:   refundPayment.ID,
			AfterState: map[string]any{
				"reservation_id":      res.ID,
				"amount_minor":        refundPayment.AmountMinor,
				"refund_pct":          refundPayment.RefundPct,
				"refunds_service_fee": rb.RefundsServiceFee,
				"simulated_ref":       refundPayment.SimulatedRef,
			},
			Reason: &auditReason,
		})
	}
	return nil
}

func (s *LedgerService) RecordCustomRefund(
	ctx context.Context,
	txnID uuid.UUID,
	res models.Reservation,
	refundPayment models.Payment,
	amountMinor int64,
	actorType enums.ActorType,
	actorID *uuid.UUID,
	reason string,
) error {
	rb := models.ComputeCustomRefund(res, amountMinor)
	return s.RecordRefund(ctx, txnID, res, refundPayment, rb, actorType, actorID, reason)
}

func (s *LedgerService) RecordPayoutRelease(
	ctx context.Context,
	txnID uuid.UUID,
	payout models.Payout,
	actorType enums.ActorType,
	actorID *uuid.UUID,
) error {
	legs := BuildPayoutReleaseLedgerEntries(
		txnID,
		payout.ReservationID,
		payout.ID,
		payout.Currency,
		payout.NetMinor,
	)
	if err := s.AssertBalanced(legs); err != nil {
		return err
	}
	if err := s.paymentRepo.InsertLedgerEntries(ctx, legs); err != nil {
		return err
	}

	if s.audit != nil {
		_ = s.audit.Record(ctx, dtos.AuditEntryInput{
			ActorType:   actorType,
			ActorID:     actorID,
			Action:      "payout.release",
			ObjectType:  "payout",
			ObjectID:    payout.ID,
			BeforeState: map[string]any{"status": enums.PayoutStatusScheduled},
			AfterState: map[string]any{
				"status":         payout.Status,
				"released_at":    payout.ReleasedAt,
				"net_minor":      payout.NetMinor,
				"simulated_ref":  payout.SimulatedRef,
				"reservation_id": payout.ReservationID,
			},
		})
	}
	return nil
}
