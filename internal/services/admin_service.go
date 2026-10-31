package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/clock"
	"golang.org/x/crypto/bcrypt"
)

type AdminService struct {
	adminRepo interfaces.AdminRepository
	audit     interfaces.AuditService
	clock     clock.Clock
}

func NewAdminService(
	adminRepo interfaces.AdminRepository,
	audit interfaces.AuditService,
	clk clock.Clock,
) interfaces.AdminService {
	return &AdminService{
		adminRepo: adminRepo,
		audit:     audit,
		clock:     clk,
	}
}

func (s *AdminService) ListAdmins(ctx context.Context) ([]dtos.AdminDetailResponse, error) {
	admins, err := s.adminRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]dtos.AdminDetailResponse, 0, len(admins))
	for _, a := range admins {
		perms, _ := s.adminRepo.GetRolePermissions(ctx, a.Role)
		overrides, _ := s.adminRepo.GetPermissionsByAdmin(ctx, a.ID)
		if perms == nil {
			perms = []string{}
		}
		if overrides == nil {
			overrides = []models.AdminPermission{}
		}

		out = append(out, dtos.AdminDetailResponse{
			ID:          a.ID,
			Email:       a.Email,
			FullName:    a.FullName,
			Role:        a.Role,
			Permissions: perms,
			Overrides:   overrides,
			CreatedAt:   a.CreatedAt,
			UpdatedAt:   a.UpdatedAt,
		})
	}

	return out, nil
}

func (s *AdminService) CreateAdmin(
	ctx context.Context,
	actor models.AuthActor,
	in dtos.CreateAdminInput,
) (*dtos.AdminDetailResponse, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if email == "" || in.Password == "" || strings.TrimSpace(in.FullName) == "" {
		return nil, fmt.Errorf("%w: email, password, and full_name are required", apperrors.ErrValidation)
	}

	role := in.Role
	if role == "" {
		role = enums.AdminRoleSupportL1
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	now := s.clock.Now()
	newAdmin := models.AdminUser{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: string(hash),
		FullName:     strings.TrimSpace(in.FullName),
		Role:         role,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	created, err := s.adminRepo.Create(ctx, newAdmin)
	if err != nil {
		return nil, err
	}

	if s.audit != nil {
		_ = s.audit.Record(ctx, dtos.AuditEntryInput{
			ActorType:  enums.ActorAdmin,
			ActorID:    &actor.SubjectID,
			Action:     "admin.create",
			ObjectType: "admin_user",
			ObjectID:   created.ID,
			AfterState: map[string]any{
				"email":     created.Email,
				"full_name": created.FullName,
				"role":      created.Role,
			},
		})
	}

	perms, _ := s.adminRepo.GetRolePermissions(ctx, created.Role)
	return &dtos.AdminDetailResponse{
		ID:          created.ID,
		Email:       created.Email,
		FullName:    created.FullName,
		Role:        created.Role,
		Permissions: perms,
		Overrides:   []models.AdminPermission{},
		CreatedAt:   created.CreatedAt,
		UpdatedAt:   created.UpdatedAt,
	}, nil
}

func (s *AdminService) UpdateAdmin(
	ctx context.Context,
	actor models.AuthActor,
	id uuid.UUID,
	in dtos.UpdateAdminInput,
) (*dtos.AdminDetailResponse, error) {
	admin, err := s.adminRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	beforeRole := admin.Role
	now := s.clock.Now()
	if in.Role != nil {
		admin.Role = *in.Role
		admin.UpdatedAt = now
		updated, err := s.adminRepo.Update(ctx, *admin)
		if err != nil {
			return nil, err
		}
		admin = updated
	}

	if len(in.Permissions) > 0 {
		for perm, granted := range in.Permissions {
			if err := s.adminRepo.SetPermission(ctx, admin.ID, perm, granted); err != nil {
				return nil, err
			}
		}
	}

	if s.audit != nil {
		_ = s.audit.Record(ctx, dtos.AuditEntryInput{
			ActorType:   enums.ActorAdmin,
			ActorID:     &actor.SubjectID,
			Action:      "admin.update_role",
			ObjectType:  "admin_user",
			ObjectID:    admin.ID,
			BeforeState: map[string]any{"role": beforeRole},
			AfterState: map[string]any{
				"role":        admin.Role,
				"permissions": in.Permissions,
			},
		})
	}

	perms, _ := s.adminRepo.GetRolePermissions(ctx, admin.Role)
	overrides, _ := s.adminRepo.GetPermissionsByAdmin(ctx, admin.ID)

	return &dtos.AdminDetailResponse{
		ID:          admin.ID,
		Email:       admin.Email,
		FullName:    admin.FullName,
		Role:        admin.Role,
		Permissions: perms,
		Overrides:   overrides,
		CreatedAt:   admin.CreatedAt,
		UpdatedAt:   admin.UpdatedAt,
	}, nil
}

func (s *AdminService) HasPermission(ctx context.Context, adminID uuid.UUID, permission string) (bool, error) {
	admin, err := s.adminRepo.FindByID(ctx, adminID)
	if err != nil {
		return false, err
	}
	return s.adminRepo.HasPermission(ctx, adminID, admin.Role, permission)
}
