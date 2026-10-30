package auth_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/controllers/auth"
)

func TestValidateRegisterUserInput(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("valid registration input normalizes email and full_name", func(t *testing.T) {
		body := `{"email":"  Host@Example.com ","password":"password123","full_name":" Maria Silva ","is_host":true}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")

		in, err := auth.ValidateRegisterUserInput(c)
		if err != nil {
			t.Fatalf("unexpected validation error: %v", err)
		}
		if in.Email != "host@example.com" || in.FullName != "Maria Silva" || !in.IsHost {
			t.Fatalf("unexpected parsed input: %+v", in)
		}
	})

	t.Run("rejects short password and invalid email", func(t *testing.T) {
		body := `{"email":"not-an-email","password":"short","full_name":""}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")

		_, err := auth.ValidateRegisterUserInput(c)
		if err == nil || len(err.Fields) != 3 {
			t.Fatalf("expected 3 field errors, got: %+v", err)
		}
	})
}
