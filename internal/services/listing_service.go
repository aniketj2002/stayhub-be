package services

import (
	"context"
	"encoding/json"
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
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/db"
	"github.com/stayhub/backend/internal/platform/storage"
)

type ListingService struct {
	cfg             *config.Config
	db              *db.DB
	listingRepo     interfaces.ListingRepository
	listingHostRepo interfaces.ListingHostRepository
	userRepo        interfaces.UserRepository
	mediaRepo       interfaces.MediaRepository
	catalogService  interfaces.CatalogService
	storage         *storage.Storage
	audit           interfaces.AuditService
	clock           clock.Clock
}

func NewListingService(
	cfg *config.Config,
	database *db.DB,
	listingRepo interfaces.ListingRepository,
	listingHostRepo interfaces.ListingHostRepository,
	userRepo interfaces.UserRepository,
	mediaRepo interfaces.MediaRepository,
	catalogService interfaces.CatalogService,
	store *storage.Storage,
	audit interfaces.AuditService,
	clk clock.Clock,
) interfaces.ListingService {
	return &ListingService{
		cfg:             cfg,
		db:              database,
		listingRepo:     listingRepo,
		listingHostRepo: listingHostRepo,
		userRepo:        userRepo,
		mediaRepo:       mediaRepo,
		catalogService:  catalogService,
		storage:         store,
		audit:           audit,
		clock:           clk,
	}
}

func (s *ListingService) CreateListing(
	ctx context.Context,
	actor models.AuthActor,
	in dtos.CreateListingInput,
) (*dtos.HostListingDetailResponse, error) {
	now := s.clock.Now()
	tz := strings.TrimSpace(in.Timezone)
	if tz == "" {
		tz = "UTC"
	}
	curr := strings.ToUpper(strings.TrimSpace(in.Currency))
	if curr == "" {
		curr = s.cfg.DefaultCurrency
	}

	var listing *models.Listing
	var version *models.ListingVersion
	var hostAccess *models.ListingHost

	err := s.db.WithinTx(ctx, func(txCtx context.Context) error {
		if !actor.IsHost && s.userRepo != nil {
			if _, err := s.userRepo.EnableHostRole(txCtx, actor.SubjectID, now); err != nil {
				return err
			}
		}

		createdListing, err := s.listingRepo.CreateListing(txCtx, actor.SubjectID)
		if err != nil {
			return err
		}

		createdVersion, err := s.listingRepo.CreateVersion(txCtx, models.ListingVersion{
			ListingID:    createdListing.ID,
			VersionNo:    1,
			Status:       enums.VersionStatusDraft,
			WizardStep:   1,
			PropertyType: in.PropertyType,
			Title:        in.Title,
			Timezone:     tz,
			Currency:     curr,
			CheckInTime:  "15:00",
			CheckOutTime: "11:00",
			Amenities:    []string{},
			MediaIDs:     []uuid.UUID{},
		})
		if err != nil {
			return err
		}

		updatedListing, err := s.listingRepo.UpdatePointersAndStatus(
			txCtx,
			createdListing.ID,
			enums.ListingStatusDraft,
			nil,
			&createdVersion.ID,
			now,
		)
		if err != nil {
			return err
		}

		createdHostAccess, err := s.listingHostRepo.CreateOwner(txCtx, createdListing.ID, actor.SubjectID, []byte(`{}`))
		if err != nil {
			return err
		}

		listing = updatedListing
		version = createdVersion
		hostAccess = createdHostAccess

		actorID := actor.SubjectID
		return s.audit.Record(txCtx, dtos.AuditEntryInput{
			ActorType:  enums.ActorHost,
			ActorID:    &actorID,
			Action:     "listing.create",
			ObjectType: "listing",
			ObjectID:   updatedListing.ID,
			AfterState: map[string]any{
				"status":             updatedListing.Status,
				"working_version_id": updatedListing.WorkingVersionID,
				"version_no":         createdVersion.VersionNo,
			},
		})
	})
	if err != nil {
		return nil, err
	}

	workingResp := s.buildVersionResponse(ctx, *version)
	return &dtos.HostListingDetailResponse{
		Listing:        *listing,
		WorkingVersion: &workingResp,
		LiveVersion:    nil,
		CheckResults:   []models.ListingCheckResult{},
		HostAccess:     *hostAccess,
	}, nil
}

