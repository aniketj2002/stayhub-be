package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	platformaws "github.com/stayhub/backend/internal/platform/aws"
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/db"
)

type CohostService struct {
	db          *db.DB
	cohostRepo  interfaces.CohostRepository
	hostRepo    interfaces.ListingHostRepository
	userRepo    interfaces.UserRepository
	listingRepo interfaces.ListingRepository
	clock       clock.Clock
	emailSender platformaws.EmailSender
	appBaseURL  string
}

func NewCohostService(
	database *db.DB,
	cohostRepo interfaces.CohostRepository,
	hostRepo interfaces.ListingHostRepository,
	userRepo interfaces.UserRepository,
	listingRepo interfaces.ListingRepository,
	clk clock.Clock,
	emailSender ...platformaws.EmailSender,
) interfaces.CohostService {
	var sender platformaws.EmailSender
	if len(emailSender) > 0 {
		sender = emailSender[0]
	}
	return &CohostService{
		db:          database,
		cohostRepo:  cohostRepo,
		hostRepo:    hostRepo,
		userRepo:    userRepo,
		listingRepo: listingRepo,
		clock:       clk,
		emailSender: sender,
		appBaseURL:  "http://localhost:5173",
	}
}

func (s *CohostService) SetEmailConfig(emailSender platformaws.EmailSender, appBaseURL string) {
	s.emailSender = emailSender
	if strings.TrimSpace(appBaseURL) != "" {
		s.appBaseURL = strings.TrimRight(strings.TrimSpace(appBaseURL), "/")
	}
}

func (s *CohostService) InviteCohost(
	ctx context.Context,
	inviterID, listingID uuid.UUID,
	in dtos.CreateCohostInviteInput,
) (*dtos.CohostInviteResponse, string, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if email == "" || !strings.Contains(email, "@") {
		return nil, "", fmt.Errorf("%w: valid email is required", apperrors.ErrValidation)
	}
	if in.PayoutShareBPS < 0 || in.PayoutShareBPS > 10000 {
		return nil, "", fmt.Errorf("%w: payout_share_bps must be between 0 and 10000", apperrors.ErrValidation)
	}

	// Verify total allocated payout shares + this invite does not exceed 10000
	members, err := s.hostRepo.ListByListing(ctx, listingID)
	if err != nil {
		return nil, "", err
	}
	allocatedShares := 0
	for _, m := range members {
		allocatedShares += m.PayoutShareBPS
	}
	if allocatedShares+in.PayoutShareBPS > 10000 {
		return nil, "", fmt.Errorf("%w: total payout share exceeds 100%% (current allocated: %d bps, requested: %d bps)",
			apperrors.ErrValidation, allocatedShares, in.PayoutShareBPS)
	}

	// Generate random token and hash
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, "", fmt.Errorf("generate token: %w", err)
	}
	rawToken := hex.EncodeToString(tokenBytes)
	hashBytes := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hashBytes[:])

	permsBytes, _ := json.Marshal(in.Permissions)
	now := s.clock.Now()
	expiresAt := now.Add(7 * 24 * time.Hour)

	inv, err := s.cohostRepo.CreateInvitation(ctx, models.CohostInvitation{
		ListingID:      listingID,
		InviterUserID:  inviterID,
		Email:          email,
		TokenHash:      tokenHash,
		Permissions:    json.RawMessage(permsBytes),
		PayoutShareBPS: in.PayoutShareBPS,
		Status:         enums.CohostInvitePending,
		ExpiresAt:      expiresAt,
	})
	if err != nil {
		return nil, "", err
	}

	if s.emailSender != nil {
		listingTitle := s.resolveListingTitle(ctx, listingID)
		hostName := s.resolveHostName(ctx, inviterID)
		baseURL := s.appBaseURL
		if baseURL == "" {
			baseURL = "http://localhost:5173"
		}
		inviteURL := fmt.Sprintf("%s/cohost-invitations/%s", strings.TrimRight(baseURL, "/"), rawToken)

		if emailErr := s.emailSender.SendCohostInvitation(ctx, platformaws.CohostInvitationEmailInput{
			RecipientEmail: inv.Email,
			HostName:       hostName,
			ListingTitle:   listingTitle,
			InviteURL:      inviteURL,
			PayoutShareBPS: inv.PayoutShareBPS,
			Permissions:    formatPermissionLabels(in.Permissions),
			ExpiresAt:      inv.ExpiresAt,
		}); emailErr != nil {
			log.Printf("warning: failed to send cohost invitation email via SES (from %s to %s): %v",
				s.emailSender.SenderEmail(), inv.Email, emailErr)
		}
	}

	return &dtos.CohostInviteResponse{
		ID:             inv.ID,
		ListingID:      inv.ListingID,
		Email:          inv.Email,
		Permissions:    in.Permissions,
		PayoutShareBPS: inv.PayoutShareBPS,
		Status:         string(inv.Status),
		ExpiresAt:      inv.ExpiresAt,
		CreatedAt:      inv.CreatedAt,
	}, rawToken, nil
}

