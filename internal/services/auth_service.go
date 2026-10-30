package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/db"
	"github.com/stayhub/backend/internal/platform/jwt"
	"github.com/stayhub/backend/internal/platform/storage"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	db          *db.DB
	userRepo    interfaces.UserRepository
	adminRepo   interfaces.AdminUserRepository
	refreshRepo interfaces.RefreshTokenRepository
	mediaRepo   interfaces.MediaRepository
	storage     *storage.Storage
	jwtManager  *jwt.Manager
	audit       interfaces.AuditService
	clock       clock.Clock
}

func NewAuthService(
	database *db.DB,
	userRepo interfaces.UserRepository,
	adminRepo interfaces.AdminUserRepository,
	refreshRepo interfaces.RefreshTokenRepository,
	mediaRepo interfaces.MediaRepository,
	store *storage.Storage,
	jwtManager *jwt.Manager,
	audit interfaces.AuditService,
	clk clock.Clock,
) interfaces.AuthService {
	return &AuthService{
		db:          database,
		userRepo:    userRepo,
		adminRepo:   adminRepo,
		refreshRepo: refreshRepo,
		mediaRepo:   mediaRepo,
		storage:     store,
		jwtManager:  jwtManager,
		audit:       audit,
		clock:       clk,
	}
}

func (s *AuthService) Register(ctx context.Context, in dtos.RegisterUserInput) (*dtos.AuthTokenResponse, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	fullName := strings.TrimSpace(in.FullName)

	pwdHashBytes, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	var createdUser *models.User
	var accessToken, rawRefresh string

	err = s.db.WithinTx(ctx, func(txCtx context.Context) error {
		u, err := s.userRepo.Create(txCtx, dtos.CreateUserParams{
			Email:        email,
			PasswordHash: string(pwdHashBytes),
			FullName:     fullName,
			IsGuest:      true,
			IsHost:       in.IsHost,
		})
		if err != nil {
			return err
		}
		createdUser = u

		tok, _, err := s.jwtManager.GenerateUserAccessToken(u.ID, u.Email, u.IsGuest, u.IsHost)
		if err != nil {
			return err
		}
		accessToken = tok

		raw, hash, expAt, err := s.jwtManager.GenerateRefreshToken()
		if err != nil {
			return err
		}
		rawRefresh = raw

		if _, err := s.refreshRepo.Create(txCtx, dtos.CreateRefreshTokenParams{
			SubjectType: jwt.SubjectTypeUser,
			SubjectID:   u.ID,
			TokenHash:   hash,
			ExpiresAt:   expAt,
		}); err != nil {
			return err
		}

		actorType := enums.ActorGuest
		if u.IsHost {
			actorType = enums.ActorHost
		}
		return s.audit.Record(txCtx, dtos.AuditEntryInput{
			ActorType:  actorType,
			ActorID:    &u.ID,
			Action:     "user.register",
			ObjectType: "user",
			ObjectID:   u.ID,
			AfterState: map[string]any{
				"email":     u.Email,
				"full_name": u.FullName,
				"is_guest":  u.IsGuest,
				"is_host":   u.IsHost,
			},
		})
	})
	if err != nil {
		return nil, err
	}

	userResp := s.buildUserResponse(ctx, *createdUser)
	return &dtos.AuthTokenResponse{
		SubjectType:  jwt.SubjectTypeUser,
		User:         &userResp,
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresIn:    int64(s.jwtManager.AccessTTL().Seconds()),
	}, nil
}

func (s *AuthService) LoginUser(ctx context.Context, in dtos.LoginInput) (*dtos.AuthTokenResponse, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	u, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, apperrors.ErrInvalidCredentials
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Password)); err != nil {
		return nil, apperrors.ErrInvalidCredentials
	}

	var accessToken, rawRefresh string
	err = s.db.WithinTx(ctx, func(txCtx context.Context) error {
		tok, _, err := s.jwtManager.GenerateUserAccessToken(u.ID, u.Email, u.IsGuest, u.IsHost)
		if err != nil {
			return err
		}
		accessToken = tok

		raw, hash, expAt, err := s.jwtManager.GenerateRefreshToken()
		if err != nil {
			return err
		}
		rawRefresh = raw

		if _, err := s.refreshRepo.Create(txCtx, dtos.CreateRefreshTokenParams{
			SubjectType: jwt.SubjectTypeUser,
			SubjectID:   u.ID,
			TokenHash:   hash,
			ExpiresAt:   expAt,
		}); err != nil {
			return err
		}

		actorType := enums.ActorGuest
		if u.IsHost {
			actorType = enums.ActorHost
		}
		return s.audit.Record(txCtx, dtos.AuditEntryInput{
			ActorType:  actorType,
			ActorID:    &u.ID,
			Action:     "user.login",
			ObjectType: "user",
			ObjectID:   u.ID,
		})
	})
	if err != nil {
		return nil, err
	}

	userResp := s.buildUserResponse(ctx, *u)
	return &dtos.AuthTokenResponse{
		SubjectType:  jwt.SubjectTypeUser,
		User:         &userResp,
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresIn:    int64(s.jwtManager.AccessTTL().Seconds()),
	}, nil
}

