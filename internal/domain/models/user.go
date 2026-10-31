package models

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/enums"
)

type authActorCtxKey struct{}

// AuthActor represents the authenticated principal in request context.
type AuthActor struct {
	SubjectID   uuid.UUID
	SubjectType string // "user" | "admin"
	Email       string
	IsGuest     bool
	IsHost      bool
}

func (a AuthActor) DefaultActorType() enums.ActorType {
	if a.SubjectType == "admin" {
		return enums.ActorAdmin
	}
	if a.IsHost {
		return enums.ActorHost
	}
	return enums.ActorGuest
}

func WithAuthActor(ctx context.Context, actor AuthActor) context.Context {
	return context.WithValue(ctx, authActorCtxKey{}, actor)
}

func AuthActorFromContext(ctx context.Context) (AuthActor, bool) {
	v, ok := ctx.Value(authActorCtxKey{}).(AuthActor)
	return v, ok
}

// User maps to the `users` table (guests and hosts; one account can be both).
type User struct {
	ID            uuid.UUID  `db:"id" json:"id"`
	Email         string     `db:"email" json:"email"`
	PasswordHash  string     `db:"password_hash" json:"-"`
	FullName      string     `db:"full_name" json:"full_name"`
	AvatarMediaID *uuid.UUID `db:"avatar_media_id" json:"avatar_media_id,omitempty"`
	IsGuest       bool       `db:"is_guest" json:"is_guest"`
	IsHost        bool       `db:"is_host" json:"is_host"`
	IsSuperhost   bool       `db:"is_superhost" json:"is_superhost"`
	Bio           *string    `db:"bio" json:"bio,omitempty"`
	Languages     []string   `db:"languages" json:"languages,omitempty"`
	HostingSince  *time.Time `db:"hosting_since" json:"hosting_since,omitempty"`
	Phone         *string    `db:"phone" json:"phone,omitempty"`
	Status        string     `db:"status" json:"status"`
	CreatedAt     time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at" json:"updated_at"`
}

// AdminUser maps to the `admin_users` table.
type AdminUser struct {
	ID           uuid.UUID       `db:"id" json:"id"`
	Email        string          `db:"email" json:"email"`
	PasswordHash string          `db:"password_hash" json:"-"`
	FullName     string          `db:"full_name" json:"full_name"`
	Role         enums.AdminRole `db:"role" json:"role"`
	CreatedAt    time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at" json:"updated_at"`
}

// RefreshToken maps to the `refresh_tokens` table.
type RefreshToken struct {
	ID          uuid.UUID  `db:"id" json:"id"`
	SubjectType string     `db:"subject_type" json:"subject_type"` // 'user' | 'admin'
	SubjectID   uuid.UUID  `db:"subject_id" json:"subject_id"`
	TokenHash   string     `db:"token_hash" json:"-"`
	ExpiresAt   time.Time  `db:"expires_at" json:"expires_at"`
	RevokedAt   *time.Time `db:"revoked_at" json:"revoked_at,omitempty"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
}
