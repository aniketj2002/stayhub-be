package admin

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/controllers"
	"github.com/stayhub/backend/internal/domain/dtos"
)

func ValidateAdminEventFilter(c *gin.Context) (dtos.EventListFilter, *controllers.ValidationError) {
	var fieldErrs []controllers.FieldError
	var filter dtos.EventListFilter

	filter.ObjectType = strings.TrimSpace(c.Query("object_type"))
	filter.ObjectID = controllers.ParseOptionalUUID(c.Query("object_id"), "object_id", &fieldErrs)
	filter.ActorType = strings.TrimSpace(c.Query("actor_type"))
	filter.ActorID = controllers.ParseOptionalUUID(c.Query("actor_id"), "actor_id", &fieldErrs)
	filter.Action = strings.TrimSpace(c.Query("action"))
	filter.Query = strings.TrimSpace(c.Query("q"))
	if len(filter.Query) > 200 {
		fieldErrs = append(fieldErrs, controllers.FieldError{Field: "q", Message: "q must be at most 200 characters"})
	}
	filter.From = parseAuditDate(c.Query("from"), "from", false, &fieldErrs)
	filter.To = parseAuditDate(c.Query("to"), "to", true, &fieldErrs)

	limit, offset := controllers.ParsePagination(c.Query("limit"), c.Query("offset"), 50, 5000, &fieldErrs)
	filter.Limit = limit
	filter.Offset = offset

	if len(fieldErrs) > 0 {
		return filter, controllers.NewValidationError("INVALID_QUERY_PARAMS", "Invalid event query parameters", fieldErrs...)
	}
	return filter, nil
}

func parseAuditDate(value, field string, endOfDay bool, errs *[]controllers.FieldError) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		*errs = append(*errs, controllers.FieldError{Field: field, Message: field + " must be YYYY-MM-DD"})
		return nil
	}
	if endOfDay {
		parsed = parsed.Add(24*time.Hour - time.Nanosecond)
	}
	return &parsed
}
