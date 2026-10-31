package admin

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/http/response"
)

type AuditController struct {
	auditService interfaces.AuditService
}

func NewAuditController(auditService interfaces.AuditService) *AuditController {
	return &AuditController{
		auditService: auditService,
	}
}

func (ctrl *AuditController) ListEvents(c *gin.Context) {
	filter, valErr := ValidateAdminEventFilter(c)
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

func (ctrl *AuditController) ExportCSV(c *gin.Context) {
	filter, valErr := ValidateAdminEventFilter(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}
	if filter.Limit <= 0 || filter.Limit > 5000 {
		filter.Limit = 5000
	}

	records, err := ctrl.auditService.ListEvents(c.Request.Context(), filter)
	if err != nil {
		response.InternalError(c, "Failed to query events for export")
		return
	}

	filename := fmt.Sprintf("audit_events_%s.csv", time.Now().UTC().Format("20060102_150405"))
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Status(http.StatusOK)

	writer := csv.NewWriter(c.Writer)
	_ = writer.Write([]string{
		"id",
		"occurred_at",
		"actor_type",
		"actor_id",
		"action",
		"object_type",
		"object_id",
		"reason",
		"request_id",
	})

	for _, r := range records {
		actorID := ""
		if r.ActorID != nil {
			actorID = r.ActorID.String()
		}
		reason := ""
		if r.Reason != nil {
			reason = *r.Reason
		}
		reqID := ""
		if r.RequestID != nil {
			reqID = *r.RequestID
		}

		_ = writer.Write([]string{
			fmt.Sprintf("%d", r.ID),
			r.OccurredAt.Format(time.RFC3339),
			string(r.ActorType),
			actorID,
			r.Action,
			r.ObjectType,
			r.ObjectID.String(),
			reason,
			reqID,
		})
	}
	writer.Flush()
}
