package audit

import (
	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/http/response"
)

type Controller struct {
	auditService interfaces.AuditService
}

func NewController(auditService interfaces.AuditService) *Controller {
	return &Controller{
		auditService: auditService,
	}
}

func (ctrl *Controller) ListEvents(c *gin.Context) {
	filter, valErr := ValidateListEventsFilter(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	records, err := ctrl.auditService.ListEvents(c.Request.Context(), filter)
	if err != nil {
		response.InternalError(c, "Failed to query event log")
		return
	}

	response.OK(c, gin.H{
		"events": records,
		"count":  len(records),
	})
}
