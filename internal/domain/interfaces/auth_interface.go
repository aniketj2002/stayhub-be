package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/models"
)

type UserRepository interface {
	Create(ctx context.Context, p dtos.CreateUserParams) (*models.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	EnableHostRole(ctx context.Context, id uuid.UUID, now time.Time) (*models.User, error)
	UpdateProfile(ctx context.Context, id uuid.UUID, fullName string, avatarMediaID *uuid.UUID, now time.Time) (*models.User, error)
}

type AdminUserRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*models.AdminUser, error)
	FindByEmail(ctx context.Context, email string) (*models.AdminUser, error)
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, p dtos.CreateRefreshTokenParams) (*models.RefreshToken, error)
	FindByHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error)
	RevokeByHash(ctx context.Context, tokenHash string, now time.Time) error
	RevokeByID(ctx context.Context, id uuid.UUID, now time.Time) error
}

type AuthService interface {
	Register(ctx context.Context, in dtos.RegisterUserInput) (*dtos.AuthTokenResponse, error)
	LoginUser(ctx context.Context, in dtos.LoginInput) (*dtos.AuthTokenResponse, error)
	LoginAdmin(ctx context.Context, in dtos.LoginInput) (*dtos.AuthTokenResponse, error)
	Refresh(ctx context.Context, in dtos.RefreshTokenInput) (*dtos.AuthTokenResponse, error)
	Logout(ctx context.Context, in dtos.RefreshTokenInput) error
	GetMe(ctx context.Context, actor models.AuthActor) (*dtos.MeResponse, error)
	BecomeHost(ctx context.Context, userID uuid.UUID) (*dtos.BecomeHostResponse, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, in dtos.UpdateProfileInput) (*dtos.UserResponse, error)
}
