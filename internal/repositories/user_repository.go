package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/db"
)

type UserRepository struct {
	db *db.DB
}

func NewUserRepository(database *db.DB) interfaces.UserRepository {
	return &UserRepository{db: database}
}

func (r *UserRepository) Create(ctx context.Context, p dtos.CreateUserParams) (*models.User, error) {
	const q = `
		INSERT INTO users (email, password_hash, full_name, is_guest, is_host)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, email, password_hash, full_name, avatar_media_id, is_guest, is_host, created_at, updated_at
	`
	var u models.User
	err := r.db.Conn(ctx).QueryRow(
		ctx,
		q,
		p.Email,
		p.PasswordHash,
		p.FullName,
		p.IsGuest,
		p.IsHost,
	).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.FullName,
		&u.AvatarMediaID,
		&u.IsGuest,
		&u.IsHost,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, apperrors.ErrEmailAlreadyExists
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	const q = `
		SELECT id, email, password_hash, full_name, avatar_media_id, is_guest, is_host, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	var u models.User
	err := r.db.Conn(ctx).QueryRow(ctx, q, id).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.FullName,
		&u.AvatarMediaID,
		&u.IsGuest,
		&u.IsHost,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	const q = `
		SELECT id, email, password_hash, full_name, avatar_media_id, is_guest, is_host, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	var u models.User
	err := r.db.Conn(ctx).QueryRow(ctx, q, email).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.FullName,
		&u.AvatarMediaID,
		&u.IsGuest,
		&u.IsHost,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("find user by email: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) EnableHostRole(ctx context.Context, id uuid.UUID, now time.Time) (*models.User, error) {
	const q = `
		UPDATE users
		SET is_host = true, updated_at = $2
		WHERE id = $1
		RETURNING id, email, password_hash, full_name, avatar_media_id, is_guest, is_host, created_at, updated_at
	`
	var u models.User
	err := r.db.Conn(ctx).QueryRow(ctx, q, id, now).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.FullName,
		&u.AvatarMediaID,
		&u.IsGuest,
		&u.IsHost,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("enable host role: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) UpdateProfile(ctx context.Context, id uuid.UUID, fullName string, avatarMediaID *uuid.UUID, now time.Time) (*models.User, error) {
	const q = `
		UPDATE users
		SET full_name = $2, avatar_media_id = $3, updated_at = $4
		WHERE id = $1
		RETURNING id, email, password_hash, full_name, avatar_media_id, is_guest, is_host, created_at, updated_at
	`
	var u models.User
	err := r.db.Conn(ctx).QueryRow(ctx, q, id, fullName, avatarMediaID, now).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.FullName,
		&u.AvatarMediaID,
		&u.IsGuest,
		&u.IsHost,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("update user profile: %w", err)
	}
	return &u, nil
}

type AdminUserRepository struct {
	db *db.DB
}

func NewAdminUserRepository(database *db.DB) interfaces.AdminUserRepository {
	return &AdminUserRepository{db: database}
}

func (r *AdminUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.AdminUser, error) {
	const q = `
		SELECT id, email, password_hash, full_name, created_at, updated_at
		FROM admin_users
		WHERE id = $1
	`
	var a models.AdminUser
	err := r.db.Conn(ctx).QueryRow(ctx, q, id).Scan(
		&a.ID,
		&a.Email,
		&a.PasswordHash,
		&a.FullName,
		&a.CreatedAt,
		&a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("find admin by id: %w", err)
	}
	return &a, nil
}

func (r *AdminUserRepository) FindByEmail(ctx context.Context, email string) (*models.AdminUser, error) {
	const q = `
		SELECT id, email, password_hash, full_name, created_at, updated_at
		FROM admin_users
		WHERE email = $1
	`
	var a models.AdminUser
	err := r.db.Conn(ctx).QueryRow(ctx, q, email).Scan(
		&a.ID,
		&a.Email,
		&a.PasswordHash,
		&a.FullName,
		&a.CreatedAt,
		&a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("find admin by email: %w", err)
	}
	return &a, nil
}

type RefreshTokenRepository struct {
	db *db.DB
}

func NewRefreshTokenRepository(database *db.DB) interfaces.RefreshTokenRepository {
	return &RefreshTokenRepository{db: database}
}

func (r *RefreshTokenRepository) Create(ctx context.Context, p dtos.CreateRefreshTokenParams) (*models.RefreshToken, error) {
	const q = `
		INSERT INTO refresh_tokens (subject_type, subject_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, subject_type, subject_id, token_hash, expires_at, revoked_at, created_at
	`
	var rt models.RefreshToken
	err := r.db.Conn(ctx).QueryRow(
		ctx,
		q,
		p.SubjectType,
		p.SubjectID,
		p.TokenHash,
		p.ExpiresAt,
	).Scan(
		&rt.ID,
		&rt.SubjectType,
		&rt.SubjectID,
		&rt.TokenHash,
		&rt.ExpiresAt,
		&rt.RevokedAt,
		&rt.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create refresh token: %w", err)
	}
	return &rt, nil
}

func (r *RefreshTokenRepository) FindByHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	const q = `
		SELECT id, subject_type, subject_id, token_hash, expires_at, revoked_at, created_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`
	var rt models.RefreshToken
	err := r.db.Conn(ctx).QueryRow(ctx, q, tokenHash).Scan(
		&rt.ID,
		&rt.SubjectType,
		&rt.SubjectID,
		&rt.TokenHash,
		&rt.ExpiresAt,
		&rt.RevokedAt,
		&rt.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("find refresh token by hash: %w", err)
	}
	return &rt, nil
}

func (r *RefreshTokenRepository) RevokeByHash(ctx context.Context, tokenHash string, now time.Time) error {
	const q = `
		UPDATE refresh_tokens
		SET revoked_at = $2
		WHERE token_hash = $1 AND revoked_at IS NULL
	`
	_, err := r.db.Conn(ctx).Exec(ctx, q, tokenHash, now)
	if err != nil {
		return fmt.Errorf("revoke refresh token by hash: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) RevokeByID(ctx context.Context, id uuid.UUID, now time.Time) error {
	const q = `
		UPDATE refresh_tokens
		SET revoked_at = $2
		WHERE id = $1 AND revoked_at IS NULL
	`
	_, err := r.db.Conn(ctx).Exec(ctx, q, id, now)
	if err != nil {
		return fmt.Errorf("revoke refresh token by id: %w", err)
	}
	return nil
}