func (s *AuthService) LoginAdmin(ctx context.Context, in dtos.LoginInput) (*dtos.AuthTokenResponse, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	admin, err := s.adminRepo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, apperrors.ErrInvalidCredentials
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(in.Password)); err != nil {
		return nil, apperrors.ErrInvalidCredentials
	}

	var accessToken, rawRefresh string
	err = s.db.WithinTx(ctx, func(txCtx context.Context) error {
		tok, _, err := s.jwtManager.GenerateAdminAccessToken(admin.ID, admin.Email)
		if err != nil {
			return err
		}
		accessToken = tok

		raw, hash, expAt, err := s.jwtManager.GenerateRefreshToken()
		if err != nil {
			return err
		}
		rawRefresh = raw

		if _, err := s.refreshRepo.Create(txCtx, dtos.CreateRefreshTokenParams{
			SubjectType: jwt.SubjectTypeAdmin,
			SubjectID:   admin.ID,
			TokenHash:   hash,
			ExpiresAt:   expAt,
		}); err != nil {
			return err
		}

		return s.audit.Record(txCtx, dtos.AuditEntryInput{
			ActorType:  enums.ActorAdmin,
			ActorID:    &admin.ID,
			Action:     "admin.login",
			ObjectType: "admin_user",
			ObjectID:   admin.ID,
		})
	})
	if err != nil {
		return nil, err
	}

	adminResp := dtos.ToAdminUserResponse(*admin)
	return &dtos.AuthTokenResponse{
		SubjectType:  jwt.SubjectTypeAdmin,
		Admin:        &adminResp,
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresIn:    int64(s.jwtManager.AccessTTL().Seconds()),
	}, nil
}

