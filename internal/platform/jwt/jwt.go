package jwt

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/platform/clock"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token has expired")
)

const (
	SubjectTypeUser  = "user"
	SubjectTypeAdmin = "admin"
)

type Header struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type Claims struct {
	JTI         string    `json:"jti"`
	SubjectID   uuid.UUID `json:"sub"`
	SubjectType string    `json:"sub_type"` // "user" | "admin"
	Email       string    `json:"email"`
	IsGuest     bool      `json:"is_guest,omitempty"`
	IsHost      bool      `json:"is_host,omitempty"`
	IssuedAt    int64     `json:"iat"`
	ExpiresAt   int64     `json:"exp"`
}

type Manager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	clock      clock.Clock
}

func NewManager(secret string, accessTTL, refreshTTL time.Duration, clk clock.Clock) *Manager {
	return &Manager{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		clock:      clk,
	}
}

func (m *Manager) AccessTTL() time.Duration {
	return m.accessTTL
}

func (m *Manager) RefreshTTL() time.Duration {
	return m.refreshTTL
}

func (m *Manager) GenerateUserAccessToken(userID uuid.UUID, email string, isGuest, isHost bool) (string, *Claims, error) {
	now := m.clock.Now().UTC()
	claims := &Claims{
		JTI:         uuid.NewString(),
		SubjectID:   userID,
		SubjectType: SubjectTypeUser,
		Email:       email,
		IsGuest:     isGuest,
		IsHost:      isHost,
		IssuedAt:    now.Unix(),
		ExpiresAt:   now.Add(m.accessTTL).Unix(),
	}
	token, err := m.sign(claims)
	if err != nil {
		return "", nil, err
	}
	return token, claims, nil
}

func (m *Manager) GenerateAdminAccessToken(adminID uuid.UUID, email string) (string, *Claims, error) {
	now := m.clock.Now().UTC()
	claims := &Claims{
		JTI:         uuid.NewString(),
		SubjectID:   adminID,
		SubjectType: SubjectTypeAdmin,
		Email:       email,
		IssuedAt:    now.Unix(),
		ExpiresAt:   now.Add(m.accessTTL).Unix(),
	}
	token, err := m.sign(claims)
	if err != nil {
		return "", nil, err
	}
	return token, claims, nil
}

func (m *Manager) VerifyAccessToken(tokenStr string) (*Claims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var hdr Header
	if err := json.Unmarshal(headerBytes, &hdr); err != nil || hdr.Alg != "HS256" {
		return nil, ErrInvalidToken
	}

	signingInput := parts[0] + "." + parts[1]
	expectedSig := m.computeHMAC([]byte(signingInput))
	actualSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrInvalidToken
	}
	if !hmac.Equal(expectedSig, actualSig) {
		return nil, ErrInvalidToken
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	var claims Claims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	if claims.SubjectID == uuid.Nil || (claims.SubjectType != SubjectTypeUser && claims.SubjectType != SubjectTypeAdmin) {
		return nil, ErrInvalidToken
	}

	nowUnix := m.clock.Now().UTC().Unix()
	if nowUnix >= claims.ExpiresAt {
		return nil, ErrExpiredToken
	}

	return &claims, nil
}

// GenerateRefreshToken returns the raw opaque token, its SHA-256 hex hash for DB storage, and its expiry timestamp.
func (m *Manager) GenerateRefreshToken() (rawToken string, tokenHash string, expiresAt time.Time, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", time.Time{}, fmt.Errorf("generate refresh token bytes: %w", err)
	}
	rawToken = base64.RawURLEncoding.EncodeToString(b)
	tokenHash = HashRefreshToken(rawToken)
	expiresAt = m.clock.Now().UTC().Add(m.refreshTTL)
	return rawToken, tokenHash, expiresAt, nil
}

// HashRefreshToken computes the deterministic SHA-256 hex digest of a raw refresh token.
func HashRefreshToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

func (m *Manager) sign(claims *Claims) (string, error) {
	hdrBytes, err := json.Marshal(Header{Alg: "HS256", Typ: "JWT"})
	if err != nil {
		return "", err
	}
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	encodedHdr := base64.RawURLEncoding.EncodeToString(hdrBytes)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signingInput := encodedHdr + "." + encodedPayload
	sig := m.computeHMAC([]byte(signingInput))
	encodedSig := base64.RawURLEncoding.EncodeToString(sig)

	return signingInput + "." + encodedSig, nil
}

func (m *Manager) computeHMAC(data []byte) []byte {
	h := hmac.New(sha256.New, m.secret)
	h.Write(data)
	return h.Sum(nil)
}
