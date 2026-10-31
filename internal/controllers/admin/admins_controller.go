package admin

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/http/middleware"
	"github.com/stayhub/backend/internal/http/response"
)

type AdminsController struct {
	adminService interfaces.AdminService
}

func NewAdminsController(adminService interfaces.AdminService) *AdminsController {
	return &AdminsController{
		adminService: adminService,
	}
}

func (ctrl *AdminsController) ListAdmins(c *gin.Context) {
	admins, err := ctrl.adminService.ListAdmins(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Failed to list admins")
		return
	}

	response.OK(c, gin.H{
		"admins": admins,
		"count":  len(admins),
	})
}

func (ctrl *AdminsController) CreateAdmin(c *gin.Context) {
	input, valErr := ValidateCreateAdmin(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	created, err := ctrl.adminService.CreateAdmin(c.Request.Context(), actor, input)
	if err != nil {
		if errors.Is(err, apperrors.ErrValidation) {
			response.Fail(c, http.StatusBadRequest, "CREATE_ADMIN_FAILED", err.Error(), nil)
			return
		}
		response.InternalError(c, "Failed to create admin")
		return
	}

	response.Created(c, created)
}

func (ctrl *AdminsController) UpdateAdmin(c *gin.Context) {
	id, valErr := ParseUUIDParam(c, "id")
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	input, valErr := ValidateUpdateAdmin(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	updated, err := ctrl.adminService.UpdateAdmin(c.Request.Context(), actor, id, input)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Admin user not found")
			return
		}
		response.InternalError(c, "Failed to update admin")
		return
	}

	response.OK(c, updated)
}