func (s *ListingService) ListHostListings(ctx context.Context, userID uuid.UUID) ([]dtos.HostListingSummaryResponse, error) {
	hostRecords, err := s.listingHostRepo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	out := make([]dtos.HostListingSummaryResponse, 0, len(hostRecords))
	for _, lh := range hostRecords {
		l, err := s.listingRepo.FindByID(ctx, lh.ListingID)
		if err != nil {
			continue
		}

		var ver *models.ListingVersion
		if l.WorkingVersionID != nil {
			ver, _ = s.listingRepo.FindVersionByID(ctx, *l.WorkingVersionID)
		}
		if ver == nil && l.LiveVersionID != nil {
			ver, _ = s.listingRepo.FindVersionByID(ctx, *l.LiveVersionID)
		}

		summary := dtos.HostListingSummaryResponse{
			ID:         l.ID,
			HostID:     l.HostID,
			Status:     l.Status,
			Currency:   s.cfg.DefaultCurrency,
			WizardStep: 1,
			HostRole:   lh.Role,
			HostAccess: lh,
			CreatedAt:  l.CreatedAt,
			UpdatedAt:  l.UpdatedAt,
		}
		if ver != nil {
			summary.Title = ver.Title
			summary.PropertyType = ver.PropertyType
			summary.City = ver.City
			summary.Country = ver.Country
			summary.MaxGuests = ver.MaxGuests
			summary.BasePriceMinor = ver.BasePriceMinor
			summary.Currency = ver.Currency
			summary.InstantBook = ver.InstantBook
			summary.WizardStep = ver.WizardStep
			if len(ver.MediaIDs) > 0 {
				if m, err := s.mediaRepo.FindByID(ctx, ver.MediaIDs[0]); err == nil && m.Status == "uploaded" {
					url := s.storage.PublicURL(m.S3Key)
					summary.CoverPhotoURL = &url
				}
			}
		}
		out = append(out, summary)
	}
	return out, nil
}

func (s *ListingService) GetHostListing(
	ctx context.Context,
	listingID uuid.UUID,
	hostAccess models.ListingHost,
) (*dtos.HostListingDetailResponse, error) {
	l, err := s.listingRepo.FindByID(ctx, listingID)
	if err != nil {
		return nil, err
	}

	var workingResp *dtos.ListingVersionResponse
	var checkVersionID *uuid.UUID
	if l.WorkingVersionID != nil {
		wv, err := s.listingRepo.FindVersionByID(ctx, *l.WorkingVersionID)
		if err != nil {
			return nil, err
		}
		resp := s.buildVersionResponse(ctx, *wv)
		workingResp = &resp
		checkVersionID = &wv.ID
	}

	var liveResp *dtos.ListingVersionResponse
	if l.LiveVersionID != nil {
		lv, err := s.listingRepo.FindVersionByID(ctx, *l.LiveVersionID)
		if err != nil {
			return nil, err
		}
		resp := s.buildVersionResponse(ctx, *lv)
		liveResp = &resp
		if checkVersionID == nil {
			checkVersionID = &lv.ID
		}
	}

	checkResults := make([]models.ListingCheckResult, 0)
	if checkVersionID != nil {
		if results, err := s.listingRepo.ListCheckResultsByVersion(ctx, *checkVersionID); err == nil {
			checkResults = results
		}
	}

	return &dtos.HostListingDetailResponse{
		Listing:        *l,
		WorkingVersion: workingResp,
		LiveVersion:    liveResp,
		CheckResults:   checkResults,
		HostAccess:     hostAccess,
	}, nil
}

