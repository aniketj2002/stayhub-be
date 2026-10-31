package services

import (
	"context"
	"fmt"
	"reflect"
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
)

type AdminListingReviewService struct {
	db          *db.DB
	listingRepo interfaces.ListingRepository
	userRepo    interfaces.UserRepository
	audit       interfaces.AuditService
	clock       clock.Clock
}

func NewAdminListingReviewService(
	database *db.DB,
	listingRepo interfaces.ListingRepository,
	userRepo interfaces.UserRepository,
	audit interfaces.AuditService,
	clk clock.Clock,
) interfaces.AdminListingReviewService {
	return &AdminListingReviewService{
		db:          database,
		listingRepo: listingRepo,
		userRepo:    userRepo,
		audit:       audit,
		clock:       clk,
	}
}

func (s *AdminListingReviewService) ListQueue(
	ctx context.Context,
	filter dtos.AdminListingReviewFilter,
) ([]dtos.AdminListingReviewQueueItem, error) {
	status := filter.Status
	if status == "" {
		status = enums.VersionStatusSubmitted
	}

	versions, err := s.listingRepo.ListVersionsByStatus(ctx, status)
	if err != nil {
		return nil, err
	}

	now := s.clock.Now()
	items := make([]dtos.AdminListingReviewQueueItem, 0, len(versions))
	for _, v := range versions {
		listing, err := s.listingRepo.FindByID(ctx, v.ListingID)
		if err != nil {
			continue
		}

		var hostName, hostEmail string
		if u, err := s.userRepo.FindByID(ctx, listing.HostID); err == nil && u != nil {
			hostName = u.FullName
			hostEmail = u.Email
		}

		checkResults, _ := s.listingRepo.ListCheckResultsByVersion(ctx, v.ID)
		if checkResults == nil {
			checkResults = []models.ListingCheckResult{}
		}

		var ageHours float64
		if v.SubmittedAt != nil {
			ageHours = now.Sub(*v.SubmittedAt).Hours()
		}

		title := ""
		if v.Title != nil {
			title = *v.Title
		}

		city := ""
		if v.City != nil {
			city = *v.City
		}
		country := ""
		if v.Country != nil {
			country = *v.Country
		}
		propType := ""
		if v.PropertyType != nil {
			propType = *v.PropertyType
		}

		items = append(items, dtos.AdminListingReviewQueueItem{
			ListingID:         v.ListingID,
			VersionID:         v.ID,
			VersionNo:         v.VersionNo,
			ListingTitle:      title,
			HostID:            listing.HostID,
			HostName:          hostName,
			HostEmail:         hostEmail,
			Status:            v.Status,
			SubmittedAt:       v.SubmittedAt,
			SubmittedAgeHours: ageHours,
			CheckResults:      checkResults,
			City:              city,
			Country:           country,
			PropertyType:      propType,
			MediaIDs:          v.MediaIDs,
			ReviewCount:       listing.ReviewCount,
			RatingAvg:         listing.RatingAvg,
		})
	}

	return items, nil
}

