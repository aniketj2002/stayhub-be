package auth

import (
	"errors"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/http/middleware"
	"github.com/stayhub/backend/internal/http/response"
)

type Controller struct {
	authService interfaces.AuthService
}

func NewController(authService interfaces.AuthService) *Controller {
	return &Controller{
		authService: authService,
	}
}

func (ctrl *Controller) Register(c *gin.Context) {
	in, valErr := ValidateRegisterUserInput(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	res, err := ctrl.authService.Register(c.Request.Context(), in)
	if err != nil {
		if errors.Is(err, apperrors.ErrEmailAlreadyExists) {
			response.Conflict(c, "EMAIL_ALREADY_EXISTS", "An account with this email already exists", nil)
			return
		}
		log.Printf("auth.Register error (request_id=%s): %v", c.GetString("request_id"), err)
		response.InternalError(c, "Failed to register user")
		return
	}

	response.Created(c, res)
}

func (ctrl *Controller) LoginUser(c *gin.Context) {
	in, valErr := ValidateLoginInput(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	res, err := ctrl.authService.LoginUser(c.Request.Context(), in)
	if err != nil {
		if errors.Is(err, apperrors.ErrInvalidCredentials) {
			response.Unauthorized(c, "Invalid email or password")
			return
		}
		response.InternalError(c, "Failed to login")
		return
	}

	response.OK(c, res)
}

func (ctrl *Controller) LoginAdmin(c *gin.Context) {
	in, valErr := ValidateLoginInput(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	res, err := ctrl.authService.LoginAdmin(c.Request.Context(), in)
	if err != nil {
		if errors.Is(err, apperrors.ErrInvalidCredentials) {
			response.Unauthorized(c, "Invalid admin email or password")
			return
		}
		response.InternalError(c, "Failed to login as admin")
		return
	}

	response.OK(c, res)
}

func (ctrl *Controller) Refresh(c *gin.Context) {
	in, valErr := ValidateRefreshTokenInput(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	res, err := ctrl.authService.Refresh(c.Request.Context(), in)
	if err != nil {
		if errors.Is(err, apperrors.ErrInvalidRefreshToken) || errors.Is(err, apperrors.ErrNotFound) {
			response.Unauthorized(c, "Invalid or expired refresh token")
			return
		}
		response.InternalError(c, "Failed to refresh token")
		return
	}

	response.OK(c, res)
}

func (ctrl *Controller) Logout(c *gin.Context) {
	in, valErr := ValidateRefreshTokenInput(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	if err := ctrl.authService.Logout(c.Request.Context(), in); err != nil {
		response.InternalError(c, "Failed to logout")
		return
	}

	response.OK(c, gin.H{"revoked": true})
}

func (ctrl *Controller) GetMe(c *gin.Context) {
	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	res, err := ctrl.authService.GetMe(c.Request.Context(), actor)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Account not found")
			return
		}
		response.InternalError(c, "Failed to load current account")
		return
	}

	response.OK(c, res)
}

func (ctrl *Controller) BecomeHost(c *gin.Context) {
	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	res, err := ctrl.authService.BecomeHost(c.Request.Context(), actor.SubjectID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "User not found")
			return
		}
		response.InternalError(c, "Failed to enable host role")
		return
	}

	response.OK(c, res)
}

func (ctrl *Controller) UpdateProfile(c *gin.Context) {
	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	in, valErr := ValidateUpdateProfileInput(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	res, err := ctrl.authService.UpdateProfile(c.Request.Context(), actor.SubjectID, in)
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrNotFound):
			response.NotFound(c, "Referenced media or user not found")
		case errors.Is(err, apperrors.ErrMediaOwnership):
			response.Forbidden(c, "Avatar media must be uploaded by your account")
		case errors.Is(err, apperrors.ErrMediaNotUploaded):
			response.BadRequest(c, "MEDIA_NOT_CONFIRMED", "Avatar media upload must be confirmed before attaching", nil)
		default:
			response.InternalError(c, "Failed to update profile")
		}
		return
	}

	response.OK(c, gin.H{"user": res})
}
