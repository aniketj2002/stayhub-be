package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/db"
)

type ReservationAlterationService struct {
	db              *db.DB
	alterationRepo  interfaces.ReservationAlterationRepository
	reservationRepo interfaces.ReservationRepository
	calendarRepo    interfaces.CalendarRepository
	listingRepo     interfaces.ListingRepository
	listingHostRepo interfaces.ListingHostRepository
	pricingService  interfaces.PricingService
	pricingRepo     interfaces.PricingRepository
	paymentRepo     interfaces.PaymentRepository
	ledger          interfaces.LedgerService
	payoutSvc       interfaces.PayoutService
	messaging       interfaces.MessagingService
	clock           clock.Clock
}

func NewReservationAlterationService(
	database *db.DB,
	alterationRepo interfaces.ReservationAlterationRepository,
	reservationRepo interfaces.ReservationRepository,
	calendarRepo interfaces.CalendarRepository,
	listingRepo interfaces.ListingRepository,
	listingHostRepo interfaces.ListingHostRepository,
	pricingService interfaces.PricingService,
	pricingRepo interfaces.PricingRepository,
	paymentRepo interfaces.PaymentRepository,
	ledger interfaces.LedgerService,
	payoutSvc interfaces.PayoutService,
	clk clock.Clock,
) interfaces.ReservationAlterationService {
	return &ReservationAlterationService{
		db:              database,
		alterationRepo:  alterationRepo,
		reservationRepo: reservationRepo,
		calendarRepo:    calendarRepo,
		listingRepo:     listingRepo,
		listingHostRepo: listingHostRepo,
		pricingService:  pricingService,
		pricingRepo:     pricingRepo,
		paymentRepo:     paymentRepo,
		ledger:          ledger,
		payoutSvc:       payoutSvc,
		clock:           clk,
	}
}

func (s *ReservationAlterationService) SetMessagingService(svc interfaces.MessagingService) {
	s.messaging = svc
}

func (s *ReservationAlterationService) RequestAlteration(
	ctx context.Context,
	guestID, reservationID uuid.UUID,
	in dtos.CreateReservationAlterationInput,
) (*dtos.ReservationAlterationResponse, error) {
	res, err := s.reservationRepo.FindByID(ctx, reservationID)
	if err != nil {
		return nil, err
	}
	if res.GuestID != guestID {
		return nil, apperrors.ErrForbidden
	}
	if res.Status != enums.ReservationStatusConfirmed {
		return nil, fmt.Errorf("%w: alterations can only be requested for confirmed reservations", apperrors.ErrValidation)
	}

	// Reject if existing pending alteration request already exists
	existingPending, err := s.alterationRepo.FindPendingByReservation(ctx, reservationID)
	if err == nil && existingPending != nil {
		return nil, fmt.Errorf("%w: an alteration request is already pending host review", apperrors.ErrValidation)
	}

	newCheckIn := res.CheckIn
	if in.CheckIn != nil && *in.CheckIn != "" {
		t, err := time.Parse("2006-01-02", *in.CheckIn)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid check_in date format, expected YYYY-MM-DD", apperrors.ErrValidation)
		}
		newCheckIn = t
	}

	newCheckOut := res.CheckOut
	if in.CheckOut != nil && *in.CheckOut != "" {
		t, err := time.Parse("2006-01-02", *in.CheckOut)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid check_out date format, expected YYYY-MM-DD", apperrors.ErrValidation)
		}
		newCheckOut = t
	}

	if !newCheckOut.After(newCheckIn) {
		return nil, fmt.Errorf("%w: check_out date must be after check_in date", apperrors.ErrValidation)
	}

	newGuests := res.Guests
	if in.Guests != nil {
		if *in.Guests < 1 {
			return nil, fmt.Errorf("%w: guest count must be at least 1", apperrors.ErrValidation)
		}
		newGuests = *in.Guests
	}

	// 1. Check availability for new nights (excluding nights already booked by this reservation)
	calEntries, err := s.calendarRepo.ListRange(ctx, res.ListingID, newCheckIn, newCheckOut)
	if err != nil {
		return nil, err
	}
	for _, entry := range calEntries {
		if entry.ReservationID != nil && *entry.ReservationID == res.ID {
			continue
		}
		return nil, fmt.Errorf("%w: requested dates are not available", apperrors.ErrDatesUnavailable)
	}

	// 2. Evaluate pricing quote for new dates
	quote, err := s.pricingService.CalculateQuote(ctx, res.ListingID, newCheckIn, newCheckOut)
	if err != nil {
		return nil, err
	}

	deltaMinor := quote.GuestTotalMinor - res.GuestTotalMinor

	// Build original snapshot and requested changes JSON
	origSnapshotMap := map[string]any{
		"check_in":          res.CheckIn.Format("2006-01-02"),
		"check_out":         res.CheckOut.Format("2006-01-02"),
		"guests":            res.Guests,
		"guest_total_minor": res.GuestTotalMinor,
		"host_net_minor":    res.HostNetMinor,
		"currency":          res.Currency,
	}
	origBytes, _ := json.Marshal(origSnapshotMap)

	requestedChangesMap := map[string]any{
		"check_in":  newCheckIn.Format("2006-01-02"),
		"check_out": newCheckOut.Format("2006-01-02"),
		"guests":    newGuests,
		"config":    in.Config,
	}
	changesBytes, _ := json.Marshal(requestedChangesMap)

	now := s.clock.Now()
	expiresAt := now.Add(24 * time.Hour)

	alt, err := s.alterationRepo.CreateAlteration(ctx, models.ReservationAlteration{
		ReservationID:    reservationID,
		RequestedBy:      guestID,
		CurrentCheckIn:   res.CheckIn,
		CurrentCheckOut:  res.CheckOut,
		NewCheckIn:       newCheckIn,
		NewCheckOut:      newCheckOut,
		OriginalSnapshot: json.RawMessage(origBytes),
		RequestedChanges: json.RawMessage(changesBytes),
		DeltaMinor:       deltaMinor,
		Currency:         res.Currency,
		Status:           enums.AlterationStatusPending,
		ExpiresAt:        expiresAt,
	})
	if err != nil {
		return nil, err
	}

	s.notifyHostOfDateChange(ctx, guestID, alt)

	return s.toAlterationResponse(alt), nil
}