func (s *ListingService) UpdateWorkingVersion(
	ctx context.Context,
	listingID uuid.UUID,
	hostAccess models.ListingHost,
	in dtos.UpdateWorkingVersionInput,
) (*dtos.HostListingDetailResponse, error) {
	if in.Amenities != nil && len(*in.Amenities) > 0 {
		catalogAmenities, err := s.catalogService.ListAmenities(ctx)
		if err != nil {
			return nil, err
		}
		validCodes := make(map[string]bool, len(catalogAmenities))
		for _, a := range catalogAmenities {
			validCodes[a.Code] = true
		}
		for _, code := range *in.Amenities {
			if !validCodes[code] {
				return nil, apperrors.ErrInvalidAmenityCode
			}
		}
	}

	if in.CancellationPolicy != nil && strings.TrimSpace(*in.CancellationPolicy) != "" {
		code := strings.ToLower(strings.TrimSpace(*in.CancellationPolicy))
		if _, err := s.catalogService.GetCancellationPolicy(ctx, code); err != nil {
			return nil, apperrors.ErrInvalidCancellationPolicy
		}
		in.CancellationPolicy = &code
	}

	if in.MediaIDs != nil && len(*in.MediaIDs) > 0 {
		if err := s.validateListingMediaIDs(ctx, listingID, *in.MediaIDs); err != nil {
			return nil, err
		}
	}

	now := s.clock.Now()
	err := s.db.WithinTx(ctx, func(txCtx context.Context) error {
		l, err := s.listingRepo.FindByID(txCtx, listingID)
		if err != nil {
			return err
		}

		var baseVersion *models.ListingVersion
		if l.WorkingVersionID != nil {
			baseVersion, err = s.listingRepo.FindVersionByID(txCtx, *l.WorkingVersionID)
			if err != nil {
				return err
			}
		} else if l.LiveVersionID != nil {
			baseVersion, err = s.listingRepo.FindVersionByID(txCtx, *l.LiveVersionID)
			if err != nil {
				return err
			}
		} else {
			return apperrors.ErrNotFound
		}

		beforeStep := baseVersion.WizardStep
		beforeStatus := baseVersion.Status

		// Fork a new draft version if the working version is currently the approved live version
		needsFork := (l.LiveVersionID != nil && baseVersion.ID == *l.LiveVersionID) ||
			(baseVersion.Status != enums.VersionStatusDraft && baseVersion.Status != enums.VersionStatusChangesRequested)

		var target models.ListingVersion
		if needsFork {
			target = *baseVersion
			target.VersionNo = baseVersion.VersionNo + 1
			target.Status = enums.VersionStatusDraft
			target.SubmittedAt = nil
			target.DecidedAt = nil
			applyWorkingVersionPatch(&target, in)

			createdVer, err := s.listingRepo.CreateVersion(txCtx, target)
			if err != nil {
				return err
			}
			if _, err := s.listingRepo.UpdatePointersAndStatus(
				txCtx,
				l.ID,
				l.Status,
				l.LiveVersionID,
				&createdVer.ID,
				now,
			); err != nil {
				return err
			}
			target = *createdVer
		} else {
			target = *baseVersion
			target.Status = enums.VersionStatusDraft
			target.UpdatedAt = now
			applyWorkingVersionPatch(&target, in)

			updatedVer, err := s.listingRepo.UpdateVersion(txCtx, target)
			if err != nil {
				return err
			}
			target = *updatedVer
		}

		actorID := hostAccess.UserID
		return s.audit.Record(txCtx, dtos.AuditEntryInput{
			ActorType:  hostAccess.ActorType(),
			ActorID:    &actorID,
			Action:     "listing.update_draft",
			ObjectType: "listing",
			ObjectID:   listingID,
			BeforeState: map[string]any{
				"version_id":  baseVersion.ID,
				"version_no":  baseVersion.VersionNo,
				"wizard_step": beforeStep,
				"status":      beforeStatus,
			},
			AfterState: map[string]any{
				"version_id":  target.ID,
				"version_no":  target.VersionNo,
				"wizard_step": target.WizardStep,
				"status":      target.Status,
			},
		})
	})
	if err != nil {
		return nil, err
	}

	return s.GetHostListing(ctx, listingID, hostAccess)
}

