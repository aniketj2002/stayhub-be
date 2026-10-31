package interfaces

import (
	"context"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/models"
)

type AdminRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*models.AdminUser, error)
	FindByEmail(ctx context.Context, email string) (*models.AdminUser, error)
	List(ctx context.Context) ([]models.AdminUser, error)
	Create(ctx context.Context, admin models.AdminUser) (*models.AdminUser, error)
	Update(ctx context.Context, admin models.AdminUser) (*models.AdminUser, error)
	GetRolePermissions(ctx context.Context, role enums.AdminRole) ([]string, error)
	GetPermissionsByAdmin(ctx context.Context, adminID uuid.UUID) ([]models.AdminPermission, error)
	SetPermission(ctx context.Context, adminID uuid.UUID, permission string, granted bool) error
	HasPermission(ctx context.Context, adminID uuid.UUID, role enums.AdminRole, permission string) (bool, error)
}

type AdminService interface {
	ListAdmins(ctx context.Context) ([]dtos.AdminDetailResponse, error)
	CreateAdmin(ctx context.Context, actor models.AuthActor, in dtos.CreateAdminInput) (*dtos.AdminDetailResponse, error)
	UpdateAdmin(ctx context.Context, actor models.AuthActor, id uuid.UUID, in dtos.UpdateAdminInput) (*dtos.AdminDetailResponse, error)
	HasPermission(ctx context.Context, adminID uuid.UUID, permission string) (bool, error)
}

type AdminReservationService interface {
	ListReservations(ctx context.Context, filter dtos.AdminReservationFilter) (*dtos.AdminReservationListResponse, error)
	GetReservationDetail(ctx context.Context, id uuid.UUID) (*dtos.AdminReservationDetailResponse, error)
	CancelReservation(ctx context.Context, actor models.AuthActor, id uuid.UUID, in dtos.AdminCancelReservationInput) (*dtos.AdminReservationDetailResponse, error)
	RefundReservation(ctx context.Context, actor models.AuthActor, id uuid.UUID, in dtos.AdminRefundReservationInput) (*dtos.AdminReservationDetailResponse, error)
}

type AdminListingReviewService interface {
	ListQueue(ctx context.Context, filter dtos.AdminListingReviewFilter) ([]dtos.AdminListingReviewQueueItem, error)
	GetDiff(ctx context.Context, versionID uuid.UUID) (*dtos.AdminListingDiffResponse, error)
	Approve(ctx context.Context, actor models.AuthActor, versionID uuid.UUID, note string) error
	RequestChanges(ctx context.Context, actor models.AuthActor, versionID uuid.UUID, note string) error
	Reject(ctx context.Context, actor models.AuthActor, versionID uuid.UUID, note string) error
}
