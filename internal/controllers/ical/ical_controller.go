package ical

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/http/response"
)

type Controller struct {
	icalService interfaces.ICalService
}

func NewController(icalService interfaces.ICalService) *Controller {
	return &Controller{icalService: icalService}
}

func (ctrl *Controller) CreateFeed(c *gin.Context) {
	listingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid listing ID format", nil)
		return
	}

	in, valErr := ValidateCreateICalFeedInput(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	res, err := ctrl.icalService.CreateFeed(c.Request.Context(), listingID, in)
	if err != nil {
		if errors.Is(err, apperrors.ErrValidation) {
			response.BadRequest(c, "VALIDATION_ERROR", err.Error(), nil)
			return
		}
		response.InternalError(c, "Failed to create ical feed")
		return
	}
	response.Created(c, res)
}

func (ctrl *Controller) ListFeeds(c *gin.Context) {
	listingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid listing ID format", nil)
		return
	}

	res, err := ctrl.icalService.ListFeeds(c.Request.Context(), listingID)
	if err != nil {
		response.InternalError(c, "Failed to list ical feeds")
		return
	}
	response.OK(c, gin.H{"feeds": res})
}

func (ctrl *Controller) DeleteFeed(c *gin.Context) {
	listingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid listing ID format", nil)
		return
	}
	feedID, err := uuid.Parse(c.Param("feedId"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid feed ID format", nil)
		return
	}

	if err := ctrl.icalService.DeleteFeed(c.Request.Context(), listingID, feedID); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Feed not found")
			return
		}
		if errors.Is(err, apperrors.ErrForbidden) {
			response.Forbidden(c, "Forbidden")
			return
		}
		response.InternalError(c, "Failed to delete ical feed")
		return
	}
	c.Status(http.StatusNoContent)
}

func (ctrl *Controller) RegenerateExportToken(c *gin.Context) {
	listingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid listing ID format", nil)
		return
	}

	res, err := ctrl.icalService.RegenerateExportToken(c.Request.Context(), listingID)
	if err != nil {
		response.InternalError(c, "Failed to regenerate export token")
		return
	}
	response.OK(c, res)
}

// ExportFeed handles GET /ical/:exportToken.ics (public calendar subscription)
func (ctrl *Controller) ExportFeed(c *gin.Context) {
	token := c.Param("token")
	if len(token) > 4 && token[len(token)-4:] == ".ics" {
		token = token[:len(token)-4]
	}

	icsData, err := ctrl.icalService.GetExportFeed(c.Request.Context(), token)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Calendar feed not found")
			return
		}
		response.InternalError(c, "Failed to export calendar feed")
		return
	}

	c.Header("Content-Type", "text/calendar; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s.ics\"", token))
	c.String(http.StatusOK, icsData)
}
