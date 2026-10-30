package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/http/response"
)

func TestRequireIdempotency_EnforcesHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/reservations", RequireIdempotency(nil, nil), func(c *gin.Context) {
		response.Created(c, gin.H{"status": "confirmed"})
	})

	// 1. Missing Idempotency-Key -> 400 MISSING_IDEMPOTENCY_KEY
	reqMissing := httptest.NewRequest(http.MethodPost, "/api/v1/reservations", bytes.NewBufferString(`{"guests":2}`))
	reqMissing.Header.Set("Content-Type", "application/json")
	wMissing := httptest.NewRecorder()
	r.ServeHTTP(wMissing, reqMissing)

	if wMissing.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when Idempotency-Key header is missing, got %d (%s)", wMissing.Code, wMissing.Body.String())
	}

	// 2. Present Idempotency-Key -> passes through to handler (201 Created)
	reqValid := httptest.NewRequest(http.MethodPost, "/api/v1/reservations", bytes.NewBufferString(`{"guests":2}`))
	reqValid.Header.Set("Content-Type", "application/json")
	reqValid.Header.Set(IdempotencyHeader, "idem-key-123")
	wValid := httptest.NewRecorder()
	r.ServeHTTP(wValid, reqValid)

	if wValid.Code != http.StatusCreated {
		t.Fatalf("expected 201 when Idempotency-Key header is provided, got %d (%s)", wValid.Code, wValid.Body.String())
	}
}