func (s *CohostService) resolveListingTitle(ctx context.Context, listingID uuid.UUID) string {
	listingTitle := "StayHub Listing"
	if s.listingRepo != nil {
		if l, err := s.listingRepo.FindByID(ctx, listingID); err == nil && l != nil {
			var verID *uuid.UUID
			if l.LiveVersionID != nil {
				verID = l.LiveVersionID
			} else if l.WorkingVersionID != nil {
				verID = l.WorkingVersionID
			}
			if verID != nil {
				if ver, err := s.listingRepo.FindVersionByID(ctx, *verID); err == nil && ver != nil && ver.Title != nil && *ver.Title != "" {
					listingTitle = *ver.Title
				}
			}
		}
	}
	return listingTitle
}

func (s *CohostService) resolveHostName(ctx context.Context, inviterID uuid.UUID) string {
	hostName := "Host"
	if s.userRepo != nil {
		if hostUser, err := s.userRepo.FindByID(ctx, inviterID); err == nil && hostUser != nil && hostUser.FullName != "" {
			hostName = hostUser.FullName
		}
	}
	return hostName
}

func formatPermissionLabels(p dtos.CohostPermissions) []string {
	var labels []string
	if p.CanManageListing {
		labels = append(labels, "Manage listing details")
	}
	if p.CanManageCalendar {
		labels = append(labels, "Manage calendar & pricing")
	}
	if p.CanManageReservations {
		labels = append(labels, "Manage reservations")
	}
	if p.CanMessageGuests {
		labels = append(labels, "Message guests")
	}
	if p.CanViewInsights {
		labels = append(labels, "View performance & insights")
	}
	return labels
}

func (s *CohostService) ListInvitations(ctx context.Context, listingID uuid.UUID) ([]dtos.CohostInviteResponse, error) {
	invs, err := s.cohostRepo.ListInvitationsByListing(ctx, listingID)
	if err != nil {
		return nil, err
	}

	out := make([]dtos.CohostInviteResponse, 0, len(invs))
	for _, inv := range invs {
		var perms dtos.CohostPermissions
		_ = json.Unmarshal(inv.Permissions, &perms)
		out = append(out, dtos.CohostInviteResponse{
			ID:             inv.ID,
			ListingID:      inv.ListingID,
			Email:          inv.Email,
			Permissions:    perms,
			PayoutShareBPS: inv.PayoutShareBPS,
			Status:         string(inv.Status),
			ExpiresAt:      inv.ExpiresAt,
			CreatedAt:      inv.CreatedAt,
		})
	}
	return out, nil
}

func (s *CohostService) CancelInvitation(ctx context.Context, listingID, inviteID uuid.UUID) error {
	inv, err := s.cohostRepo.FindInvitationByID(ctx, inviteID)
	if err != nil {
		return err
	}
	if inv.ListingID != listingID {
		return apperrors.ErrForbidden
	}
	if inv.Status != enums.CohostInvitePending {
		return fmt.Errorf("%w: invitation is not pending", apperrors.ErrValidation)
	}

	return s.cohostRepo.UpdateInvitationStatus(ctx, inviteID, enums.CohostInviteCancelled, s.clock.Now())
}

