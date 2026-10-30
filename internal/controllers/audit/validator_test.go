package audit_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/controllers/audit"
)

func TestValidateListEventsFilter_Valid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	objID := uuid.New()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/v1/events?object_type=reservation&object_id="+objID.String()+"&actor_type=guest&limit=25&offset=10",
		nil,
	)

	filter, valErr := audit.ValidateListEventsFilter(c)
	if valErr != nil {
		t.Fatalf("expected no validation error, got: %+v", valErr)
	}
	if filter.ObjectType != "reservation" || filter.ObjectID == nil || *filter.ObjectID != objID {
		t.Fatalf("unexpected object filter: %+v", filter)
	}
	if filter.ActorType != "guest" || filter.Limit != 25 || filter.Offset != 10 {
		t.Fatalf("unexpected pagination/actor filter: %+v", filter)
	}
}

func TestValidateListEventsFilter_InvalidFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/v1/events?object_id=not-a-uuid&actor_type=invalid_role&limit=-5",
		nil,
	)

	_, valErr := audit.ValidateListEventsFilter(c)
	if valErr == nil {
		t.Fatal("expected validation error for invalid query params, got nil")
	}
	if len(valErr.Fields) != 3 {
		t.Fatalf("expected 3 field errors (actor_type, object_id, limit), got %d: %+v", len(valErr.Fields), valErr.Fields)
	}
}