func (s *ListingService) SubmitListing(
	ctx context.Context,
	listingID uuid.UUID,
	hostAccess models.ListingHost,
) (*dtos.SubmitListingResponse, error) {
	l, err := s.listingRepo.FindByID(ctx, listingID)
	if err != nil {
		return nil, err
	}
	if l.WorkingVersionID == nil {
		return nil, apperrors.ErrNotFound
	}
	wv, err := s.listingRepo.FindVersionByID(ctx, *l.WorkingVersionID)
	if err != nil {
		return nil, err
	}

	// Load valid policies & media records for automated check evaluation
	policies, err := s.catalogService.ListCancellationPolicies(ctx)
	if err != nil {
		return nil, err
	}
	validPolicies := make(map[string]bool, len(policies))
	for _, p := range policies {
		validPolicies[p.Code] = true
	}

	mediaRecords, err := s.mediaRepo.FindByIDs(ctx, wv.MediaIDs)
	if err != nil {
		return nil, err
	}
	hostMembers, err := s.listingHostRepo.ListByListing(ctx, listingID)
	if err != nil {
		return nil, err
	}
	authorizedUploaders := map[uuid.UUID]bool{
		l.HostID: true,
	}
	for _, hm := range hostMembers {
		authorizedUploaders[hm.UserID] = true
	}

	rawResults, allPassed := EvaluateListingChecks(*wv, validPolicies, mediaRecords, authorizedUploaders)

	now := s.clock.Now()
	var savedResults []models.ListingCheckResult
	var finalListing *models.Listing
	var finalVersion *models.ListingVersion

	txErr := s.db.WithinTx(ctx, func(txCtx context.Context) error {
		res, err := s.listingRepo.ReplaceCheckResults(txCtx, wv.ID, rawResults)
		if err != nil {
			return err
		}
		savedResults = res

		actorID := hostAccess.UserID
		if !allPassed {
			finalListing = l
			finalVersion = wv
			return s.audit.Record(txCtx, dtos.AuditEntryInput{
				ActorType:  hostAccess.ActorType(),
				ActorID:    &actorID,
				Action:     "listing.submit_failed",
				ObjectType: "listing",
				ObjectID:   listingID,
				AfterState: map[string]any{
					"version_id":    wv.ID,
					"version_no":    wv.VersionNo,
					"check_results": savedResults,
				},
			})
		}

		wv.SubmittedAt = &now
		wv.UpdatedAt = now

		if s.cfg.VersionMode == "v2" {
			if err := statemachine.ValidateVersionTransition(wv.Status, enums.VersionStatusSubmitted); err != nil {
				return err
			}
			wv.Status = enums.VersionStatusSubmitted
			updatedVer, err := s.listingRepo.UpdateVersion(txCtx, *wv)
			if err != nil {
				return err
			}
			finalVersion = updatedVer

			nextListingStatus := l.Status
			if l.LiveVersionID == nil {
				nextListingStatus = enums.ListingStatusPendingReview
			}
			if err := statemachine.ValidateListingTransition(l.Status, nextListingStatus); err != nil {
				return err
			}
			updatedListing, err := s.listingRepo.UpdatePointersAndStatus(
				txCtx,
				l.ID,
				nextListingStatus,
				l.LiveVersionID,
				&updatedVer.ID,
				now,
			)
			if err != nil {
				return err
			}
			finalListing = updatedListing

			return s.audit.Record(txCtx, dtos.AuditEntryInput{
				ActorType:   hostAccess.ActorType(),
				ActorID:     &actorID,
				Action:      "listing.submit",
				ObjectType:  "listing",
				ObjectID:    listingID,
				BeforeState: map[string]any{"status": l.Status, "version_status": enums.VersionStatusDraft},
				AfterState:  map[string]any{"status": updatedListing.Status, "version_status": updatedVer.Status},
			})
		}

		// V1: Auto-approve when all 3 automated checks pass
		if l.LiveVersionID != nil && *l.LiveVersionID != wv.ID {
			if prevLive, err := s.listingRepo.FindVersionByID(txCtx, *l.LiveVersionID); err == nil {
				if err := statemachine.ValidateVersionTransition(prevLive.Status, enums.VersionStatusSuperseded); err != nil {
					return err
				}
				prevLive.Status = enums.VersionStatusSuperseded
				prevLive.UpdatedAt = now
				_, _ = s.listingRepo.UpdateVersion(txCtx, *prevLive)
			}
		}

		if err := statemachine.ValidateVersionTransition(wv.Status, enums.VersionStatusApproved); err != nil {
			return err
		}
		wv.Status = enums.VersionStatusApproved
		wv.DecidedAt = &now
		updatedVer, err := s.listingRepo.UpdateVersion(txCtx, *wv)
		if err != nil {
			return err
		}
		finalVersion = updatedVer

		if err := statemachine.ValidateListingTransition(l.Status, enums.ListingStatusLive); err != nil {
			return err
		}
		updatedListing, err := s.listingRepo.UpdatePointersAndStatus(
			txCtx,
			l.ID,
			enums.ListingStatusLive,
			&updatedVer.ID,
			&updatedVer.ID,
			now,
		)
		if err != nil {
			return err
		}
		finalListing = updatedListing

		if err := s.audit.Record(txCtx, dtos.AuditEntryInput{
			ActorType:   hostAccess.ActorType(),
			ActorID:     &actorID,
			Action:      "listing.submit",
			ObjectType:  "listing",
			ObjectID:    listingID,
			BeforeState: map[string]any{"status": l.Status, "version_status": enums.VersionStatusDraft},
			AfterState:  map[string]any{"status": enums.ListingStatusPendingReview, "version_status": enums.VersionStatusSubmitted},
		}); err != nil {
			return err
		}

		return s.audit.Record(txCtx, dtos.AuditEntryInput{
			ActorType:   enums.ActorSystem,
			ActorID:     nil,
			Action:      "listing.auto_approve",
			ObjectType:  "listing",
			ObjectID:    listingID,
			BeforeState: map[string]any{"status": l.Status, "live_version_id": l.LiveVersionID},
			AfterState: map[string]any{
				"status":          updatedListing.Status,
				"live_version_id": updatedListing.LiveVersionID,
				"version_no":      updatedVer.VersionNo,
			},
		})
	})
	if txErr != nil {
		return nil, txErr
	}

	resp := &dtos.SubmitListingResponse{
		Listing:      *finalListing,
		Version:      s.buildVersionResponse(ctx, *finalVersion),
		CheckResults: savedResults,
		AllPassed:    allPassed,
	}
	if !allPassed {
		return resp, apperrors.ErrListingChecksFailed
	}
	return resp, nil
}