func (s *CohostService) GetInvitationDetails(ctx context.Context, rawToken string) (*dtos.CohostInvitationDetailsResponse, error) {
	hashBytes := sha256.Sum256([]byte(strings.TrimSpace(rawToken)))
	tokenHash := hex.EncodeToString(hashBytes[:])

	inv, err := s.cohostRepo.FindInvitationByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, err
	}

	listingTitle := "StayHub Listing"
	if s.listingRepo != nil {
		if l, err := s.listingRepo.FindByID(ctx, inv.ListingID); err == nil && l != nil {
			var verID *uuid.UUID
			if l.LiveVersionID != nil {
				verID = l.LiveVersionID
			} else if l.WorkingVersionID != nil {
				verID = l.WorkingVersionID
			}
			if verID != nil {
				if ver, err := s.listingRepo.FindVersionByID(ctx, *verID); err == nil && ver != nil && ver.Title != nil {
					listingTitle = *ver.Title
				}
			}
		}
	}

	hostName := "Host"
	if hostUser, err := s.userRepo.FindByID(ctx, inv.InviterUserID); err == nil && hostUser != nil {
		hostName = hostUser.FullName
	}

	var perms dtos.CohostPermissions
	_ = json.Unmarshal(inv.Permissions, &perms)

	status := string(inv.Status)
	if inv.Status == enums.CohostInvitePending && inv.ExpiresAt.Before(s.clock.Now()) {
		status = string(enums.CohostInviteExpired)
	}

	return &dtos.CohostInvitationDetailsResponse{
		ID:             inv.ID,
		ListingID:      inv.ListingID,
		ListingTitle:   listingTitle,
		HostName:       hostName,
		InvitedEmail:   inv.Email,
		Permissions:    perms,
		PayoutShareBPS: inv.PayoutShareBPS,
		Status:         status,
		ExpiresAt:      inv.ExpiresAt,
	}, nil
}

func (s *CohostService) AcceptInvitation(ctx context.Context, user models.User, rawToken string) (*dtos.AcceptCohostInviteResponse, error) {
	hashBytes := sha256.Sum256([]byte(strings.TrimSpace(rawToken)))
	tokenHash := hex.EncodeToString(hashBytes[:])

	inv, err := s.cohostRepo.FindInvitationByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, err
	}

	now := s.clock.Now()
	if inv.Status != enums.CohostInvitePending {
		return nil, fmt.Errorf("%w: invitation is %s", apperrors.ErrValidation, inv.Status)
	}
	if inv.ExpiresAt.Before(now) {
		_ = s.cohostRepo.UpdateInvitationStatus(ctx, inv.ID, enums.CohostInviteExpired, now)
		return nil, fmt.Errorf("%w: invitation has expired", apperrors.ErrValidation)
	}
	if !strings.EqualFold(inv.Email, user.Email) {
		return nil, fmt.Errorf("%w: logged-in user email (%s) does not match invitation email (%s)",
			apperrors.ErrForbidden, user.Email, inv.Email)
	}

	var perms dtos.CohostPermissions
	_ = json.Unmarshal(inv.Permissions, &perms)

	err = s.db.WithinTx(ctx, func(txCtx context.Context) error {
		// Update invitation status
		if err := s.cohostRepo.UpdateInvitationStatus(txCtx, inv.ID, enums.CohostInviteAccepted, now); err != nil {
			return err
		}

		// Enable host role on user if not already enabled
		if !user.IsHost {
			if _, err := s.userRepo.EnableHostRole(txCtx, user.ID, now); err != nil {
				return err
			}
		}

		// Insert cohost in listing_hosts
		_, err := s.hostRepo.CreateCohost(txCtx, models.ListingHost{
			ListingID:             inv.ListingID,
			UserID:                user.ID,
			Role:                  enums.ListingHostRoleCohost,
			CanManageListing:      perms.CanManageListing,
			CanManageCalendar:     perms.CanManageCalendar,
			CanManageReservations: perms.CanManageReservations,
			CanMessageGuests:      perms.CanMessageGuests,
			CanViewInsights:       perms.CanViewInsights,
			PayoutShareBPS:        inv.PayoutShareBPS,
			InvitedBy:             &inv.InviterUserID,
		})
		return err
	})
	if err != nil {
		return nil, err
	}

	return &dtos.AcceptCohostInviteResponse{
		ListingID: inv.ListingID,
		Status:    "accepted",
	}, nil
}