func (s *AdminListingReviewService) GetDiff(
	ctx context.Context,
	versionID uuid.UUID,
) (*dtos.AdminListingDiffResponse, error) {
	targetVer, err := s.listingRepo.FindVersionByID(ctx, versionID)
	if err != nil {
		return nil, err
	}

	listing, err := s.listingRepo.FindByID(ctx, targetVer.ListingID)
	if err != nil {
		return nil, err
	}

	var liveVer *models.ListingVersion
	if listing.LiveVersionID != nil {
		liveVer, _ = s.listingRepo.FindVersionByID(ctx, *listing.LiveVersionID)
	}

	diffs := make(map[string]dtos.FieldDiff)
	addDiff := func(fieldName string, oldVal, newVal any) {
		if !reflect.DeepEqual(oldVal, newVal) {
			diffs[fieldName] = dtos.FieldDiff{Old: oldVal, New: newVal}
		}
	}

	var liveVerNo *int
	if liveVer != nil {
		liveVerNo = &liveVer.VersionNo
		addDiff("title", derefString(liveVer.Title), derefString(targetVer.Title))
		addDiff("description", derefString(liveVer.Description), derefString(targetVer.Description))
		addDiff("property_type", derefString(liveVer.PropertyType), derefString(targetVer.PropertyType))
		addDiff("base_price_minor", derefInt64(liveVer.BasePriceMinor), derefInt64(targetVer.BasePriceMinor))
		addDiff("cleaning_fee_minor", liveVer.CleaningFeeMinor, targetVer.CleaningFeeMinor)
		addDiff("max_guests", derefInt(liveVer.MaxGuests), derefInt(targetVer.MaxGuests))
		addDiff("cancellation_policy", derefString(liveVer.CancellationPolicy), derefString(targetVer.CancellationPolicy))
		addDiff("house_rules", derefString(liveVer.HouseRules), derefString(targetVer.HouseRules))
		addDiff("check_in_time", liveVer.CheckInTime, targetVer.CheckInTime)
		addDiff("check_out_time", liveVer.CheckOutTime, targetVer.CheckOutTime)
		addDiff("amenities", liveVer.Amenities, targetVer.Amenities)
	} else {
		// First version submitted: diff against nil
		addDiff("title", nil, derefString(targetVer.Title))
		addDiff("description", nil, derefString(targetVer.Description))
		addDiff("property_type", nil, derefString(targetVer.PropertyType))
		addDiff("base_price_minor", nil, derefInt64(targetVer.BasePriceMinor))
		addDiff("cleaning_fee_minor", nil, targetVer.CleaningFeeMinor)
		addDiff("max_guests", nil, derefInt(targetVer.MaxGuests))
		addDiff("cancellation_policy", nil, derefString(targetVer.CancellationPolicy))
		addDiff("house_rules", nil, derefString(targetVer.HouseRules))
		addDiff("check_in_time", nil, targetVer.CheckInTime)
		addDiff("check_out_time", nil, targetVer.CheckOutTime)
		addDiff("amenities", nil, targetVer.Amenities)
	}

	photosDiff := computePhotosDiff(liveVer, targetVer)

	var hostName, hostEmail string
	if u, err := s.userRepo.FindByID(ctx, listing.HostID); err == nil && u != nil {
		hostName = u.FullName
		hostEmail = u.Email
	}

	checkResults, _ := s.listingRepo.ListCheckResultsByVersion(ctx, targetVer.ID)
	if checkResults == nil {
		checkResults = []models.ListingCheckResult{}
	}

	city := ""
	if targetVer.City != nil {
		city = *targetVer.City
	}
	country := ""
	if targetVer.Country != nil {
		country = *targetVer.Country
	}

	return &dtos.AdminListingDiffResponse{
		VersionID:     targetVer.ID,
		ListingID:     targetVer.ListingID,
		VersionNo:     targetVer.VersionNo,
		Status:        targetVer.Status,
		SubmittedAt:   targetVer.SubmittedAt,
		LiveVersionID: listing.LiveVersionID,
		LiveVersionNo: liveVerNo,
		FieldDiffs:    diffs,
		PhotosDiff:    photosDiff,
		City:          city,
		Country:       country,
		HostName:      hostName,
		HostEmail:     hostEmail,
		TargetVersion: targetVer,
		LiveVersion:   liveVer,
		CheckResults:  checkResults,
	}, nil
}

func (s *AdminListingReviewService) Approve(
	ctx context.Context,
	actor models.AuthActor,
	versionID uuid.UUID,
	note string,
) error {
	note = strings.TrimSpace(note)
	if note == "" {
		return fmt.Errorf("%w: decision note/reason is required", apperrors.ErrValidation)
	}

	now := s.clock.Now()
	return s.db.WithinTx(ctx, func(txCtx context.Context) error {
		v, err := s.listingRepo.FindVersionByID(txCtx, versionID)
		if err != nil {
			return err
		}

		if err := statemachine.ValidateVersionTransition(v.Status, enums.VersionStatusApproved); err != nil {
			return err
		}

		listing, err := s.listingRepo.FindByID(txCtx, v.ListingID)
		if err != nil {
			return err
		}

		// Supersede previous live version if present
		if listing.LiveVersionID != nil && *listing.LiveVersionID != v.ID {
			if prevLive, err := s.listingRepo.FindVersionByID(txCtx, *listing.LiveVersionID); err == nil {
				if err := statemachine.ValidateVersionTransition(prevLive.Status, enums.VersionStatusSuperseded); err == nil {
					prevLive.Status = enums.VersionStatusSuperseded
					prevLive.UpdatedAt = now
					_, _ = s.listingRepo.UpdateVersion(txCtx, *prevLive)
				}
			}
		}

		v.Status = enums.VersionStatusApproved
		v.DecidedAt = &now
		v.DecidedBy = &actor.SubjectID
		v.DecisionNote = &note
		v.UpdatedAt = now

		updatedVer, err := s.listingRepo.UpdateVersion(txCtx, *v)
		if err != nil {
			return err
		}

		nextListingStatus := enums.ListingStatusLive
		var workingVerID *uuid.UUID = listing.WorkingVersionID
		if workingVerID != nil && *workingVerID == updatedVer.ID {
			workingVerID = nil
		}

		updatedListing, err := s.listingRepo.UpdatePointersAndStatus(
			txCtx,
			listing.ID,
			nextListingStatus,
			&updatedVer.ID,
			workingVerID,
			now,
		)
		if err != nil {
			return err
		}

		if s.audit != nil {
			_ = s.audit.Record(txCtx, dtos.AuditEntryInput{
				ActorType:  enums.ActorAdmin,
				ActorID:    &actor.SubjectID,
				Action:     "listing.approve",
				ObjectType: "listing",
				ObjectID:   listing.ID,
				AfterState: map[string]any{
					"listing_status":  updatedListing.Status,
					"live_version_id": updatedListing.LiveVersionID,
					"version_status":  updatedVer.Status,
					"note":            note,
				},
				Reason: &note,
			})
		}
		return nil
	})
}

