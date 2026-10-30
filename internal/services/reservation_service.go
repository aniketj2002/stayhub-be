package services

import (
	"context"
	"crypto/rand"
	"encoding/json"
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
	"github.com/stayhub/backend/internal/http/middleware"
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/db"
	"github.com/stayhub/backend/internal/platform/redis"
	"github.com/stayhub/backend/internal/platform/storage"
)

type ReservationService struct {
	cfg             *config.Config
	db              *db.DB
	rdb             *redis.Client
	resRepo         interfaces.ReservationRepository
	paymentRepo     interfaces.PaymentRepository
	listingRepo     interfaces.ListingRepository
	listingHostRepo interfaces.ListingHostRepository
	calendarRepo    interfaces.CalendarRepository
	userRepo        interfaces.UserRepository
	mediaRepo       interfaces.MediaRepository
	catalogService  interfaces.CatalogService
	storage         *storage.Storage
	audit           interfaces.AuditService
	clock           clock.Clock
}

func NewReservationService(
	cfg *config.Config,
	database *db.DB,
	rdb *redis.Client,
	resRepo interfaces.ReservationRepository,
	paymentRepo interfaces.PaymentRepository,
	listingRepo interfaces.ListingRepository,
	listingHostRepo interfaces.ListingHostRepository,
	calendarRepo interfaces.CalendarRepository,
	userRepo interfaces.UserRepository,
	mediaRepo interfaces.MediaRepository,
	catalogService interfaces.CatalogService,
	store *storage.Storage,
	audit interfaces.AuditService,
	clk clock.Clock,
) interfaces.ReservationService {
	return &ReservationService{
		cfg:             cfg,
		db:              database,
		rdb:             rdb,
		resRepo:         resRepo,
		paymentRepo:     paymentRepo,
		listingRepo:     listingRepo,
		listingHostRepo: listingHostRepo,
		calendarRepo:    calendarRepo,
		userRepo:        userRepo,
		mediaRepo:       mediaRepo,
		catalogService:  catalogService,
		storage:         store,
		audit:           audit,
		clock:           clk,
	}
}

// BuildChargeLedgerEntries constructs a balanced 4-leg double-entry ledger transaction for a guest payment charge.
func BuildChargeLedgerEntries(
	txnID, reservationID, paymentID uuid.UUID,
	currency string,
	guestTotalMinor, guestFeeMinor, hostFeeMinor, hostNetMinor int64,
) []models.LedgerEntry {
	var entries []models.LedgerEntry
	resID := reservationID
	payID := paymentID

	if guestTotalMinor > 0 {
		entries = append(entries, models.LedgerEntry{
			TxnID:         txnID,
			ReservationID: &resID,
			PaymentID:     &payID,
			Account:       enums.LedgerAccountGuestCash,
			Direction:     enums.LedgerDirectionDebit,
			AmountMinor:   guestTotalMinor,
			Currency:      currency,
		})
	}
	if guestFeeMinor > 0 {
		entries = append(entries, models.LedgerEntry{
			TxnID:         txnID,
			ReservationID: &resID,
			PaymentID:     &payID,
			Account:       enums.LedgerAccountPlatformGuestFee,
			Direction:     enums.LedgerDirectionCredit,
			AmountMinor:   guestFeeMinor,
			Currency:      currency,
		})
	}
	if hostFeeMinor > 0 {
		entries = append(entries, models.LedgerEntry{
			TxnID:         txnID,
			ReservationID: &resID,
			PaymentID:     &payID,
			Account:       enums.LedgerAccountPlatformHostFee,
			Direction:     enums.LedgerDirectionCredit,
			AmountMinor:   hostFeeMinor,
			Currency:      currency,
		})
	}
	if hostNetMinor > 0 {
		entries = append(entries, models.LedgerEntry{
			TxnID:         txnID,
			ReservationID: &resID,
			PaymentID:     &payID,
			Account:       enums.LedgerAccountHostPayable,
			Direction:     enums.LedgerDirectionCredit,
			AmountMinor:   hostNetMinor,
			Currency:      currency,
		})
	}
	return entries
}

// BuildRefundLedgerEntries constructs a balanced double-entry ledger transaction for a full or partial refund.
func BuildRefundLedgerEntries(
	txnID, reservationID, refundPaymentID uuid.UUID,
	currency string,
	rb models.CancellationRefundBreakdown,
) []models.LedgerEntry {
	if rb.RefundAmountMinor <= 0 {
		return nil
	}
	var entries []models.LedgerEntry
	resID := reservationID
	payID := refundPaymentID

	if rb.RefundedGuestFeeMinor > 0 {
		entries = append(entries, models.LedgerEntry{
			TxnID:         txnID,
			ReservationID: &resID,
			PaymentID:     &payID,
			Account:       enums.LedgerAccountPlatformGuestFee,
			Direction:     enums.LedgerDirectionDebit,
			AmountMinor:   rb.RefundedGuestFeeMinor,
			Currency:      currency,
		})
	}
	if rb.RefundedHostFeeMinor > 0 {
		entries = append(entries, models.LedgerEntry{
			TxnID:         txnID,
			ReservationID: &resID,
			PaymentID:     &payID,
			Account:       enums.LedgerAccountPlatformHostFee,
			Direction:     enums.LedgerDirectionDebit,
			AmountMinor:   rb.RefundedHostFeeMinor,
			Currency:      currency,
		})
	}
	if rb.RefundedHostNetMinor > 0 {
		entries = append(entries, models.LedgerEntry{
			TxnID:         txnID,
			ReservationID: &resID,
			PaymentID:     &payID,
			Account:       enums.LedgerAccountHostPayable,
			Direction:     enums.LedgerDirectionDebit,
			AmountMinor:   rb.RefundedHostNetMinor,
			Currency:      currency,
		})
	}
	entries = append(entries, models.LedgerEntry{
		TxnID:         txnID,
		ReservationID: &resID,
		PaymentID:     &payID,
		Account:       enums.LedgerAccountRefunds,
		Direction:     enums.LedgerDirectionCredit,
		AmountMinor:   rb.RefundAmountMinor,
		Currency:      currency,
	})
	return entries
}

// BuildPayoutReleaseLedgerEntries constructs a balanced 2-leg ledger transaction when a host payout is released.
func BuildPayoutReleaseLedgerEntries(
	txnID, reservationID, payoutID uuid.UUID,
	currency string,
	netMinor int64,
) []models.LedgerEntry {
	if netMinor <= 0 {
		return nil
	}
	resID := reservationID
	poID := payoutID
	return []models.LedgerEntry{
		{
			TxnID:         txnID,
			ReservationID: &resID,
			PayoutID:      &poID,
			Account:       enums.LedgerAccountHostPayable,
			Direction:     enums.LedgerDirectionDebit,
			AmountMinor:   netMinor,
			Currency:      currency,
		},
		{
			TxnID:         txnID,
			ReservationID: &resID,
			PayoutID:      &poID,
			Account:       enums.LedgerAccountHostPaid,
			Direction:     enums.LedgerDirectionCredit,
			AmountMinor:   netMinor,
			Currency:      currency,
		},
	}
}