func (s *ReservationAlterationService) ListPendingForHost(
	ctx context.Context,
	hostUserID uuid.UUID,
) ([]dtos.HostPendingAlteration, error) {
	hosts, err := s.listingHostRepo.ListByUser(ctx, hostUserID)
	if err != nil {
		return nil, err
	}
	listingIDs := make([]uuid.UUID, 0, len(hosts))
	for _, lh := range hosts {
		if lh.Role == enums.ListingHostRoleOwner || lh.CanManageReservations {
			listingIDs = append(listingIDs, lh.ListingID)
		}
	}
	if len(listingIDs) == 0 {
		return []dtos.HostPendingAlteration{}, nil
	}

	reservations, err := s.reservationRepo.ListByListingIDs(ctx, listingIDs)
	if err != nil {
		return nil, err
	}
	reservationIDs := make([]uuid.UUID, 0, len(reservations))
	byID := make(map[uuid.UUID]models.Reservation, len(reservations))
	for _, res := range reservations {
		reservationIDs = append(reservationIDs, res.ID)
		byID[res.ID] = res
	}

	alts, err := s.alterationRepo.ListPendingByReservationIDs(ctx, reservationIDs)
	if err != nil {
		return nil, err
	}

	out := make([]dtos.HostPendingAlteration, 0, len(alts))
	for _, alt := range alts {
		if alt.RequestedBy == hostUserID {
			continue
		}
		item := dtos.HostPendingAlteration{
			ReservationAlterationResponse: *s.toAlterationResponse(&alt),
		}
		if res, ok := byID[alt.ReservationID]; ok {
			item.ReservationCode = res.Code
			if lv, err := s.listingRepo.FindVersionByID(ctx, res.ListingVersionID); err == nil && lv != nil && lv.Title != nil {
				item.ListingTitle = *lv.Title
			}
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *ReservationAlterationService) notifyHostOfDateChange(ctx context.Context, guestID uuid.UUID, alt *models.ReservationAlteration) {
	if s.messaging == nil || alt == nil {
		return
	}
	conv, err := s.messaging.GetOrCreateConversation(ctx, alt.ReservationID)
	if err != nil || conv == nil {
		return
	}
	body := fmt.Sprintf(
		"Date change requested: %s – %s instead of %s – %s. Please approve or decline it on the reservation.",
		alt.NewCheckIn.Format("2006-01-02"),
		alt.NewCheckOut.Format("2006-01-02"),
		alt.CurrentCheckIn.Format("2006-01-02"),
		alt.CurrentCheckOut.Format("2006-01-02"),
	)
	_, _ = s.messaging.SendMessage(ctx, guestID, conv.ID, dtos.SendMessageInput{Body: body})
}

func (s *ReservationAlterationService) ListAlterations(
	ctx context.Context,
	userID, reservationID uuid.UUID,
) ([]dtos.ReservationAlterationResponse, error) {
	res, err := s.reservationRepo.FindByID(ctx, reservationID)
	if err != nil {
		return nil, err
	}

	isAuthorized := (res.GuestID == userID)
	if !isAuthorized {
		lh, err := s.listingHostRepo.FindByListingAndUser(ctx, res.ListingID, userID)
		if err == nil && lh != nil {
			isAuthorized = true
		}
	}
	if !isAuthorized {
		return nil, apperrors.ErrForbidden
	}

	list, err := s.alterationRepo.ListAlterationsByReservation(ctx, reservationID)
	if err != nil {
		return nil, err
	}

	out := make([]dtos.ReservationAlterationResponse, 0, len(list))
	for _, a := range list {
		out = append(out, *s.toAlterationResponse(&a))
	}
	return out, nil
}

func (s *ReservationAlterationService) ApproveAlteration(
	ctx context.Context,
	hostActor models.AuthActor,
	alterationID uuid.UUID,
) (*dtos.ReservationAlterationResponse, error) {
	now := s.clock.Now()
	var updatedAlt *models.ReservationAlteration

	err := s.db.WithinTx(ctx, func(txCtx context.Context) error {
		alt, err := s.alterationRepo.FindAlterationByIDForUpdate(txCtx, alterationID)
		if err != nil {
			return err
		}
		if alt.Status != enums.AlterationStatusPending {
			return fmt.Errorf("%w: alteration is not pending (status: %s)", apperrors.ErrValidation, alt.Status)
		}
		if now.After(alt.ExpiresAt) {
			_ = s.alterationRepo.UpdateAlterationStatus(txCtx, alterationID, enums.AlterationStatusExpired, &now, nil)
			return fmt.Errorf("%w: alteration request has expired", apperrors.ErrValidation)
		}

		res, err := s.reservationRepo.FindByIDForUpdate(txCtx, alt.ReservationID)
		if err != nil {
			return err
		}

		// Verify host permissions
		lh, err := s.listingHostRepo.FindByListingAndUser(txCtx, res.ListingID, hostActor.SubjectID)
		if err != nil || lh == nil || (lh.Role != "owner" && !lh.CanManageReservations) {
			return apperrors.ErrForbidden
		}

		// Check calendar availability excluding this reservation's nights
		calEntries, err := s.calendarRepo.ListRange(txCtx, res.ListingID, alt.NewCheckIn, alt.NewCheckOut)
		if err != nil {
			return err
		}
		for _, entry := range calEntries {
			if entry.ReservationID != nil && *entry.ReservationID == res.ID {
				continue
			}
			return fmt.Errorf("%w: new dates are no longer available", apperrors.ErrDatesUnavailable)
		}

		// 1. Swap calendar nights
		if err := s.calendarRepo.ReleaseReservationNights(txCtx, res.ID); err != nil {
			return err
		}
		newNights := models.EnumerateNights(alt.NewCheckIn, alt.NewCheckOut)
		if err := s.calendarRepo.BookNights(txCtx, res.ListingID, res.ID, newNights); err != nil {
			return err
		}

		// 2. Compute quote for new dates
		quote, err := s.pricingService.CalculateQuote(txCtx, res.ListingID, alt.NewCheckIn, alt.NewCheckOut)
		if err != nil {
			return err
		}

		// 3. Extract requested changes (e.g. guests, dates)
		var reqChanges map[string]any
		_ = json.Unmarshal(alt.RequestedChanges, &reqChanges)
		if gVal, ok := reqChanges["guests"].(float64); ok && int(gVal) > 0 {
			res.Guests = int(gVal)
		}

		// Compute new check-in/out instants
		loc, err := time.LoadLocation(res.Timezone)
		if err != nil {
			loc = time.UTC
		}
		inParsed, _ := time.Parse("15:04:05", res.CheckInTime)
		outParsed, _ := time.Parse("15:04:05", res.CheckOutTime)
		newCheckInAt := time.Date(alt.NewCheckIn.Year(), alt.NewCheckIn.Month(), alt.NewCheckIn.Day(),
			inParsed.Hour(), inParsed.Minute(), inParsed.Second(), 0, loc).UTC()
		newCheckOutAt := time.Date(alt.NewCheckOut.Year(), alt.NewCheckOut.Month(), alt.NewCheckOut.Day(),
			outParsed.Hour(), outParsed.Minute(), outParsed.Second(), 0, loc).UTC()

		// Preserve old financial amounts before updating reservation
		oldGuestFeeMinor := res.GuestFeeMinor
		oldHostFeeMinor := res.HostFeeMinor
		oldHostNetMinor := res.HostNetMinor

		// Update reservation fields
		res.CheckIn = alt.NewCheckIn
		res.CheckOut = alt.NewCheckOut
		res.CheckInAt = newCheckInAt
		res.CheckOutAt = newCheckOutAt
		res.NightlyTotalMinor = quote.SubtotalMinor
		res.DiscountMinor = quote.DiscountMinor
		res.CleaningFeeMinor = quote.CleaningFeeMinor
		res.GuestFeeBPS = quote.GuestFeeBPS
		res.GuestFeeMinor = quote.GuestFeeMinor
		res.GuestTotalMinor = quote.GuestTotalMinor
		res.HostFeeBPS = quote.HostFeeBPS
		res.HostFeeMinor = quote.HostFeeMinor
		res.HostNetMinor = quote.HostNetMinor
		res.UpdatedAt = now

		if _, err := s.reservationRepo.Update(txCtx, *res); err != nil {
			return err
		}

		// 4. Update reservation_nights
		var resNights []models.ReservationNight
		for _, np := range quote.Nights {
			rateType := enums.RateTypeBase
			if np.Source == "weekend" {
				rateType = enums.RateTypeWeekend
			} else if np.Source == "override" {
				rateType = enums.RateTypeOverride
			}
			resNights = append(resNights, models.ReservationNight{
				ReservationID: res.ID,
				Night:         np.Night,
				RateType:      rateType,
				PriceMinor:    np.PriceMinor,
			})
		}
		_ = s.pricingRepo.CreateReservationNights(txCtx, resNights)

		// 5. Handle money delta: charge or refund
		deltaMinor := alt.DeltaMinor
		txnID := uuid.New()
		if deltaMinor > 0 {
			payID := uuid.New()
			simRef := fmt.Sprintf("sim_chg_%s", payID.String()[:8])
			chargePayment, err := s.paymentRepo.CreatePayment(txCtx, models.Payment{
				ID:             payID,
				ReservationID:  res.ID,
				Kind:           enums.PaymentKindCharge,
				AmountMinor:    deltaMinor,
				Currency:       res.Currency,
				Status:         enums.PaymentStatusSucceeded,
				SimulatedRef:   &simRef,
				IdempotencyKey: fmt.Sprintf("alteration:charge:%s", alt.ID),
			})
			if err != nil {
				return err
			}

			deltaGuestFee := quote.GuestFeeMinor - oldGuestFeeMinor
			if deltaGuestFee < 0 {
				deltaGuestFee = 0
			}
			deltaHostFee := quote.HostFeeMinor - oldHostFeeMinor
			if deltaHostFee < 0 {
				deltaHostFee = 0
			}
			deltaHostNet := quote.HostNetMinor - oldHostNetMinor
			if deltaHostNet < 0 {
				deltaHostNet = 0
			}
			legs := BuildChargeLedgerEntries(txnID, res.ID, chargePayment.ID, res.Currency, deltaMinor, deltaGuestFee, deltaHostFee, deltaHostNet)
			if err := s.paymentRepo.InsertLedgerEntries(txCtx, legs); err != nil {
				return err
			}
		} else if deltaMinor < 0 {
			refundMinor := -deltaMinor
			payID := uuid.New()
			simRef := fmt.Sprintf("sim_ref_%s", payID.String()[:8])
			refundPayment, err := s.paymentRepo.CreatePayment(txCtx, models.Payment{
				ID:             payID,
				ReservationID:  res.ID,
				Kind:           enums.PaymentKindRefund,
				AmountMinor:    refundMinor,
				Currency:       res.Currency,
				Status:         enums.PaymentStatusSucceeded,
				SimulatedRef:   &simRef,
				IdempotencyKey: fmt.Sprintf("alteration:refund:%s", alt.ID),
			})
			if err != nil {
				return err
			}

			deltaGuestFee := oldGuestFeeMinor - quote.GuestFeeMinor
			if deltaGuestFee < 0 {
				deltaGuestFee = 0
			}
			deltaHostFee := oldHostFeeMinor - quote.HostFeeMinor
			if deltaHostFee < 0 {
				deltaHostFee = 0
			}
			deltaHostNet := oldHostNetMinor - quote.HostNetMinor
			if deltaHostNet < 0 {
				deltaHostNet = 0
			}

			rb := models.CancellationRefundBreakdown{
				RefundAmountMinor:     refundMinor,
				RefundedGuestFeeMinor: deltaGuestFee,
				RefundedHostFeeMinor:  deltaHostFee,
				RefundedHostNetMinor:  deltaHostNet,
			}
			legs := BuildRefundLedgerEntries(txnID, res.ID, refundPayment.ID, res.Currency, rb)
			if err := s.paymentRepo.InsertLedgerEntries(txCtx, legs); err != nil {
				return err
			}

			if s.payoutSvc != nil && deltaHostNet > 0 {
				_ = s.payoutSvc.AdjustPayoutOnRefund(txCtx, *res, deltaHostNet)
			}
		}

		// 6. Update alteration record
		if err := s.alterationRepo.UpdateAlterationStatus(txCtx, alterationID, enums.AlterationStatusApproved, &now, nil); err != nil {
			return err
		}
		alt.Status = enums.AlterationStatusApproved
		alt.DecidedAt = &now
		updatedAlt = alt

		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.toAlterationResponse(updatedAlt), nil
}

func (s *ReservationAlterationService) DeclineAlteration(
	ctx context.Context,
	hostActor models.AuthActor,
	alterationID uuid.UUID,
	in dtos.DeclineReservationAlterationInput,
) (*dtos.ReservationAlterationResponse, error) {
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return nil, fmt.Errorf("%w: decline reason is required", apperrors.ErrValidation)
	}

	now := s.clock.Now()
	alt, err := s.alterationRepo.FindAlterationByID(ctx, alterationID)
	if err != nil {
		return nil, err
	}
	if alt.Status != enums.AlterationStatusPending {
		return nil, fmt.Errorf("%w: alteration is not pending (status: %s)", apperrors.ErrValidation, alt.Status)
	}

	res, err := s.reservationRepo.FindByID(ctx, alt.ReservationID)
	if err != nil {
		return nil, err
	}

	lh, err := s.listingHostRepo.FindByListingAndUser(ctx, res.ListingID, hostActor.SubjectID)
	if err != nil || lh == nil || (lh.Role != "owner" && !lh.CanManageReservations) {
		return nil, apperrors.ErrForbidden
	}

	if err := s.alterationRepo.UpdateAlterationStatus(ctx, alterationID, enums.AlterationStatusDeclined, &now, &reason); err != nil {
		return nil, err
	}

	alt.Status = enums.AlterationStatusDeclined
	alt.DecidedAt = &now
	alt.DecisionReason = &reason

	return s.toAlterationResponse(alt), nil
}

func (s *ReservationAlterationService) ExpireDuePendingAlterations(ctx context.Context) (int, error) {
	now := s.clock.Now()
	expired, err := s.alterationRepo.ListExpiredPending(ctx, now)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, a := range expired {
		reason := "expired_24h_window"
		if err := s.alterationRepo.UpdateAlterationStatus(ctx, a.ID, enums.AlterationStatusExpired, &now, &reason); err == nil {
			count++
		}
	}
	return count, nil
}

func (s *ReservationAlterationService) toAlterationResponse(alt *models.ReservationAlteration) *dtos.ReservationAlterationResponse {
	var origMap map[string]any
	_ = json.Unmarshal(alt.OriginalSnapshot, &origMap)

	var reqMap map[string]any
	_ = json.Unmarshal(alt.RequestedChanges, &reqMap)

	return &dtos.ReservationAlterationResponse{
		ID:               alt.ID,
		ReservationID:    alt.ReservationID,
		RequestedBy:      alt.RequestedBy,
		CurrentCheckIn:   alt.CurrentCheckIn.Format("2006-01-02"),
		CurrentCheckOut:  alt.CurrentCheckOut.Format("2006-01-02"),
		NewCheckIn:       alt.NewCheckIn.Format("2006-01-02"),
		NewCheckOut:      alt.NewCheckOut.Format("2006-01-02"),
		OriginalSnapshot: origMap,
		RequestedChanges: reqMap,
		DeltaMinor:       alt.DeltaMinor,
		Currency:         alt.Currency,
		Status:           alt.Status,
		ExpiresAt:        alt.ExpiresAt,
		DecidedAt:        alt.DecidedAt,
		DecisionReason:   alt.DecisionReason,
		CreatedAt:        alt.CreatedAt,
	}
}