func (s *AdminListingReviewService) RequestChanges(
	ctx context.Context,
	actor models.AuthActor,
	versionID uuid.UUID,
	note string,
) error {
	note = strings.TrimSpace(note)
	if note == "" {
		return fmt.Errorf("%w: changes requested note is required", apperrors.ErrValidation)
	}

	now := s.clock.Now()
	return s.db.WithinTx(ctx, func(txCtx context.Context) error {
		v, err := s.listingRepo.FindVersionByID(txCtx, versionID)
		if err != nil {
			return err
		}

		if err := statemachine.ValidateVersionTransition(v.Status, enums.VersionStatusChangesRequested); err != nil {
			return err
		}

		v.Status = enums.VersionStatusChangesRequested
		v.DecidedAt = &now
		v.DecidedBy = &actor.SubjectID
		v.DecisionNote = &note
		v.UpdatedAt = now

		updatedVer, err := s.listingRepo.UpdateVersion(txCtx, *v)
		if err != nil {
			return err
		}

		if s.audit != nil {
			_ = s.audit.Record(txCtx, dtos.AuditEntryInput{
				ActorType:  enums.ActorAdmin,
				ActorID:    &actor.SubjectID,
				Action:     "listing.request_changes",
				ObjectType: "listing",
				ObjectID:   v.ListingID,
				AfterState: map[string]any{
					"version_id":     updatedVer.ID,
					"version_status": updatedVer.Status,
					"note":           note,
				},
				Reason: &note,
			})
		}
		return nil
	})
}

func (s *AdminListingReviewService) Reject(
	ctx context.Context,
	actor models.AuthActor,
	versionID uuid.UUID,
	note string,
) error {
	note = strings.TrimSpace(note)
	if note == "" {
		return fmt.Errorf("%w: rejection note is required", apperrors.ErrValidation)
	}

	now := s.clock.Now()
	return s.db.WithinTx(ctx, func(txCtx context.Context) error {
		v, err := s.listingRepo.FindVersionByID(txCtx, versionID)
		if err != nil {
			return err
		}

		if err := statemachine.ValidateVersionTransition(v.Status, enums.VersionStatusRejected); err != nil {
			return err
		}

		v.Status = enums.VersionStatusRejected
		v.DecidedAt = &now
		v.DecidedBy = &actor.SubjectID
		v.DecisionNote = &note
		v.UpdatedAt = now

		updatedVer, err := s.listingRepo.UpdateVersion(txCtx, *v)
		if err != nil {
			return err
		}

		listing, err := s.listingRepo.FindByID(txCtx, v.ListingID)
		if err != nil {
			return err
		}

		// If no live version exists, mark listing as rejected
		if listing.LiveVersionID == nil {
			_, _ = s.listingRepo.UpdatePointersAndStatus(
				txCtx,
				listing.ID,
				enums.ListingStatusRejected,
				nil,
				listing.WorkingVersionID,
				now,
			)
		}

		if s.audit != nil {
			_ = s.audit.Record(txCtx, dtos.AuditEntryInput{
				ActorType:  enums.ActorAdmin,
				ActorID:    &actor.SubjectID,
				Action:     "listing.reject",
				ObjectType: "listing",
				ObjectID:   v.ListingID,
				AfterState: map[string]any{
					"version_id":     updatedVer.ID,
					"version_status": updatedVer.Status,
					"note":           note,
				},
				Reason: &note,
			})
		}
		return nil
	})
}

func computePhotosDiff(oldVer, newVer *models.ListingVersion) dtos.PhotosDiff {
	var oldIDs []uuid.UUID
	if oldVer != nil {
		oldIDs = oldVer.MediaIDs
	}
	newIDs := newVer.MediaIDs

	oldSet := make(map[uuid.UUID]bool, len(oldIDs))
	for _, id := range oldIDs {
		oldSet[id] = true
	}
	newSet := make(map[uuid.UUID]bool, len(newIDs))
	for _, id := range newIDs {
		newSet[id] = true
	}

	var added []uuid.UUID
	for _, id := range newIDs {
		if !oldSet[id] {
			added = append(added, id)
		}
	}

	var removed []uuid.UUID
	for _, id := range oldIDs {
		if !newSet[id] {
			removed = append(removed, id)
		}
	}

	// Check reordering: filter both to shared IDs and check order
	var sharedInOld []uuid.UUID
	for _, id := range oldIDs {
		if newSet[id] {
			sharedInOld = append(sharedInOld, id)
		}
	}
	var sharedInNew []uuid.UUID
	for _, id := range newIDs {
		if oldSet[id] {
			sharedInNew = append(sharedInNew, id)
		}
	}

	reordered := !reflect.DeepEqual(sharedInOld, sharedInNew)

	return dtos.PhotosDiff{
		Added:     added,
		Removed:   removed,
		Reordered: reordered,
	}
}

func derefInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

func derefInt64(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}
