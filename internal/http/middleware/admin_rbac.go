package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/http/response"
	"github.com/stayhub/backend/internal/platform/jwt"
)

// RequireAdminPermission verifies that the caller is an authenticated admin with the specified permission.
func RequireAdminPermission(
	jwtManager *jwt.Manager,
	adminRepo interfaces.AdminRepository,
	permission string,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := authenticateRequest(c, jwtManager)
		if !ok {
			return
		}
		if actor.SubjectType != jwt.SubjectTypeAdmin {
			response.Forbidden(c, "Administrator privileges are required")
			return
		}

		if adminRepo == nil {
			response.InternalError(c, "Admin repository not configured")
			return
		}

		admin, err := adminRepo.FindByID(c.Request.Context(), actor.SubjectID)
		if err != nil {
			if errors.Is(err, apperrors.ErrNotFound) {
				response.Forbidden(c, "Admin account not found")
				return
			}
			response.InternalError(c, "Failed to verify admin account")
			return
		}

		hasPerm, err := adminRepo.HasPermission(c.Request.Context(), admin.ID, admin.Role, permission)
		if err != nil {
			response.InternalError(c, "Failed to evaluate admin permissions")
			return
		}
		if !hasPerm {
			response.Fail(c, http.StatusForbidden, "INSUFFICIENT_PERMISSIONS", "You do not have permission to perform this action", gin.H{
				"required_permission": permission,
			})
			return
		}

		storeActorInContext(c, actor)
		c.Next()
	}
}
