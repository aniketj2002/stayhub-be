package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/db"
)

type CalendarService struct {
	db           *db.DB
	listingRepo  interfaces.ListingRepository
	calendarRepo interfaces.CalendarRepository
	audit        interfaces.AuditService
}

func NewCalendarService(
	database *db.DB,
	listingRepo interfaces.ListingRepository,
	calendarRepo interfaces.CalendarRepository,
	audit interfaces.AuditService,
) interfaces.CalendarService {
	return &CalendarService{
		db:           database,
		listingRepo:  listingRepo,
		calendarRepo: calendarRepo,
		audit:        audit,
	}
}

func (s *CalendarService) GetCalendar(
	ctx context.Context,
	listingID uuid.UUID,
	in dtos.CalendarQueryInput,
) (*dtos.CalendarResponse, error) {
	if _, err := s.listingRepo.FindByID(ctx, listingID); err != nil {
		return nil, err
	}

	rows, err := s.calendarRepo.ListRange(ctx, listingID, in.StartDate, in.EndDate)
	if err != nil {
		return nil, err
	}

	byDate := make(map[string]models.ListingCalendar, len(rows))
	for _, r := range rows {
		byDate[r.Night.Format("2006-01-02")] = r
	}

	allNights := models.EnumerateNights(in.StartDate, in.EndDate)
	entries := make([]dtos.CalendarNightEntry, 0, len(allNights))
	for _, night := range allNights {
		dateStr := night.Format("2006-01-02")
		if rec, exists := byDate[dateStr]; exists {
			entries = append(entries, dtos.CalendarNightEntry{
				Night:         dateStr,
				Status:        string(rec.Status),
				ReservationID: rec.ReservationID,
				BlockReason:   rec.BlockReason,
			})
		} else {
			entries = append(entries, dtos.CalendarNightEntry{
				Night:  dateStr,
				Status: "available",
			})
		}
	}

	return &dtos.CalendarResponse{
		ListingID: listingID.String(),
		StartDate: in.StartDate.Format("2006-01-02"),
		EndDate:   in.EndDate.Format("2006-01-02"),
		Nights:    entries,
	}, nil
}

func (s *CalendarService) BlockDates(
	ctx context.Context,
	listingID uuid.UUID,
	hostAccess models.ListingHost,
	in dtos.CalendarBlockInput,
) (*dtos.CalendarResponse, error) {
	nights := models.EnumerateNights(in.StartDate, in.EndDate)
	err := s.db.WithinTx(ctx, func(txCtx context.Context) error {
		if err := s.calendarRepo.BlockNights(txCtx, listingID, nights, in.BlockReason); err != nil {
			return err
		}

		actorID := hostAccess.UserID
		return s.audit.Record(txCtx, dtos.AuditEntryInput{
			ActorType:  hostAccess.ActorType(),
			ActorID:    &actorID,
			Action:     "calendar.block",
			ObjectType: "listing",
			ObjectID:   listingID,
			AfterState: map[string]any{
				"start_date":   in.StartDate.Format("2006-01-02"),
				"end_date":     in.EndDate.Format("2006-01-02"),
				"nights_count": len(nights),
				"block_reason": in.BlockReason,
			},
			Reason: in.BlockReason,
		})
	})
	if err != nil {
		return nil, err
	}

	return s.GetCalendar(ctx, listingID, dtos.CalendarQueryInput{
		StartDate: in.StartDate,
		EndDate:   in.EndDate,
	})
}

func (s *CalendarService) UnblockDates(
	ctx context.Context,
	listingID uuid.UUID,
	hostAccess models.ListingHost,
	in dtos.CalendarUnblockInput,
) (*dtos.CalendarResponse, error) {
	nights := models.EnumerateNights(in.StartDate, in.EndDate)
	err := s.db.WithinTx(ctx, func(txCtx context.Context) error {
		if err := s.calendarRepo.UnblockNights(txCtx, listingID, in.StartDate, in.EndDate); err != nil {
			return err
		}

		actorID := hostAccess.UserID
		return s.audit.Record(txCtx, dtos.AuditEntryInput{
			ActorType:  hostAccess.ActorType(),
			ActorID:    &actorID,
			Action:     "calendar.unblock",
			ObjectType: "listing",
			ObjectID:   listingID,
			AfterState: map[string]any{
				"start_date":   in.StartDate.Format("2006-01-02"),
				"end_date":     in.EndDate.Format("2006-01-02"),
				"nights_count": len(nights),
			},
		})
	})
	if err != nil {
		return nil, err
	}

	return s.GetCalendar(ctx, listingID, dtos.CalendarQueryInput{
		StartDate: in.StartDate,
		EndDate:   in.EndDate,
	})
}