func (s *AuthService) Refresh(ctx context.Context, in dtos.RefreshTokenInput) (*dtos.AuthTokenResponse, error) {
	tokenHash := jwt.HashRefreshToken(strings.TrimSpace(in.RefreshToken))
	rt, err := s.refreshRepo.FindByHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, apperrors.ErrInvalidRefreshToken
		}
		return nil, err
	}

	now := s.clock.Now()
	if rt.RevokedAt != nil || !rt.ExpiresAt.After(now) {
		return nil, apperrors.ErrInvalidRefreshToken
	}

	var resp *dtos.AuthTokenResponse
	err = s.db.WithinTx(ctx, func(txCtx context.Context) error {
		if err := s.refreshRepo.RevokeByID(txCtx, rt.ID, now); err != nil {
			return err
		}

		rawRefresh, newHash, expAt, err := s.jwtManager.GenerateRefreshToken()
		if err != nil {
			return err
		}

		if _, err := s.refreshRepo.Create(txCtx, dtos.CreateRefreshTokenParams{
			SubjectType: rt.SubjectType,
			SubjectID:   rt.SubjectID,
			TokenHash:   newHash,
			ExpiresAt:   expAt,
		}); err != nil {
			return err
		}

		if rt.SubjectType == jwt.SubjectTypeAdmin {
			admin, err := s.adminRepo.FindByID(txCtx, rt.SubjectID)
			if err != nil {
				return err
			}
			accessTok, _, err := s.jwtManager.GenerateAdminAccessToken(admin.ID, admin.Email)
			if err != nil {
				return err
			}
			adminResp := dtos.ToAdminUserResponse(*admin)
			resp = &dtos.AuthTokenResponse{
				SubjectType:  jwt.SubjectTypeAdmin,
				Admin:        &adminResp,
				AccessToken:  accessTok,
				RefreshToken: rawRefresh,
				ExpiresIn:    int64(s.jwtManager.AccessTTL().Seconds()),
			}
			return nil
		}

		user, err := s.userRepo.FindByID(txCtx, rt.SubjectID)
		if err != nil {
			return err
		}
		accessTok, _, err := s.jwtManager.GenerateUserAccessToken(user.ID, user.Email, user.IsGuest, user.IsHost)
		if err != nil {
			return err
		}
		userResp := s.buildUserResponse(txCtx, *user)
		resp = &dtos.AuthTokenResponse{
			SubjectType:  jwt.SubjectTypeUser,
			User:         &userResp,
			AccessToken:  accessTok,
			RefreshToken: rawRefresh,
			ExpiresIn:    int64(s.jwtManager.AccessTTL().Seconds()),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *AuthService) Logout(ctx context.Context, in dtos.RefreshTokenInput) error {
	raw := strings.TrimSpace(in.RefreshToken)
	if raw == "" {
		return nil
	}
	tokenHash := jwt.HashRefreshToken(raw)
	return s.refreshRepo.RevokeByHash(ctx, tokenHash, s.clock.Now())
}

func (s *AuthService) GetMe(ctx context.Context, actor models.AuthActor) (*dtos.MeResponse, error) {
	if actor.SubjectType == jwt.SubjectTypeAdmin {
		admin, err := s.adminRepo.FindByID(ctx, actor.SubjectID)
		if err != nil {
			return nil, err
		}
		adminResp := dtos.ToAdminUserResponse(*admin)
		return &dtos.MeResponse{
			SubjectType: jwt.SubjectTypeAdmin,
			Admin:       &adminResp,
		}, nil
	}

	user, err := s.userRepo.FindByID(ctx, actor.SubjectID)
	if err != nil {
		return nil, err
	}
	userResp := s.buildUserResponse(ctx, *user)
	return &dtos.MeResponse{
		SubjectType: jwt.SubjectTypeUser,
		User:        &userResp,
	}, nil
}

func (s *AuthService) BecomeHost(ctx context.Context, userID uuid.UUID) (*dtos.BecomeHostResponse, error) {
	var updatedUser *models.User
	var accessToken string

	err := s.db.WithinTx(ctx, func(txCtx context.Context) error {
		existing, err := s.userRepo.FindByID(txCtx, userID)
		if err != nil {
			return err
		}

		if existing.IsHost {
			updatedUser = existing
		} else {
			u, err := s.userRepo.EnableHostRole(txCtx, userID, s.clock.Now())
			if err != nil {
				return err
			}
			updatedUser = u

			if err := s.audit.Record(txCtx, dtos.AuditEntryInput{
				ActorType:   enums.ActorHost,
				ActorID:     &userID,
				Action:      "user.become_host",
				ObjectType:  "user",
				ObjectID:    userID,
				BeforeState: map[string]any{"is_host": false},
				AfterState:  map[string]any{"is_host": true},
			}); err != nil {
				return err
			}
		}

		tok, _, err := s.jwtManager.GenerateUserAccessToken(updatedUser.ID, updatedUser.Email, updatedUser.IsGuest, updatedUser.IsHost)
		if err != nil {
			return err
		}
		accessToken = tok
		return nil
	})
	if err != nil {
		return nil, err
	}

	userResp := s.buildUserResponse(ctx, *updatedUser)
	return &dtos.BecomeHostResponse{
		User:        userResp,
		AccessToken: accessToken,
		ExpiresIn:   int64(s.jwtManager.AccessTTL().Seconds()),
	}, nil
}

func (s *AuthService) UpdateProfile(ctx context.Context, userID uuid.UUID, in dtos.UpdateProfileInput) (*dtos.UserResponse, error) {
	var updatedUser *models.User

	err := s.db.WithinTx(ctx, func(txCtx context.Context) error {
		existing, err := s.userRepo.FindByID(txCtx, userID)
		if err != nil {
			return err
		}

		fullName := existing.FullName
		if in.FullName != nil && strings.TrimSpace(*in.FullName) != "" {
			fullName = strings.TrimSpace(*in.FullName)
		}

		avatarID := existing.AvatarMediaID
		if in.AvatarMediaID != nil {
			m, err := s.mediaRepo.FindByID(txCtx, *in.AvatarMediaID)
			if err != nil {
				return err
			}
			if m.UploadedBy != userID {
				return apperrors.ErrMediaOwnership
			}
			if m.Status != "uploaded" {
				return apperrors.ErrMediaNotUploaded
			}
			avatarID = in.AvatarMediaID
		}

		u, err := s.userRepo.UpdateProfile(txCtx, userID, fullName, avatarID, s.clock.Now())
		if err != nil {
			return err
		}
		updatedUser = u

		actorType := enums.ActorGuest
		if u.IsHost {
			actorType = enums.ActorHost
		}
		return s.audit.Record(txCtx, dtos.AuditEntryInput{
			ActorType:  actorType,
			ActorID:    &userID,
			Action:     "user.update_profile",
			ObjectType: "user",
			ObjectID:   userID,
			BeforeState: map[string]any{
				"full_name":       existing.FullName,
				"avatar_media_id": existing.AvatarMediaID,
			},
			AfterState: map[string]any{
				"full_name":       u.FullName,
				"avatar_media_id": u.AvatarMediaID,
			},
		})
	})
	if err != nil {
		return nil, err
	}

	resp := s.buildUserResponse(ctx, *updatedUser)
	return &resp, nil
}

func (s *AuthService) buildUserResponse(ctx context.Context, u models.User) dtos.UserResponse {
	var avatarURL *string
	if u.AvatarMediaID != nil && s.mediaRepo != nil && s.storage != nil {
		if m, err := s.mediaRepo.FindByID(ctx, *u.AvatarMediaID); err == nil && m.Status == "uploaded" {
			url := s.storage.PublicURL(m.S3Key)
			avatarURL = &url
		}
	}
	return dtos.ToUserResponse(u, avatarURL)
}