func (s *CohostService) ListCohosts(ctx context.Context, listingID uuid.UUID) ([]dtos.CohostMemberResponse, error) {
	members, err := s.hostRepo.ListByListing(ctx, listingID)
	if err != nil {
		return nil, err
	}

	out := make([]dtos.CohostMemberResponse, 0, len(members))
	for _, m := range members {
		var fullName, email string
		if u, err := s.userRepo.FindByID(ctx, m.UserID); err == nil && u != nil {
			fullName = u.FullName
			email = u.Email
		}
		out = append(out, dtos.CohostMemberResponse{
			UserID:   m.UserID,
			FullName: fullName,
			Email:    email,
			Role:     string(m.Role),
			Permissions: dtos.CohostPermissions{
				CanManageListing:      m.CanManageListing,
				CanManageCalendar:     m.CanManageCalendar,
				CanManageReservations: m.CanManageReservations,
				CanMessageGuests:      m.CanMessageGuests,
				CanViewInsights:       m.CanViewInsights,
			},
			PayoutShareBPS: m.PayoutShareBPS,
			CreatedAt:      m.CreatedAt,
		})
	}
	return out, nil
}

func (s *CohostService) UpdateCohost(ctx context.Context, listingID, targetUserID uuid.UUID, in dtos.UpdateCohostInput) (*dtos.CohostMemberResponse, error) {
	member, err := s.hostRepo.FindByListingAndUser(ctx, listingID, targetUserID)
	if err != nil {
		return nil, err
	}
	if member.Role == enums.ListingHostRoleOwner {
		return nil, fmt.Errorf("%w: cannot modify owner through co-host management", apperrors.ErrForbidden)
	}

	if in.PayoutShareBPS != nil {
		newShare := *in.PayoutShareBPS
		if newShare < 0 || newShare > 10000 {
			return nil, fmt.Errorf("%w: payout_share_bps must be between 0 and 10000", apperrors.ErrValidation)
		}

		allMembers, err := s.hostRepo.ListByListing(ctx, listingID)
		if err != nil {
			return nil, err
		}
		allocatedShares := 0
		for _, m := range allMembers {
			if m.UserID != targetUserID {
				allocatedShares += m.PayoutShareBPS
			}
		}
		if allocatedShares+newShare > 10000 {
			return nil, fmt.Errorf("%w: total payout share exceeds 100%%", apperrors.ErrValidation)
		}
		member.PayoutShareBPS = newShare
	}

	if in.Permissions != nil {
		member.CanManageListing = in.Permissions.CanManageListing
		member.CanManageCalendar = in.Permissions.CanManageCalendar
		member.CanManageReservations = in.Permissions.CanManageReservations
		member.CanMessageGuests = in.Permissions.CanMessageGuests
		member.CanViewInsights = in.Permissions.CanViewInsights
	}

	updated, err := s.hostRepo.UpdateCohost(ctx, *member)
	if err != nil {
		return nil, err
	}

	var fullName, email string
	if u, err := s.userRepo.FindByID(ctx, updated.UserID); err == nil && u != nil {
		fullName = u.FullName
		email = u.Email
	}

	return &dtos.CohostMemberResponse{
		UserID:   updated.UserID,
		FullName: fullName,
		Email:    email,
		Role:     string(updated.Role),
		Permissions: dtos.CohostPermissions{
			CanManageListing:      updated.CanManageListing,
			CanManageCalendar:     updated.CanManageCalendar,
			CanManageReservations: updated.CanManageReservations,
			CanMessageGuests:      updated.CanMessageGuests,
			CanViewInsights:       updated.CanViewInsights,
		},
		PayoutShareBPS: updated.PayoutShareBPS,
		CreatedAt:      updated.CreatedAt,
	}, nil
}

func (s *CohostService) RemoveCohost(ctx context.Context, listingID, targetUserID uuid.UUID) error {
	member, err := s.hostRepo.FindByListingAndUser(ctx, listingID, targetUserID)
	if err != nil {
		return err
	}
	if member.Role == enums.ListingHostRoleOwner {
		return fmt.Errorf("%w: cannot remove listing owner", apperrors.ErrForbidden)
	}

	return s.hostRepo.DeleteCohost(ctx, listingID, targetUserID)
}
