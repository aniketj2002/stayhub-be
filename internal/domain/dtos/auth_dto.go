package dtos

import (
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/models"
)

type RegisterUserInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
	IsHost   bool   `json:"is_host"`
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshTokenInput struct {
	RefreshToken string `json:"refresh_token"`
}

type UpdateProfileInput struct {
	FullName      *string    `json:"full_name,omitempty"`
	AvatarMediaID *uuid.UUID `json:"avatar_media_id,omitempty"`
}

type CreateUserParams struct {
	Email        string
	PasswordHash string
	FullName     string
	IsGuest      bool
	IsHost       bool
}

type CreateRefreshTokenParams struct {
	SubjectType string
	SubjectID   uuid.UUID
	TokenHash   string
	ExpiresAt   time.Time
}

type UserResponse struct {
	ID            uuid.UUID  `json:"id"`
	Email         string     `json:"email"`
	FullName      string     `json:"full_name"`
	AvatarMediaID *uuid.UUID `json:"avatar_media_id,omitempty"`
	AvatarURL     *string    `json:"avatar_url,omitempty"`
	IsGuest       bool       `json:"is_guest"`
	IsHost        bool       `json:"is_host"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func ToUserResponse(u models.User, avatarURL *string) UserResponse {
	return UserResponse{
		ID:            u.ID,
		Email:         u.Email,
		FullName:      u.FullName,
		AvatarMediaID: u.AvatarMediaID,
		AvatarURL:     avatarURL,
		IsGuest:       u.IsGuest,
		IsHost:        u.IsHost,
		CreatedAt:     u.CreatedAt,
		UpdatedAt:     u.UpdatedAt,
	}
}

type AdminUserResponse struct {
	ID        uuid.UUID       `json:"id"`
	Email     string          `json:"email"`
	FullName  string          `json:"full_name"`
	Role      enums.AdminRole `json:"role"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

func ToAdminUserResponse(a models.AdminUser) AdminUserResponse {
	return AdminUserResponse{
		ID:        a.ID,
		Email:     a.Email,
		FullName:  a.FullName,
		Role:      a.Role,
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	}
}

type AuthTokenResponse struct {
	SubjectType  string             `json:"subject_type"`
	User         *UserResponse      `json:"user,omitempty"`
	Admin        *AdminUserResponse `json:"admin,omitempty"`
	AccessToken  string             `json:"access_token"`
	RefreshToken string             `json:"refresh_token"`
	ExpiresIn    int64              `json:"expires_in"`
}

type BecomeHostResponse struct {
	User        UserResponse `json:"user"`
	AccessToken string       `json:"access_token"`
	ExpiresIn   int64        `json:"expires_in"`
}

type MeResponse struct {
	SubjectType string             `json:"subject_type"`
	User        *UserResponse      `json:"user,omitempty"`
	Admin       *AdminUserResponse `json:"admin,omitempty"`
}
