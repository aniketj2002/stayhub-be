package audit

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/controllers"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
)

var validActorTypes = map[string]bool{
	string(enums.ActorGuest):  true,
	string(enums.ActorHost):   true,
	string(enums.ActorCohost): true,
	string(enums.ActorAdmin):  true,
	string(enums.ActorSystem): true,
}

func ValidateListEventsFilter(c *gin.Context) (dtos.EventListFilter, *controllers.ValidationError) {
	var fieldErrs []controllers.FieldError

	objectType := strings.TrimSpace(c.Query("object_type"))
	actorType := strings.TrimSpace(c.Query("actor_type"))
	action := strings.TrimSpace(c.Query("action"))

	if actorType != "" && !validActorTypes[actorType] {
		fieldErrs = append(fieldErrs, controllers.FieldError{
			Field:   "actor_type",
			Message: "actor_type must be one of: guest, host, cohost, admin, system",
		})
	}

	objectID := controllers.ParseOptionalUUID(c.Query("object_id"), "object_id", &fieldErrs)
	actorID := controllers.ParseOptionalUUID(c.Query("actor_id"), "actor_id", &fieldErrs)
	limit, offset := controllers.ParsePagination(c.Query("limit"), c.Query("offset"), 50, 200, &fieldErrs)

	if len(fieldErrs) > 0 {
		return dtos.EventListFilter{}, controllers.NewValidationError(
			"VALIDATION_ERROR",
			"Invalid event log filter parameters",
			fieldErrs...,
		)
	}

	return dtos.EventListFilter{
		ObjectType: objectType,
		ObjectID:   objectID,
		ActorType:  actorType,
		ActorID:    actorID,
		Action:     action,
		Limit:      limit,
		Offset:     offset,
	}, nil
}
