package media

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/http/middleware"
	"github.com/stayhub/backend/internal/http/response"
)

type Controller struct {
	mediaService interfaces.MediaService
}

func NewController(mediaService interfaces.MediaService) *Controller {
	return &Controller{
		mediaService: mediaService,
	}
}

func (ctrl *Controller) PresignUpload(c *gin.Context) {
	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	in, valErr := ValidatePresignMediaInput(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	res, err := ctrl.mediaService.PresignUpload(c.Request.Context(), actor, in)
	if err != nil {
		response.InternalError(c, "Failed to issue presigned upload URL")
		return
	}

	response.Created(c, res)
}

// UploadContent handles PUT /api/v1/media/:id/upload from the browser's XHR upload,
// persisting the file locally and forwarding to AWS S3 if S3 credentials are configured.
func (ctrl *Controller) UploadContent(c *gin.Context) {
	mediaID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", "id must be a valid UUID", nil)
		return
	}

	limited := io.LimitReader(c.Request.Body, maxMediaSizeBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		response.BadRequest(c, "INVALID_UPLOAD_BODY", "Failed to read uploaded file bytes", nil)
		return
	}
	if int64(len(data)) == 0 || int64(len(data)) > maxMediaSizeBytes {
		response.BadRequest(c, "VALIDATION_ERROR", "Uploaded file must be between 1 byte and 20 MB", nil)
		return
	}

	if err := ctrl.mediaService.UploadContent(c.Request.Context(), mediaID, c.GetHeader("Content-Type"), data); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Media record not found")
			return
		}
		response.InternalError(c, "Failed to store uploaded media")
		return
	}

	c.Status(http.StatusOK)
}

// ServeFile serves uploaded media files at GET /api/v1/media/file/*filepath.
func (ctrl *Controller) ServeFile(c *gin.Context) {
	relPath := strings.TrimPrefix(c.Param("filepath"), "/")
	if relPath == "" {
		response.NotFound(c, "Media file not found")
		return
	}

	localPath, err := ctrl.mediaService.ResolveLocalFilePath(relPath)
	if err != nil {
		response.BadRequest(c, "INVALID_MEDIA_PATH", "Invalid media file path", nil)
		return
	}

	if _, err := os.Stat(localPath); err != nil {
		response.NotFound(c, "Media file not found")
		return
	}

	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.File(localPath)
}

func (ctrl *Controller) ConfirmUpload(c *gin.Context) {
	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	mediaID, in, valErr := ValidateConfirmMediaInput(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	res, err := ctrl.mediaService.ConfirmUpload(c.Request.Context(), actor, mediaID, in)
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrNotFound):
			response.NotFound(c, "Media record not found")
		case errors.Is(err, apperrors.ErrMediaOwnership):
			response.Forbidden(c, "You can only confirm media uploads initiated by your account")
		default:
			response.InternalError(c, "Failed to confirm media upload")
		}
		return
	}

	response.OK(c, gin.H{"media": res})
}

func (ctrl *Controller) GetMedia(c *gin.Context) {
	mediaID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", "id must be a valid UUID", nil)
		return
	}

	res, err := ctrl.mediaService.GetByID(c.Request.Context(), mediaID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Media record not found")
			return
		}
		response.InternalError(c, "Failed to retrieve media")
		return
	}

	response.OK(c, gin.H{"media": res})
}

// ServeByID redirects or serves media content by UUID at GET /api/v1/media/:id/raw.
func (ctrl *Controller) ServeByID(c *gin.Context) {
	mediaID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", "id must be a valid UUID", nil)
		return
	}

	res, err := ctrl.mediaService.GetByID(c.Request.Context(), mediaID)
	if err != nil || res == nil {
		response.NotFound(c, "Media record not found")
		return
	}

	if res.S3Key != "" {
		if localPath, err := ctrl.mediaService.ResolveLocalFilePath(res.S3Key); err == nil {
			if _, statErr := os.Stat(localPath); statErr == nil {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
				c.File(localPath)
				return
			}
		}
	}

	if res.PublicURL != "" {
		c.Redirect(http.StatusTemporaryRedirect, res.PublicURL)
		return
	}

	response.NotFound(c, "Media file not found")
}