func (s *ReservationService) CreateReservation(
	ctx context.Context,
	actor models.AuthActor,
	idemKey string,
	in dtos.CreateReservationInput,
) (*dtos.ReservationDetailResponse, error) {
	l, err := s.listingRepo.FindByID(ctx, in.ListingID)
	if err != nil {
		return nil, err
	}
	if l.Status != enums.ListingStatusLive || l.LiveVersionID == nil {
		return nil, apperrors.ErrListingNotLive
	}
	if l.HostID == actor.SubjectID {
		return nil, apperrors.ErrSelfBookingNotAllowed
	}
	if _, err := s.listingHostRepo.FindByListingAndUser(ctx, l.ID, actor.SubjectID); err == nil {
		return nil, apperrors.ErrSelfBookingNotAllowed
	}

	lv, err := s.listingRepo.FindVersionByID(ctx, *l.LiveVersionID)
	if err != nil {
		return nil, err
	}

	maxGuests := 1
	if lv.MaxGuests != nil {
		maxGuests = *lv.MaxGuests
	}
	if in.Guests > maxGuests {
		return nil, apperrors.ErrGuestLimitExceeded
	}

	nights := models.EnumerateNights(in.CheckIn, in.CheckOut)
	if len(nights) == 0 {
		return nil, apperrors.ErrValidation
	}

	basePrice := int64(0)
	if lv.BasePriceMinor != nil {
		basePrice = *lv.BasePriceMinor
	}
	quote := models.ComputeStayQuote(basePrice, lv.CleaningFeeMinor, len(nights), s.cfg.GuestFeeBPS, s.cfg.HostFeeBPS)

	checkInAt, err := models.CombineLocalDateAndTime(in.CheckIn, lv.CheckInTime, lv.Timezone)
	if err != nil {
		checkInAt, _ = models.CombineLocalDateAndTime(in.CheckIn, "15:00", "UTC")
	}
	checkOutAt, err := models.CombineLocalDateAndTime(in.CheckOut, lv.CheckOutTime, lv.Timezone)
	if err != nil {
		checkOutAt, _ = models.CombineLocalDateAndTime(in.CheckOut, "11:00", "UTC")
	}

	policyCode := "flexible"
	if lv.CancellationPolicy != nil && strings.TrimSpace(*lv.CancellationPolicy) != "" {
		policyCode = strings.ToLower(strings.TrimSpace(*lv.CancellationPolicy))
	}

	now := s.clock.Now()

	// Prevent booking for past dates
	loc := time.UTC
	if lv.Timezone != "" {
		if l, err := time.LoadLocation(lv.Timezone); err == nil {
			loc = l
		}
	}
	nowInLoc := now.In(loc)
	todayInLoc := time.Date(nowInLoc.Year(), nowInLoc.Month(), nowInLoc.Day(), 0, 0, 0, 0, time.UTC)
	if in.CheckIn.Before(todayInLoc) || checkInAt.Before(now) {
		return nil, apperrors.ErrPastDate
	}

	resID := uuid.New()
	resCode := generateReservationCode()

	initialStatus := enums.ReservationStatusPending
	var requestExpiresAt *time.Time
	var decidedAt *time.Time
	if lv.InstantBook {
		initialStatus = enums.ReservationStatusConfirmed
		decidedAt = &now
	} else {
		exp := now.Add(time.Duration(s.cfg.RequestExpiryHours) * time.Hour)
		requestExpiresAt = &exp
	}

	var createdRes *models.Reservation
	err = s.db.WithinTx(ctx, func(txCtx context.Context) error {
		resRow, err := s.resRepo.Create(txCtx, models.Reservation{
			ID:                 resID,
			Code:               resCode,
			ListingID:          l.ID,
			ListingVersionID:   lv.ID,
			GuestID:            actor.SubjectID,
			HostID:             l.HostID,
			Status:             initialStatus,
			IsInstantBook:      lv.InstantBook,
			CheckIn:            in.CheckIn,
			CheckOut:           in.CheckOut,
			CheckInTime:        lv.CheckInTime,
			CheckOutTime:       lv.CheckOutTime,
			Timezone:           lv.Timezone,
			CheckInAt:          checkInAt,
			CheckOutAt:         checkOutAt,
			Guests:             in.Guests,
			GuestMessage:       in.GuestMessage,
			Currency:           lv.Currency,
			NightlyTotalMinor:  quote.NightlyTotalMinor,
			CleaningFeeMinor:   quote.CleaningFeeMinor,
			GuestFeeBPS:        quote.GuestFeeBPS,
			GuestFeeMinor:      quote.GuestFeeMinor,
			GuestTotalMinor:    quote.GuestTotalMinor,
			HostFeeBPS:         quote.HostFeeBPS,
			HostFeeMinor:       quote.HostFeeMinor,
			HostNetMinor:       quote.HostNetMinor,
			CancellationPolicy: policyCode,
			RequestExpiresAt:   requestExpiresAt,
			DecidedAt:          decidedAt,
		})
		if err != nil {
			return err
		}
		createdRes = resRow

		// Atomically lock calendar nights (rejects double-booking or booking over host block)
		if err := s.calendarRepo.BookNights(txCtx, l.ID, resRow.ID, nights); err != nil {
			return err
		}

		// Record simulated guest charge
		simChargeRef := fmt.Sprintf("sim_chg_%s", strings.ReplaceAll(resRow.ID.String()[:12], "-", ""))
		chargePayment, err := s.paymentRepo.CreatePayment(txCtx, models.Payment{
			ReservationID:  resRow.ID,
			Kind:           enums.PaymentKindCharge,
			Status:         enums.PaymentStatusSucceeded,
			AmountMinor:    resRow.GuestTotalMinor,
			Currency:       resRow.Currency,
			IdempotencyKey: fmt.Sprintf("charge:%s:%s", resRow.ID.String(), idemKey),
			SimulatedRef:   &simChargeRef,
		})
		if err != nil {
			return err
		}

		// Record balanced double-entry ledger entries
		ledgerLegs := BuildChargeLedgerEntries(
			uuid.New(),
			resRow.ID,
			chargePayment.ID,
			resRow.Currency,
			resRow.GuestTotalMinor,
			resRow.GuestFeeMinor,
			resRow.HostFeeMinor,
			resRow.HostNetMinor,
		)
		if err := s.paymentRepo.InsertLedgerEntries(txCtx, ledgerLegs); err != nil {
			return err
		}

		guestActorID := actor.SubjectID
		if err := s.audit.Record(txCtx, dtos.AuditEntryInput{
			ActorType:  enums.ActorGuest,
			ActorID:    &guestActorID,
			Action:     "reservation.create",
			ObjectType: "reservation",
			ObjectID:   resRow.ID,
			AfterState: map[string]any{
				"code":               resRow.Code,
				"status":             resRow.Status,
				"is_instant_book":    resRow.IsInstantBook,
				"check_in":           in.CheckIn.Format("2006-01-02"),
				"check_out":          in.CheckOut.Format("2006-01-02"),
				"guest_total_minor":  resRow.GuestTotalMinor,
				"host_net_minor":     resRow.HostNetMinor,
				"request_expires_at": resRow.RequestExpiresAt,
			},
		}); err != nil {
			return err
		}

		if err := s.audit.Record(txCtx, dtos.AuditEntryInput{
			ActorType:  enums.ActorGuest,
			ActorID:    &guestActorID,
			Action:     "payment.charge",
			ObjectType: "payment",
			ObjectID:   chargePayment.ID,
			AfterState: map[string]any{
				"reservation_id": resRow.ID,
				"amount_minor":   chargePayment.AmountMinor,
				"currency":       chargePayment.Currency,
				"status":         chargePayment.Status,
				"simulated_ref":  chargePayment.SimulatedRef,
			},
		}); err != nil {
			return err
		}

		// If Instant Book, immediately schedule the host payout row
		if resRow.IsInstantBook {
			payout, err := s.paymentRepo.CreatePayout(txCtx, models.Payout{
				ReservationID:  resRow.ID,
				HostID:         resRow.HostID,
				Status:         enums.PayoutStatusScheduled,
				GrossMinor:     resRow.NightlyTotalMinor + resRow.CleaningFeeMinor,
				HostFeeMinor:   resRow.HostFeeMinor,
				NetMinor:       resRow.HostNetMinor,
				Currency:       resRow.Currency,
				IdempotencyKey: fmt.Sprintf("payout:schedule:%s", resRow.ID.String()),
			})
			if err != nil {
				return err
			}
			if err := s.audit.Record(txCtx, dtos.AuditEntryInput{
				ActorType:  enums.ActorSystem,
				ActorID:    nil,
				Action:     "payout.schedule",
				ObjectType: "payout",
				ObjectID:   payout.ID,
				AfterState: map[string]any{
					"reservation_id": resRow.ID,
					"status":         payout.Status,
					"gross_minor":    payout.GrossMinor,
					"host_fee_minor": payout.HostFeeMinor,
					"net_minor":      payout.NetMinor,
				},
			}); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Best-effort schedule Redis timers (Postgres reconciliation also backs these up)
	if s.rdb != nil && createdRes != nil {
		if createdRes.Status == enums.ReservationStatusPending && createdRes.RequestExpiresAt != nil {
			_ = s.rdb.ScheduleTimer(ctx, fmt.Sprintf("expire_pending:%s", createdRes.ID.String()), *createdRes.RequestExpiresAt)
		} else if createdRes.Status == enums.ReservationStatusConfirmed {
			_ = s.rdb.ScheduleTimer(ctx, fmt.Sprintf("auto_checkin:%s", createdRes.ID.String()), createdRes.CheckInAt)
			_ = s.rdb.ScheduleTimer(ctx, fmt.Sprintf("auto_checkout:%s", createdRes.ID.String()), createdRes.CheckOutAt)
		}
	}

	return s.GetReservationDetail(ctx, actor, createdRes.ID)
}

func (s *ReservationService) ListGuestTrips(
	ctx context.Context,
	guestID uuid.UUID,
	tab string,
) ([]dtos.ReservationDetailResponse, error) {
	rows, err := s.resRepo.ListByGuest(ctx, guestID)
	if err != nil {
		return nil, err
	}

	now := s.clock.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	cleanTab := strings.ToLower(strings.TrimSpace(tab))

	out := make([]dtos.ReservationDetailResponse, 0, len(rows))
	for _, r := range rows {
		switch cleanTab {
		case "upcoming":
			if (r.Status != enums.ReservationStatusPending && r.Status != enums.ReservationStatusConfirmed && r.Status != enums.ReservationStatusCheckedIn) || r.CheckOut.Before(today) {
				continue
			}
		case "past":
			if r.Status != enums.ReservationStatusCompleted && r.Status != enums.ReservationStatusReviewed && !r.CheckOut.Before(today) {
				continue
			}
		case "cancelled":
			if r.Status != enums.ReservationStatusCancelledByGuest && r.Status != enums.ReservationStatusCancelledByHost && r.Status != enums.ReservationStatusDeclined {
				continue
			}
		}
		detail, err := s.buildReservationDetail(ctx, r, false)
		if err != nil {
			continue
		}
		out = append(out, *detail)
	}
	return out, nil
}

func (s *ReservationService) GetReservationDetail(
	ctx context.Context,
	actor models.AuthActor,
	reservationID uuid.UUID,
) (*dtos.ReservationDetailResponse, error) {
	r, err := s.resRepo.FindByID(ctx, reservationID)
	if err != nil {
		return nil, err
	}

	isGuest := actor.SubjectType == "user" && r.GuestID == actor.SubjectID
	isAdmin := actor.SubjectType == "admin"
	isHostMember := false
	if actor.SubjectType == "user" && !isGuest {
		if _, err := s.listingHostRepo.FindByListingAndUser(ctx, r.ListingID, actor.SubjectID); err == nil {
			isHostMember = true
		} else if r.HostID == actor.SubjectID {
			isHostMember = true
		}
	}

	if !isGuest && !isHostMember && !isAdmin {
		return nil, apperrors.ErrForbidden
	}

	return s.buildReservationDetail(ctx, *r, isHostMember || isAdmin)
}

func (s *ReservationService) GetCancelPreview(
	ctx context.Context,
	guestID, reservationID uuid.UUID,
) (*dtos.CancelPreviewResponse, error) {
	r, err := s.resRepo.FindByID(ctx, reservationID)
	if err != nil {
		return nil, err
	}
	if r.GuestID != guestID {
		return nil, apperrors.ErrForbidden
	}
	if err := statemachine.ValidateReservationTransition(r.Status, enums.ReservationStatusCancelledByGuest); err != nil {
		return nil, err
	}

	now := s.clock.Now()
	var breakdown models.CancellationRefundBreakdown
	if r.Status == enums.ReservationStatusPending {
		breakdown = models.ComputeFullRefund(*r, now)
	} else {
		var rules []models.CancellationPolicyRule
		if pol, err := s.catalogService.GetCancellationPolicy(ctx, r.CancellationPolicy); err == nil && pol != nil {
			rules = pol.Rules
		}
		breakdown = models.ComputePolicyRefund(*r, rules, now)
	}

	return &dtos.CancelPreviewResponse{
		ReservationID:  r.ID,
		Code:           r.Code,
		Status:         r.Status,
		EvaluatedAt:    now,
		CheckInAt:      r.CheckInAt,
		Currency:       r.Currency,
		GuestPaidMinor: r.GuestTotalMinor,
		Breakdown:      breakdown,
	}, nil
}

func (s *ReservationService) CancelByGuest(
	ctx context.Context,
	guestID, reservationID uuid.UUID,
	idemKey string,
	in dtos.CancelReservationInput,
) (*dtos.ReservationDetailResponse, error) {
	now := s.clock.Now()

	err := s.db.WithinTx(ctx, func(txCtx context.Context) error {
		r, err := s.resRepo.FindByIDForUpdate(txCtx, reservationID)
		if err != nil {
			return err
		}
		if r.GuestID != guestID {
			return apperrors.ErrForbidden
		}
		if err := statemachine.ValidateReservationTransition(r.Status, enums.ReservationStatusCancelledByGuest); err != nil {
			return err
		}

		prevStatus := r.Status
		var rb models.CancellationRefundBreakdown
		if r.Status == enums.ReservationStatusPending {
			rb = models.ComputeFullRefund(*r, now)
		} else {
			var rules []models.CancellationPolicyRule
			if pol, err := s.catalogService.GetCancellationPolicy(txCtx, r.CancellationPolicy); err == nil && pol != nil {
				rules = pol.Rules
			}
			rb = models.ComputePolicyRefund(*r, rules, now)
		}

		actorGuest := enums.ActorGuest
		r.Status = enums.ReservationStatusCancelledByGuest
		r.CancelledAt = &now
		r.CancelledBy = &actorGuest
		r.CancelReason = in.CancelReason
		r.UpdatedAt = now

		updatedRes, err := s.resRepo.Update(txCtx, *r)
		if err != nil {
			return err
		}

		if err := s.calendarRepo.ReleaseReservationNights(txCtx, updatedRes.ID); err != nil {
			return err
		}

		if err := s.audit.Record(txCtx, dtos.AuditEntryInput{
			ActorType:   enums.ActorGuest,
			ActorID:     &guestID,
			Action:      "reservation.cancel_by_guest",
			ObjectType:  "reservation",
			ObjectID:    updatedRes.ID,
			BeforeState: map[string]any{"status": prevStatus},
			AfterState: map[string]any{
				"status":              updatedRes.Status,
				"refund_pct":          rb.RefundPct,
				"refund_amount_minor": rb.RefundAmountMinor,
				"retained_net_minor":  rb.RetainedHostNetMinor,
			},
			Reason: in.CancelReason,
		}); err != nil {
			return err
		}

		if rb.RefundAmountMinor > 0 {
			if err := s.issueRefundInTx(txCtx, *updatedRes, rb, enums.ActorGuest, &guestID, idemKey); err != nil {
				return err
			}
		}

		// Update or cancel scheduled host payout if reservation was confirmed
		if payout, err := s.paymentRepo.FindPayoutByReservation(txCtx, updatedRes.ID); err == nil && payout != nil {
			prevPayoutState := map[string]any{
				"status":         payout.Status,
				"gross_minor":    payout.GrossMinor,
				"host_fee_minor": payout.HostFeeMinor,
				"net_minor":      payout.NetMinor,
			}
			if rb.RetainedHostNetMinor <= 0 {
				if err := statemachine.ValidatePayoutTransition(payout.Status, enums.PayoutStatusCancelled); err != nil {
					return err
				}
				payout.Status = enums.PayoutStatusCancelled
				payout.UpdatedAt = now
				updatedPayout, err := s.paymentRepo.UpdatePayout(txCtx, *payout)
				if err != nil {
					return err
				}
				if err := s.audit.Record(txCtx, dtos.AuditEntryInput{
					ActorType:   enums.ActorSystem,
					ActorID:     nil,
					Action:      "payout.cancel",
					ObjectType:  "payout",
					ObjectID:    updatedPayout.ID,
					BeforeState: prevPayoutState,
					AfterState:  map[string]any{"status": updatedPayout.Status},
				}); err != nil {
					return err
				}
			} else {
				if err := statemachine.ValidatePayoutTransition(payout.Status, enums.PayoutStatusScheduled); err != nil {
					return err
				}
				payout.GrossMinor = rb.RetainedGrossMinor
				payout.HostFeeMinor = rb.RetainedHostFeeMinor
				payout.NetMinor = rb.RetainedHostNetMinor
				releaseAt := now.Add(time.Duration(s.cfg.PayoutReleaseAfterCheckInHours) * time.Hour)
				payout.ReleaseAt = &releaseAt
				payout.UpdatedAt = now
				updatedPayout, err := s.paymentRepo.UpdatePayout(txCtx, *payout)
				if err != nil {
					return err
				}
				if err := s.audit.Record(txCtx, dtos.AuditEntryInput{
					ActorType:   enums.ActorSystem,
					ActorID:     nil,
					Action:      "payout.adjust_partial_refund",
					ObjectType:  "payout",
					ObjectID:    updatedPayout.ID,
					BeforeState: prevPayoutState,
					AfterState: map[string]any{
						"status":         updatedPayout.Status,
						"gross_minor":    updatedPayout.GrossMinor,
						"host_fee_minor": updatedPayout.HostFeeMinor,
						"net_minor":      updatedPayout.NetMinor,
						"release_at":     updatedPayout.ReleaseAt,
					},
				}); err != nil {
					return err
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if s.rdb != nil {
		_ = s.rdb.RemoveTimer(ctx, fmt.Sprintf("expire_pending:%s", reservationID.String()))
	}

	return s.GetReservationDetail(ctx, models.AuthActor{
		SubjectID:   guestID,
		SubjectType: "user",
		IsGuest:     true,
	}, reservationID)
}

func (s *ReservationService) GetHostToday(ctx context.Context, hostUserID uuid.UUID) (*dtos.HostTodayResponse, error) {
	hostRecords, err := s.listingHostRepo.ListByUser(ctx, hostUserID)
	if err != nil {
		return nil, err
	}

	listingIDs := make([]uuid.UUID, 0, len(hostRecords))
	for _, lh := range hostRecords {
		listingIDs = append(listingIDs, lh.ListingID)
	}

	resp := &dtos.HostTodayResponse{
		PendingRequests:  []dtos.ReservationDetailResponse{},
		ArrivingToday:    []dtos.ReservationDetailResponse{},
		CurrentlyHosting: []dtos.ReservationDetailResponse{},
		CheckingOutToday: []dtos.ReservationDetailResponse{},
		Upcoming:         []dtos.ReservationDetailResponse{},
	}
	if len(listingIDs) == 0 {
		return resp, nil
	}

	reservations, err := s.resRepo.ListByListingIDs(ctx, listingIDs)
	if err != nil {
		return nil, err
	}

	now := s.clock.Now().UTC()
	todayStr := now.Format("2006-01-02")

	for _, r := range reservations {
		detail, err := s.buildReservationDetail(ctx, r, true)
		if err != nil {
			continue
		}
		checkInStr := r.CheckIn.Format("2006-01-02")
		checkOutStr := r.CheckOut.Format("2006-01-02")

		switch r.Status {
		case enums.ReservationStatusPending:
			resp.PendingRequests = append(resp.PendingRequests, *detail)
		case enums.ReservationStatusConfirmed:
			if checkInStr == todayStr {
				resp.ArrivingToday = append(resp.ArrivingToday, *detail)
			} else if checkOutStr == todayStr {
				resp.CheckingOutToday = append(resp.CheckingOutToday, *detail)
			} else if checkInStr < todayStr && checkOutStr > todayStr {
				resp.CurrentlyHosting = append(resp.CurrentlyHosting, *detail)
			} else if checkInStr > todayStr {
				resp.Upcoming = append(resp.Upcoming, *detail)
			}
		case enums.ReservationStatusCheckedIn:
			if checkOutStr == todayStr {
				resp.CheckingOutToday = append(resp.CheckingOutToday, *detail)
			} else {
				resp.CurrentlyHosting = append(resp.CurrentlyHosting, *detail)
			}
		}
	}

	return resp, nil
}

func (s *ReservationService) AcceptByHost(
	ctx context.Context,
	hostUserID, reservationID uuid.UUID,
	idemKey string,
) (*dtos.ReservationDetailResponse, error) {
	now := s.clock.Now()
	var updatedRes *models.Reservation

	err := s.db.WithinTx(ctx, func(txCtx context.Context) error {
		r, err := s.resRepo.FindByIDForUpdate(txCtx, reservationID)
		if err != nil {
			return err
		}

		lh, err := s.authorizeHostOnReservation(txCtx, r.ListingID, hostUserID, middleware.PermManageReservations)
		if err != nil {
			return err
		}

		if err := statemachine.ValidateReservationTransition(r.Status, enums.ReservationStatusConfirmed); err != nil {
			return err
		}
		if r.RequestExpiresAt != nil && !r.RequestExpiresAt.After(now) {
			return apperrors.ErrRequestExpired
		}

		r.Status = enums.ReservationStatusConfirmed
		r.DecidedAt = &now
		r.UpdatedAt = now

		resRow, err := s.resRepo.Update(txCtx, *r)
		if err != nil {
			return err
		}
		updatedRes = resRow

		payout, err := s.paymentRepo.CreatePayout(txCtx, models.Payout{
			ReservationID:  resRow.ID,
			HostID:         resRow.HostID,
			Status:         enums.PayoutStatusScheduled,
			GrossMinor:     resRow.NightlyTotalMinor + resRow.CleaningFeeMinor,
			HostFeeMinor:   resRow.HostFeeMinor,
			NetMinor:       resRow.HostNetMinor,
			Currency:       resRow.Currency,
			IdempotencyKey: fmt.Sprintf("payout:accept:%s:%s", resRow.ID.String(), idemKey),
		})
		if err != nil {
			return err
		}

		if err := s.audit.Record(txCtx, dtos.AuditEntryInput{
			ActorType:   lh.ActorType(),
			ActorID:     &hostUserID,
			Action:      "reservation.accept",
			ObjectType:  "reservation",
			ObjectID:    resRow.ID,
			BeforeState: map[string]any{"status": enums.ReservationStatusPending},
			AfterState:  map[string]any{"status": resRow.Status, "decided_at": resRow.DecidedAt},
		}); err != nil {
			return err
		}

		return s.audit.Record(txCtx, dtos.AuditEntryInput{
			ActorType:  enums.ActorSystem,
			ActorID:    nil,
			Action:     "payout.schedule",
			ObjectType: "payout",
			ObjectID:   payout.ID,
			AfterState: map[string]any{
				"reservation_id": resRow.ID,
				"status":         payout.Status,
				"gross_minor":    payout.GrossMinor,
				"host_fee_minor": payout.HostFeeMinor,
				"net_minor":      payout.NetMinor,
			},
		})
	})
	if err != nil {
		return nil, err
	}

	if s.rdb != nil && updatedRes != nil {
		_ = s.rdb.RemoveTimer(ctx, fmt.Sprintf("expire_pending:%s", updatedRes.ID.String()))
		_ = s.rdb.ScheduleTimer(ctx, fmt.Sprintf("auto_checkin:%s", updatedRes.ID.String()), updatedRes.CheckInAt)
		_ = s.rdb.ScheduleTimer(ctx, fmt.Sprintf("auto_checkout:%s", updatedRes.ID.String()), updatedRes.CheckOutAt)
	}

	return s.GetReservationDetail(ctx, models.AuthActor{
		SubjectID:   hostUserID,
		SubjectType: "user",
		IsHost:      true,
	}, reservationID)
}

func (s *ReservationService) DeclineByHost(
	ctx context.Context,
	hostUserID, reservationID uuid.UUID,
	idemKey string,
	in dtos.DeclineReservationInput,
) (*dtos.ReservationDetailResponse, error) {
	now := s.clock.Now()
	reason := strings.TrimSpace(in.DeclineReason)

	err := s.db.WithinTx(ctx, func(txCtx context.Context) error {
		r, err := s.resRepo.FindByIDForUpdate(txCtx, reservationID)
		if err != nil {
			return err
		}

		lh, err := s.authorizeHostOnReservation(txCtx, r.ListingID, hostUserID, middleware.PermManageReservations)
		if err != nil {
			return err
		}

		if err := statemachine.ValidateReservationTransition(r.Status, enums.ReservationStatusDeclined); err != nil {
			return err
		}

		r.Status = enums.ReservationStatusDeclined
		r.DecidedAt = &now
		r.DeclineReason = &reason
		r.UpdatedAt = now

		updatedRes, err := s.resRepo.Update(txCtx, *r)
		if err != nil {
			return err
		}

		if err := s.calendarRepo.ReleaseReservationNights(txCtx, updatedRes.ID); err != nil {
			return err
		}

		rb := models.ComputeFullRefund(*updatedRes, now)
		if err := s.issueRefundInTx(txCtx, *updatedRes, rb, lh.ActorType(), &hostUserID, idemKey); err != nil {
			return err
		}

		return s.audit.Record(txCtx, dtos.AuditEntryInput{
			ActorType:   lh.ActorType(),
			ActorID:     &hostUserID,
			Action:      "reservation.decline",
			ObjectType:  "reservation",
			ObjectID:    updatedRes.ID,
			BeforeState: map[string]any{"status": enums.ReservationStatusPending},
			AfterState: map[string]any{
				"status":              updatedRes.Status,
				"decline_reason":      updatedRes.DeclineReason,
				"refund_amount_minor": rb.RefundAmountMinor,
			},
			Reason: &reason,
		})
	})
	if err != nil {
		return nil, err
	}

	if s.rdb != nil {
		_ = s.rdb.RemoveTimer(ctx, fmt.Sprintf("expire_pending:%s", reservationID.String()))
	}

	return s.GetReservationDetail(ctx, models.AuthActor{
		SubjectID:   hostUserID,
		SubjectType: "user",
		IsHost:      true,
	}, reservationID)
}

func (s *ReservationService) CancelByHost(
	ctx context.Context,
	hostUserID, reservationID uuid.UUID,
	idemKey string,
	in dtos.CancelReservationInput,
) (*dtos.ReservationDetailResponse, error) {
	now := s.clock.Now()

	err := s.db.WithinTx(ctx, func(txCtx context.Context) error {
		r, err := s.resRepo.FindByIDForUpdate(txCtx, reservationID)
		if err != nil {
			return err
		}

		lh, err := s.authorizeHostOnReservation(txCtx, r.ListingID, hostUserID, middleware.PermManageReservations)
		if err != nil {
			return err
		}

		if err := statemachine.ValidateReservationTransition(r.Status, enums.ReservationStatusCancelledByHost); err != nil {
			return err
		}

		actorType := lh.ActorType()
		r.Status = enums.ReservationStatusCancelledByHost
		r.CancelledAt = &now
		r.CancelledBy = &actorType
		r.CancelReason = in.CancelReason
		r.UpdatedAt = now

		updatedRes, err := s.resRepo.Update(txCtx, *r)
		if err != nil {
			return err
		}

		if err := s.calendarRepo.ReleaseReservationNights(txCtx, updatedRes.ID); err != nil {
			return err
		}

		// Host cancellation always gives the guest a 100% full refund
		rb := models.ComputeFullRefund(*updatedRes, now)
		if err := s.issueRefundInTx(txCtx, *updatedRes, rb, actorType, &hostUserID, idemKey); err != nil {
			return err
		}

		if payout, err := s.paymentRepo.FindPayoutByReservation(txCtx, updatedRes.ID); err == nil && payout != nil {
			prevStatus := payout.Status
			if err := statemachine.ValidatePayoutTransition(payout.Status, enums.PayoutStatusCancelled); err != nil {
				return err
			}
			payout.Status = enums.PayoutStatusCancelled
			payout.UpdatedAt = now
			updatedPayout, err := s.paymentRepo.UpdatePayout(txCtx, *payout)
			if err != nil {
				return err
			}
			if err := s.audit.Record(txCtx, dtos.AuditEntryInput{
				ActorType:   actorType,
				ActorID:     &hostUserID,
				Action:      "payout.cancel",
				ObjectType:  "payout",
				ObjectID:    updatedPayout.ID,
				BeforeState: map[string]any{"status": prevStatus},
				AfterState:  map[string]any{"status": updatedPayout.Status},
			}); err != nil {
				return err
			}
		}

		return s.audit.Record(txCtx, dtos.AuditEntryInput{
			ActorType:   actorType,
			ActorID:     &hostUserID,
			Action:      "reservation.cancel_by_host",
			ObjectType:  "reservation",
			ObjectID:    updatedRes.ID,
			BeforeState: map[string]any{"status": enums.ReservationStatusConfirmed},
			AfterState: map[string]any{
				"status":              updatedRes.Status,
				"refund_amount_minor": rb.RefundAmountMinor,
			},
			Reason: in.CancelReason,
		})
	})
	if err != nil {
		return nil, err
	}

	return s.GetReservationDetail(ctx, models.AuthActor{
		SubjectID:   hostUserID,
		SubjectType: "user",
		IsHost:      true,
	}, reservationID)
}

func (s *ReservationService) CheckInByHost(
	ctx context.Context,
	hostUserID, reservationID uuid.UUID,
	idemKey string,
) (*dtos.ReservationDetailResponse, error) {
	now := s.clock.Now()
	releaseAt := now.Add(time.Duration(s.cfg.PayoutReleaseAfterCheckInHours) * time.Hour)
	var scheduledPayoutID *uuid.UUID

	err := s.db.WithinTx(ctx, func(txCtx context.Context) error {
		r, err := s.resRepo.FindByIDForUpdate(txCtx, reservationID)
		if err != nil {
			return err
		}

		lh, err := s.authorizeHostOnReservation(txCtx, r.ListingID, hostUserID, middleware.PermManageReservations)
		if err != nil {
			return err
		}

		if err := statemachine.ValidateReservationTransition(r.Status, enums.ReservationStatusCheckedIn); err != nil {
			return err
		}

		r.Status = enums.ReservationStatusCheckedIn
		r.CheckedInAt = &now
		r.UpdatedAt = now

		updatedRes, err := s.resRepo.Update(txCtx, *r)
		if err != nil {
			return err
		}

		if payout, err := s.paymentRepo.FindPayoutByReservation(txCtx, updatedRes.ID); err == nil && payout != nil {
			payout.ReleaseAt = &releaseAt
			payout.UpdatedAt = now
			updatedPayout, err := s.paymentRepo.UpdatePayout(txCtx, *payout)
			if err != nil {
				return err
			}
			scheduledPayoutID = &updatedPayout.ID

			if err := s.audit.Record(txCtx, dtos.AuditEntryInput{
				ActorType:   enums.ActorSystem,
				ActorID:     nil,
				Action:      "payout.schedule_release",
				ObjectType:  "payout",
				ObjectID:    updatedPayout.ID,
				BeforeState: map[string]any{"release_at": nil},
				AfterState:  map[string]any{"release_at": updatedPayout.ReleaseAt},
			}); err != nil {
				return err
			}
		}

		return s.audit.Record(txCtx, dtos.AuditEntryInput{
			ActorType:   lh.ActorType(),
			ActorID:     &hostUserID,
			Action:      "reservation.check_in",
			ObjectType:  "reservation",
			ObjectID:    updatedRes.ID,
			BeforeState: map[string]any{"status": enums.ReservationStatusConfirmed},
			AfterState: map[string]any{
				"status":        updatedRes.Status,
				"checked_in_at": updatedRes.CheckedInAt,
				"release_at":    releaseAt,
			},
		})
	})
	if err != nil {
		return nil, err
	}

	if s.rdb != nil && scheduledPayoutID != nil {
		_ = s.rdb.ScheduleTimer(ctx, fmt.Sprintf("release_payout:%s", scheduledPayoutID.String()), releaseAt)
	}

	return s.GetReservationDetail(ctx, models.AuthActor{
		SubjectID:   hostUserID,
		SubjectType: "user",
		IsHost:      true,
	}, reservationID)
}

func (s *ReservationService) GetHostEarnings(ctx context.Context, hostUserID uuid.UUID) (*dtos.HostEarningsResponse, error) {
	hostRecords, err := s.listingHostRepo.ListByUser(ctx, hostUserID)
	if err != nil {
		return nil, err
	}

	var permittedListingIDs []uuid.UUID
	for _, lh := range hostRecords {
		if middleware.HasListingPermission(&lh, middleware.PermViewInsights) {
			permittedListingIDs = append(permittedListingIDs, lh.ListingID)
		}
	}

	resp := &dtos.HostEarningsResponse{
		Currency: s.cfg.DefaultCurrency,
		Payouts:  []dtos.EarningsPayoutItem{},
	}
	if len(permittedListingIDs) == 0 {
		return resp, nil
	}

	items, err := s.paymentRepo.ListPayoutsByListingIDs(ctx, permittedListingIDs)
	if err != nil {
		return nil, err
	}

	resp.Payouts = items
	for _, item := range items {
		switch item.Payout.Status {
		case enums.PayoutStatusReleased:
			resp.PaidMinor += item.Payout.NetMinor
		case enums.PayoutStatusScheduled:
			resp.UpcomingMinor += item.Payout.NetMinor
		case enums.PayoutStatusHeld:
			resp.HeldMinor += item.Payout.NetMinor
		case enums.PayoutStatusCancelled:
			resp.CancelledMinor += item.Payout.NetMinor
		}
	}

	return resp, nil
}

// ExpireDuePendingReservations transitions expired pending requests (>24h) to declined, refunds 100%, and frees calendar nights.
func (s *ReservationService) ExpireDuePendingReservations(ctx context.Context, limit int) (int, error) {
	now := s.clock.Now()
	dueList, err := s.resRepo.ListDuePendingExpirations(ctx, now, limit)
	if err != nil {
		return 0, err
	}

	processed := 0
	for _, item := range dueList {
		err := s.db.WithinTx(ctx, func(txCtx context.Context) error {
			r, err := s.resRepo.FindByIDForUpdate(txCtx, item.ID)
			if err != nil {
				return err
			}
			if err := statemachine.ValidateReservationTransition(r.Status, enums.ReservationStatusDeclined); err != nil {
				return nil
			}

			reason := "host_response_timeout_24h"
			r.Status = enums.ReservationStatusDeclined
			r.DecidedAt = &now
			r.DeclineReason = &reason
			r.UpdatedAt = now

			updatedRes, err := s.resRepo.Update(txCtx, *r)
			if err != nil {
				return err
			}

			if err := s.calendarRepo.ReleaseReservationNights(txCtx, updatedRes.ID); err != nil {
				return err
			}

			rb := models.ComputeFullRefund(*updatedRes, now)
			if err := s.issueRefundInTx(txCtx, *updatedRes, rb, enums.ActorSystem, nil, "expire:"+updatedRes.ID.String()); err != nil {
				return err
			}

			return s.audit.Record(txCtx, dtos.AuditEntryInput{
				ActorType:   enums.ActorSystem,
				ActorID:     nil,
				Action:      "reservation.expire_pending",
				ObjectType:  "reservation",
				ObjectID:    updatedRes.ID,
				BeforeState: map[string]any{"status": enums.ReservationStatusPending},
				AfterState: map[string]any{
					"status":              updatedRes.Status,
					"decline_reason":      reason,
					"refund_amount_minor": rb.RefundAmountMinor,
				},
				Reason: &reason,
			})
		})
		if err == nil {
			processed++
			if s.rdb != nil {
				_ = s.rdb.RemoveTimer(ctx, fmt.Sprintf("expire_pending:%s", item.ID.String()))
			}
		}
	}
	return processed, nil
}

// ProcessDueCheckIns automatically transitions confirmed reservations whose check_in_at <= now to checked_in.
func (s *ReservationService) ProcessDueCheckIns(ctx context.Context, limit int) (int, error) {
	now := s.clock.Now()
	dueList, err := s.resRepo.ListDueCheckIns(ctx, now, limit)
	if err != nil {
		return 0, err
	}

	processed := 0
	for _, item := range dueList {
		releaseAt := now.Add(time.Duration(s.cfg.PayoutReleaseAfterCheckInHours) * time.Hour)
		var payoutID *uuid.UUID

		err := s.db.WithinTx(ctx, func(txCtx context.Context) error {
			r, err := s.resRepo.FindByIDForUpdate(txCtx, item.ID)
			if err != nil {
				return err
			}
			if err := statemachine.ValidateReservationTransition(r.Status, enums.ReservationStatusCheckedIn); err != nil {
				return nil
			}

			r.Status = enums.ReservationStatusCheckedIn
			r.CheckedInAt = &now
			r.UpdatedAt = now

			updatedRes, err := s.resRepo.Update(txCtx, *r)
			if err != nil {
				return err
			}

			if payout, err := s.paymentRepo.FindPayoutByReservation(txCtx, updatedRes.ID); err == nil && payout != nil {
				payout.ReleaseAt = &releaseAt
				payout.UpdatedAt = now
				if updatedPayout, err := s.paymentRepo.UpdatePayout(txCtx, *payout); err == nil {
					payoutID = &updatedPayout.ID
				}
			}

			return s.audit.Record(txCtx, dtos.AuditEntryInput{
				ActorType:   enums.ActorSystem,
				ActorID:     nil,
				Action:      "reservation.auto_check_in",
				ObjectType:  "reservation",
				ObjectID:    updatedRes.ID,
				BeforeState: map[string]any{"status": enums.ReservationStatusConfirmed},
				AfterState: map[string]any{
					"status":        updatedRes.Status,
					"checked_in_at": updatedRes.CheckedInAt,
					"release_at":    releaseAt,
				},
			})
		})
		if err == nil {
			processed++
			if s.rdb != nil {
				_ = s.rdb.RemoveTimer(ctx, fmt.Sprintf("auto_checkin:%s", item.ID.String()))
				if payoutID != nil {
					_ = s.rdb.ScheduleTimer(ctx, fmt.Sprintf("release_payout:%s", payoutID.String()), releaseAt)
				}
			}
		}
	}
	return processed, nil
}

// ProcessDueCheckOuts automatically transitions checked_in reservations whose check_out_at <= now to completed.
func (s *ReservationService) ProcessDueCheckOuts(ctx context.Context, limit int) (int, error) {
	now := s.clock.Now()
	dueList, err := s.resRepo.ListDueCheckOuts(ctx, now, limit)
	if err != nil {
		return 0, err
	}

	processed := 0
	for _, item := range dueList {
		err := s.db.WithinTx(ctx, func(txCtx context.Context) error {
			r, err := s.resRepo.FindByIDForUpdate(txCtx, item.ID)
			if err != nil {
				return err
			}
			if err := statemachine.ValidateReservationTransition(r.Status, enums.ReservationStatusCompleted); err != nil {
				return nil
			}

			r.Status = enums.ReservationStatusCompleted
			r.CompletedAt = &now
			r.UpdatedAt = now

			updatedRes, err := s.resRepo.Update(txCtx, *r)
			if err != nil {
				return err
			}

			return s.audit.Record(txCtx, dtos.AuditEntryInput{
				ActorType:   enums.ActorSystem,
				ActorID:     nil,
				Action:      "reservation.auto_complete",
				ObjectType:  "reservation",
				ObjectID:    updatedRes.ID,
				BeforeState: map[string]any{"status": enums.ReservationStatusCheckedIn},
				AfterState:  map[string]any{"status": updatedRes.Status, "completed_at": updatedRes.CompletedAt},
			})
		})
		if err == nil {
			processed++
			if s.rdb != nil {
				_ = s.rdb.RemoveTimer(ctx, fmt.Sprintf("auto_checkout:%s", item.ID.String()))
			}
		}
	}
	return processed, nil
}

// ReleaseDuePayouts releases scheduled payouts whose release_at (checked_in_at + 24h) <= now.
func (s *ReservationService) ReleaseDuePayouts(ctx context.Context, limit int) (int, error) {
	now := s.clock.Now()
	dueList, err := s.paymentRepo.ListDuePayouts(ctx, now, limit)
	if err != nil {
		return 0, err
	}

	processed := 0
	for _, item := range dueList {
		err := s.db.WithinTx(ctx, func(txCtx context.Context) error {
			p, err := s.paymentRepo.FindPayoutByIDForUpdate(txCtx, item.ID)
			if err != nil {
				return err
			}
			if p.ReleaseAt == nil || p.ReleaseAt.After(now) {
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

			legs := BuildPayoutReleaseLedgerEntries(
				uuid.New(),
				updatedPayout.ReservationID,
				updatedPayout.ID,
				updatedPayout.Currency,
				updatedPayout.NetMinor,
			)
			if err := s.paymentRepo.InsertLedgerEntries(txCtx, legs); err != nil {
				return err
			}

			return s.audit.Record(txCtx, dtos.AuditEntryInput{
				ActorType:   enums.ActorSystem,
				ActorID:     nil,
				Action:      "payout.release",
				ObjectType:  "payout",
				ObjectID:    updatedPayout.ID,
				BeforeState: map[string]any{"status": enums.PayoutStatusScheduled},
				AfterState: map[string]any{
					"status":         updatedPayout.Status,
					"released_at":    updatedPayout.ReleasedAt,
					"net_minor":      updatedPayout.NetMinor,
					"simulated_ref":  updatedPayout.SimulatedRef,
					"reservation_id": updatedPayout.ReservationID,
				},
			})
		})
		if err == nil {
			processed++
			if s.rdb != nil {
				_ = s.rdb.RemoveTimer(ctx, fmt.Sprintf("release_payout:%s", item.ID.String()))
			}
		}
	}
	return processed, nil
}

func (s *ReservationService) issueRefundInTx(
	txCtx context.Context,
	res models.Reservation,
	rb models.CancellationRefundBreakdown,
	actorType enums.ActorType,
	actorID *uuid.UUID,
	idemKey string,
) error {
	if rb.RefundAmountMinor <= 0 {
		return nil
	}

	var chargePaymentID *uuid.UUID
	if chg, err := s.paymentRepo.FindChargeByReservation(txCtx, res.ID); err == nil && chg != nil {
		chargePaymentID = &chg.ID
	}

	basisBytes, _ := json.Marshal(rb)
	simRef := fmt.Sprintf("sim_ref_%s", strings.ReplaceAll(res.ID.String()[:12], "-", ""))
	refundPct := rb.RefundPct

	refundPayment, err := s.paymentRepo.CreatePayment(txCtx, models.Payment{
		ReservationID:    res.ID,
		Kind:             enums.PaymentKindRefund,
		Status:           enums.PaymentStatusSucceeded,
		AmountMinor:      rb.RefundAmountMinor,
		Currency:         res.Currency,
		IdempotencyKey:   fmt.Sprintf("refund:%s:%s", res.ID.String(), idemKey),
		SimulatedRef:     &simRef,
		RelatedPaymentID: chargePaymentID,
		RefundPct:        &refundPct,
		RefundBasis:      json.RawMessage(basisBytes),
	})
	if err != nil {
		return err
	}

	legs := BuildRefundLedgerEntries(
		uuid.New(),
		res.ID,
		refundPayment.ID,
		res.Currency,
		rb,
	)
	if err := s.paymentRepo.InsertLedgerEntries(txCtx, legs); err != nil {
		return err
	}

	return s.audit.Record(txCtx, dtos.AuditEntryInput{
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
	})
}

func (s *ReservationService) authorizeHostOnReservation(
	ctx context.Context,
	listingID, userID uuid.UUID,
	perm middleware.ListingPermission,
) (*models.ListingHost, error) {
	lh, err := s.listingHostRepo.FindByListingAndUser(ctx, listingID, userID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, apperrors.ErrForbidden
		}
		return nil, err
	}
	if !middleware.HasListingPermission(lh, perm) {
		return nil, apperrors.ErrForbidden
	}
	return lh, nil
}

func (s *ReservationService) buildReservationDetail(
	ctx context.Context,
	r models.Reservation,
	includeHostFinancials bool,
) (*dtos.ReservationDetailResponse, error) {
	nights := len(models.EnumerateNights(r.CheckIn, r.CheckOut))

	listingSummary := dtos.ReservationListingSummary{
		ID: r.ListingID,
	}
	if lv, err := s.listingRepo.FindVersionByID(ctx, r.ListingVersionID); err == nil && lv != nil {
		listingSummary.Title = derefString(lv.Title)
		listingSummary.PropertyType = derefString(lv.PropertyType)
		listingSummary.City = derefString(lv.City)
		listingSummary.Country = derefString(lv.Country)
		listingSummary.AddressLine1 = derefString(lv.AddressLine1)
		if len(lv.MediaIDs) > 0 && s.mediaRepo != nil && s.storage != nil {
			if m, err := s.mediaRepo.FindByID(ctx, lv.MediaIDs[0]); err == nil {
				url := s.storage.PublicURL(m.S3Key)
				listingSummary.CoverPhotoURL = &url
			}
		}
	}

	guestSummary := dtos.ReservationUserSummary{ID: r.GuestID}
	if u, err := s.userRepo.FindByID(ctx, r.GuestID); err == nil && u != nil {
		guestSummary.FullName = u.FullName
		guestSummary.Email = u.Email
		if u.AvatarMediaID != nil && s.mediaRepo != nil && s.storage != nil {
			if m, err := s.mediaRepo.FindByID(ctx, *u.AvatarMediaID); err == nil {
				url := s.storage.PublicURL(m.S3Key)
				guestSummary.AvatarURL = &url
			}
		}
	}

	hostSummary := dtos.ReservationUserSummary{ID: r.HostID}
	if u, err := s.userRepo.FindByID(ctx, r.HostID); err == nil && u != nil {
		hostSummary.FullName = u.FullName
		hostSummary.Email = u.Email
		if u.AvatarMediaID != nil && s.mediaRepo != nil && s.storage != nil {
			if m, err := s.mediaRepo.FindByID(ctx, *u.AvatarMediaID); err == nil {
				url := s.storage.PublicURL(m.S3Key)
				hostSummary.AvatarURL = &url
			}
		}
	}

	payments, _ := s.paymentRepo.ListPaymentsByReservation(ctx, r.ID)
	if payments == nil {
		payments = []models.Payment{}
	}

	var payout *models.Payout
	var ledger []models.LedgerEntry
	if includeHostFinancials {
		payout, _ = s.paymentRepo.FindPayoutByReservation(ctx, r.ID)
		ledger, _ = s.paymentRepo.ListLedgerByReservation(ctx, r.ID)
	}

	var policy models.CancellationPolicy
	if pol, err := s.catalogService.GetCancellationPolicy(ctx, r.CancellationPolicy); err == nil && pol != nil {
		policy = *pol
	} else {
		policy = models.CancellationPolicy{Code: r.CancellationPolicy, Name: r.CancellationPolicy}
	}

	return &dtos.ReservationDetailResponse{
		Reservation:        r,
		Nights:             nights,
		Listing:            listingSummary,
		Guest:              guestSummary,
		Host:               hostSummary,
		Payments:           payments,
		Payout:             payout,
		LedgerEntries:      ledger,
		CancellationPolicy: policy,
	}, nil
}

func generateReservationCode() string {
	const alphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	var sb strings.Builder
	sb.WriteString("SH-")
	for _, v := range b {
		sb.WriteByte(alphabet[int(v)%len(alphabet)])
	}
	return sb.String()
}
