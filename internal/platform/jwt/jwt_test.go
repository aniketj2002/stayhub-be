package jwt

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/platform/clock"
)

func TestJWTManager_UserAndAdminTokens(t *testing.T) {
	t0 := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMockClock(t0)
	mgr := NewManager("test-secret-key", 15*time.Minute, 7*24*time.Hour, clk)

	userID := uuid.New()
	tok, issuedClaims, err := mgr.GenerateUserAccessToken(userID, "guest@example.com", true, false)
	if err != nil {
		t.Fatalf("unexpected error generating token: %v", err)
	}
	if tok == "" || issuedClaims == nil {
		t.Fatalf("expected non-empty token and claims")
	}

	parsed, err := mgr.VerifyAccessToken(tok)
	if err != nil {
		t.Fatalf("unexpected verify error: %v", err)
	}
	if parsed.SubjectID != userID || parsed.SubjectType != SubjectTypeUser || !parsed.IsGuest || parsed.IsHost {
		t.Fatalf("unexpected parsed claims: %+v", parsed)
	}

	// Advance clock past expiry
	clk.Advance(16 * time.Minute)
	if _, err := mgr.VerifyAccessToken(tok); !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("expected ErrExpiredToken, got %v", err)
	}

	// Tampered signature check
	otherMgr := NewManager("wrong-secret", 15*time.Minute, 7*24*time.Hour, clk)
	if _, err := otherMgr.VerifyAccessToken(tok); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken for wrong secret, got %v", err)
	}

	// Refresh token generation & hashing
	raw, hash, exp, err := mgr.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("refresh token err: %v", err)
	}
	if raw == "" || hash == "" || HashRefreshToken(raw) != hash {
		t.Fatalf("refresh token hash mismatch")
	}
	if !exp.After(clk.Now()) {
		t.Fatalf("expected refresh token expiry in the future")
	}
}
