package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/http/response"
	"github.com/stayhub/backend/internal/platform/jwt"
)

const AuthorizationHeader = "Authorization"

func ActorFromGin(c *gin.Context) (models.AuthActor, bool) {
	if v, ok := c.Get("auth_actor"); ok {
		if actor, ok := v.(models.AuthActor); ok {
			return actor, true
		}
	}
	return models.AuthActorFromContext(c.Request.Context())
}

// RequireAuth verifies a Bearer JWT access token (user or admin) and populates AuthActor in context.
func RequireAuth(jwtManager *jwt.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := authenticateRequest(c, jwtManager)
		if !ok {
			return
		}
		storeActorInContext(c, actor)
		c.Next()
	}
}

// RequireUser verifies that the caller is an authenticated Guest/Host user (not an admin account).
func RequireUser(jwtManager *jwt.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := authenticateRequest(c, jwtManager)
		if !ok {
			return
		}
		if actor.SubjectType != jwt.SubjectTypeUser {
			response.Forbidden(c, "This endpoint requires a user account")
			return
		}
		storeActorInContext(c, actor)
		c.Next()
	}
}

// RequireHost verifies that the caller is an authenticated user with is_host = true.
// If the token claim has is_host=false (e.g. immediately after calling /me/become-host),
// it falls back to checking the users table before rejecting.
func RequireHost(jwtManager *jwt.Manager, userRepo interfaces.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := authenticateRequest(c, jwtManager)
		if !ok {
			return
		}
		if actor.SubjectType != jwt.SubjectTypeUser {
			response.Forbidden(c, "This endpoint requires a host user account")
			return
		}

		if !actor.IsHost && userRepo != nil {
			u, err := userRepo.FindByID(c.Request.Context(), actor.SubjectID)
			if err == nil && u.IsHost {
				actor.IsHost = true
				actor.IsGuest = u.IsGuest
			}
		}

		if !actor.IsHost {
			response.Fail(c, http.StatusForbidden, "HOST_ROLE_REQUIRED", "Host role is required to perform this action", nil)
			return
		}

		storeActorInContext(c, actor)
		c.Next()
	}
}

// RequireAdmin verifies that the caller is an authenticated admin from admin_users.
func RequireAdmin(jwtManager *jwt.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := authenticateRequest(c, jwtManager)
		if !ok {
			return
		}
		if actor.SubjectType != jwt.SubjectTypeAdmin {
			response.Forbidden(c, "Administrator privileges are required")
			return
		}
		storeActorInContext(c, actor)
		c.Next()
	}
}

func authenticateRequest(c *gin.Context, jwtManager *jwt.Manager) (models.AuthActor, bool) {
	rawHeader := strings.TrimSpace(c.GetHeader(AuthorizationHeader))
	if rawHeader == "" {
		response.Unauthorized(c, "Missing Authorization header")
		return models.AuthActor{}, false
	}

	parts := strings.SplitN(rawHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		response.Unauthorized(c, "Authorization header must be in format 'Bearer <token>'")
		return models.AuthActor{}, false
	}

	claims, err := jwtManager.VerifyAccessToken(strings.TrimSpace(parts[1]))
	if err != nil {
		if errors.Is(err, jwt.ErrExpiredToken) {
			response.Fail(c, http.StatusUnauthorized, "TOKEN_EXPIRED", "Access token has expired", nil)
			return models.AuthActor{}, false
		}
		response.Unauthorized(c, "Invalid access token")
		return models.AuthActor{}, false
	}

	return models.AuthActor{
		SubjectID:   claims.SubjectID,
		SubjectType: claims.SubjectType,
		Email:       claims.Email,
		IsGuest:     claims.IsGuest,
		IsHost:      claims.IsHost,
	}, true
}

func storeActorInContext(c *gin.Context, actor models.AuthActor) {
	c.Set("subject_id", actor.SubjectID)
	c.Set("subject_type", actor.SubjectType)
	c.Set("auth_actor", actor)

	ctx := models.WithAuthActor(c.Request.Context(), actor)
	c.Request = c.Request.WithContext(ctx)
}
