package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/models"
)

type CohostRepository interface {
	CreateInvitation(ctx context.Context, inv models.CohostInvitation) (*models.CohostInvitation, error)
	FindInvitationByTokenHash(ctx context.Context, tokenHash string) (*models.CohostInvitation, error)
	ListInvitationsByListing(ctx context.Context, listingID uuid.UUID) ([]models.CohostInvitation, error)
	UpdateInvitationStatus(ctx context.Context, id uuid.UUID, status enums.CohostInviteStatus, now time.Time) error
	FindInvitationByID(ctx context.Context, id uuid.UUID) (*models.CohostInvitation, error)

	CreatePayoutSplits(ctx context.Context, splits []models.PayoutSplit) error
	ListPayoutSplits(ctx context.Context, payoutID uuid.UUID) ([]models.PayoutSplit, error)
	UpdatePayoutSplitsStatus(ctx context.Context, payoutID uuid.UUID, status enums.PayoutSplitStatus, now time.Time) error
}

type CohostService interface {
	InviteCohost(ctx context.Context, inviterID, listingID uuid.UUID, in dtos.CreateCohostInviteInput) (*dtos.CohostInviteResponse, string, error)
	ListInvitations(ctx context.Context, listingID uuid.UUID) ([]dtos.CohostInviteResponse, error)
	CancelInvitation(ctx context.Context, listingID, inviteID uuid.UUID) error
	GetInvitationDetails(ctx context.Context, rawToken string) (*dtos.CohostInvitationDetailsResponse, error)
	AcceptInvitation(ctx context.Context, user models.User, rawToken string) (*dtos.AcceptCohostInviteResponse, error)
	ListCohosts(ctx context.Context, listingID uuid.UUID) ([]dtos.CohostMemberResponse, error)
	UpdateCohost(ctx context.Context, listingID, targetUserID uuid.UUID, in dtos.UpdateCohostInput) (*dtos.CohostMemberResponse, error)
	RemoveCohost(ctx context.Context, listingID, targetUserID uuid.UUID) error
}