func (s *ListingService) UpdateHostConfig(
	ctx context.Context,
	listingID uuid.UUID,
	hostAccess models.ListingHost,
	in dtos.UpdateHostConfigInput,
) (*models.ListingHost, error) {
	var updated *models.ListingHost
	err := s.db.WithinTx(ctx, func(txCtx context.Context) error {
		lh, err := s.listingHostRepo.UpdateConfig(txCtx, listingID, hostAccess.UserID, in.Config)
		if err != nil {
			return err
		}
		updated = lh

		actorID := hostAccess.UserID
		return s.audit.Record(txCtx, dtos.AuditEntryInput{
			ActorType:   hostAccess.ActorType(),
			ActorID:     &actorID,
			Action:      "listing_host.update_config",
			ObjectType:  "listing",
			ObjectID:    listingID,
			BeforeState: map[string]any{"config": json.RawMessage(hostAccess.Config)},
			AfterState:  map[string]any{"config": json.RawMessage(lh.Config)},
		})
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// EvaluateListingChecks runs the 3 automated checks required at listing submission:
// 1. required_fields
// 2. min_photos (>= 5 confirmed photos uploaded by authorized listing hosts)
// 3. house_rules_policy (house_rules non-empty + valid cancellation_policy)
func EvaluateListingChecks(
	v models.ListingVersion,
	validPolicies map[string]bool,
	mediaRecords []models.Media,
	authorizedUploaders map[uuid.UUID]bool,
) ([]models.ListingCheckResult, bool) {
	var results []models.ListingCheckResult
	allPassed := true

	// 1. required_fields
	var missing []string
	if v.PropertyType == nil || strings.TrimSpace(*v.PropertyType) == "" {
		missing = append(missing, "property_type")
	}
	if v.Title == nil || strings.TrimSpace(*v.Title) == "" {
		missing = append(missing, "title")
	}
	if v.Description == nil || strings.TrimSpace(*v.Description) == "" {
		missing = append(missing, "description")
	}
	if v.AddressLine1 == nil || strings.TrimSpace(*v.AddressLine1) == "" {
		missing = append(missing, "address_line1")
	}
	if v.City == nil || strings.TrimSpace(*v.City) == "" {
		missing = append(missing, "city")
	}
	if v.Country == nil || len(strings.TrimSpace(*v.Country)) != 2 {
		missing = append(missing, "country")
	}
	if strings.TrimSpace(v.Timezone) == "" {
		missing = append(missing, "timezone")
	} else if _, err := time.LoadLocation(strings.TrimSpace(v.Timezone)); err != nil {
		missing = append(missing, "timezone (invalid IANA timezone)")
	}
	if v.MaxGuests == nil || *v.MaxGuests <= 0 {
		missing = append(missing, "max_guests")
	}
	if v.BasePriceMinor == nil || *v.BasePriceMinor <= 0 {
		missing = append(missing, "base_price_minor")
	}
	if strings.TrimSpace(v.CheckInTime) == "" {
		missing = append(missing, "check_in_time")
	}
	if strings.TrimSpace(v.CheckOutTime) == "" {
		missing = append(missing, "check_out_time")
	}

	if len(missing) > 0 {
		allPassed = false
		detail := "Missing or invalid required fields: " + strings.Join(missing, ", ")
		results = append(results, models.ListingCheckResult{
			ListingVersionID: v.ID,
			CheckCode:        "required_fields",
			Outcome:          "fail",
			Detail:           &detail,
		})
	} else {
		detail := "All required listing fields are complete"
		results = append(results, models.ListingCheckResult{
			ListingVersionID: v.ID,
			CheckCode:        "required_fields",
			Outcome:          "pass",
			Detail:           &detail,
		})
	}

	// 2. min_photos (>= 5 confirmed photos)
	mediaByID := make(map[uuid.UUID]models.Media, len(mediaRecords))
	for _, m := range mediaRecords {
		mediaByID[m.ID] = m
	}

	validPhotoCount := 0
	var photoErrDetail string
	seenMedia := make(map[uuid.UUID]bool)
	for _, id := range v.MediaIDs {
		if seenMedia[id] {
			photoErrDetail = "Duplicate media_ids are not allowed"
			break
		}
		seenMedia[id] = true
		m, exists := mediaByID[id]
		if !exists {
			photoErrDetail = fmt.Sprintf("Media ID %s does not exist", id.String())
			break
		}
		if m.Status != "uploaded" {
			photoErrDetail = fmt.Sprintf("Media ID %s is not confirmed as uploaded", id.String())
			break
		}
		if len(authorizedUploaders) > 0 && !authorizedUploaders[m.UploadedBy] {
			photoErrDetail = fmt.Sprintf("Media ID %s was not uploaded by an authorized host of this listing", id.String())
			break
		}
		validPhotoCount++
	}

	if photoErrDetail != "" {
		allPassed = false
		results = append(results, models.ListingCheckResult{
			ListingVersionID: v.ID,
			CheckCode:        "min_photos",
			Outcome:          "fail",
			Detail:           &photoErrDetail,
		})
	} else if validPhotoCount < 5 {
		allPassed = false
		detail := fmt.Sprintf("At least 5 confirmed photos are required (found %d)", validPhotoCount)
		results = append(results, models.ListingCheckResult{
			ListingVersionID: v.ID,
			CheckCode:        "min_photos",
			Outcome:          "fail",
			Detail:           &detail,
		})
	} else {
		detail := fmt.Sprintf("%d confirmed photos verified", validPhotoCount)
		results = append(results, models.ListingCheckResult{
			ListingVersionID: v.ID,
			CheckCode:        "min_photos",
			Outcome:          "pass",
			Detail:           &detail,
		})
	}

	// 3. house_rules_policy
	hasRules := v.HouseRules != nil && strings.TrimSpace(*v.HouseRules) != ""
	hasValidPolicy := v.CancellationPolicy != nil && validPolicies[strings.ToLower(strings.TrimSpace(*v.CancellationPolicy))]
	if !hasRules || !hasValidPolicy {
		allPassed = false
		detail := "Both non-empty house_rules and a valid cancellation_policy (flexible, moderate, firm, strict) are required"
		results = append(results, models.ListingCheckResult{
			ListingVersionID: v.ID,
			CheckCode:        "house_rules_policy",
			Outcome:          "fail",
			Detail:           &detail,
		})
	} else {
		detail := "House rules and cancellation policy verified"
		results = append(results, models.ListingCheckResult{
			ListingVersionID: v.ID,
			CheckCode:        "house_rules_policy",
			Outcome:          "pass",
			Detail:           &detail,
		})
	}

	return results, allPassed
}

func (s *ListingService) validateListingMediaIDs(ctx context.Context, listingID uuid.UUID, mediaIDs []uuid.UUID) error {
	l, err := s.listingRepo.FindByID(ctx, listingID)
	if err != nil {
		return err
	}
	hostMembers, err := s.listingHostRepo.ListByListing(ctx, listingID)
	if err != nil {
		return err
	}
	authorizedUploaders := map[uuid.UUID]bool{
		l.HostID: true,
	}
	for _, hm := range hostMembers {
		authorizedUploaders[hm.UserID] = true
	}

	records, err := s.mediaRepo.FindByIDs(ctx, mediaIDs)
	if err != nil {
		return err
	}
	byID := make(map[uuid.UUID]models.Media, len(records))
	for _, m := range records {
		byID[m.ID] = m
	}

	for _, id := range mediaIDs {
		m, exists := byID[id]
		if !exists {
			return apperrors.ErrNotFound
		}
		if m.Status != "uploaded" {
			return apperrors.ErrMediaNotUploaded
		}
		if !authorizedUploaders[m.UploadedBy] {
			return apperrors.ErrMediaOwnership
		}
	}
	return nil
}

func (s *ListingService) buildVersionResponse(ctx context.Context, v models.ListingVersion) dtos.ListingVersionResponse {
	photos := make([]dtos.MediaResponse, 0, len(v.MediaIDs))
	if len(v.MediaIDs) > 0 && s.mediaRepo != nil && s.storage != nil {
		if records, err := s.mediaRepo.FindByIDs(ctx, v.MediaIDs); err == nil {
			byID := make(map[uuid.UUID]models.Media, len(records))
			for _, m := range records {
				byID[m.ID] = m
			}
			for _, id := range v.MediaIDs {
				if m, ok := byID[id]; ok {
					photos = append(photos, dtos.ToMediaResponse(m, s.storage.PublicURL(m.S3Key)))
				}
			}
		}
	}
	return dtos.ListingVersionResponse{
		ListingVersion: v,
		Photos:         photos,
	}
}

func applyWorkingVersionPatch(v *models.ListingVersion, in dtos.UpdateWorkingVersionInput) {
	if in.WizardStep != nil && *in.WizardStep > 0 {
		v.WizardStep = *in.WizardStep
	}
	if in.PropertyType != nil {
		v.PropertyType = in.PropertyType
	}
	if in.Title != nil {
		v.Title = in.Title
	}
	if in.Description != nil {
		v.Description = in.Description
	}
	if in.AddressLine1 != nil {
		v.AddressLine1 = in.AddressLine1
	}
	if in.AddressLine2 != nil {
		v.AddressLine2 = in.AddressLine2
	}
	if in.City != nil {
		v.City = in.City
	}
	if in.Region != nil {
		v.Region = in.Region
	}
	if in.Country != nil {
		upper := strings.ToUpper(strings.TrimSpace(*in.Country))
		v.Country = &upper
	}
	if in.PostalCode != nil {
		v.PostalCode = in.PostalCode
	}
	if in.Timezone != nil && strings.TrimSpace(*in.Timezone) != "" {
		v.Timezone = strings.TrimSpace(*in.Timezone)
	}
	if in.MaxGuests != nil {
		v.MaxGuests = in.MaxGuests
	}
	if in.BasePriceMinor != nil {
		v.BasePriceMinor = in.BasePriceMinor
	}
	if in.CleaningFeeMinor != nil {
		v.CleaningFeeMinor = *in.CleaningFeeMinor
	}
	if in.Currency != nil && strings.TrimSpace(*in.Currency) != "" {
		v.Currency = strings.ToUpper(strings.TrimSpace(*in.Currency))
	}
	if in.InstantBook != nil {
		v.InstantBook = *in.InstantBook
	}
	if in.CheckInTime != nil && strings.TrimSpace(*in.CheckInTime) != "" {
		v.CheckInTime = strings.TrimSpace(*in.CheckInTime)
	}
	if in.CheckOutTime != nil && strings.TrimSpace(*in.CheckOutTime) != "" {
		v.CheckOutTime = strings.TrimSpace(*in.CheckOutTime)
	}
	if in.HouseRules != nil {
		v.HouseRules = in.HouseRules
	}
	if in.CancellationPolicy != nil {
		v.CancellationPolicy = in.CancellationPolicy
	}
	if in.Amenities != nil {
		v.Amenities = *in.Amenities
	}
	if in.MediaIDs != nil {
		v.MediaIDs = *in.MediaIDs
	}
}
