package interfaces

import (
	"context"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/models"
)

type LedgerService interface {
	AssertBalanced(entries []models.LedgerEntry) error
	RecordCharge(
		ctx context.Context,
		txnID uuid.UUID,
		res models.Reservation,
		payment models.Payment,
		guestTotal, guestFee, hostFee, hostNet int64,
		actorType enums.ActorType,
		actorID *uuid.UUID,
	) error
	RecordRefund(
		ctx context.Context,
		txnID uuid.UUID,
		res models.Reservation,
		refundPayment models.Payment,
		rb models.CancellationRefundBreakdown,
		actorType enums.ActorType,
		actorID *uuid.UUID,
		reason string,
	) error
	RecordCustomRefund(
		ctx context.Context,
		txnID uuid.UUID,
		res models.Reservation,
		refundPayment models.Payment,
		amountMinor int64,
		actorType enums.ActorType,
		actorID *uuid.UUID,
		reason string,
	) error
	RecordPayoutRelease(
		ctx context.Context,
		txnID uuid.UUID,
		payout models.Payout,
		actorType enums.ActorType,
		actorID *uuid.UUID,
	) error
}
