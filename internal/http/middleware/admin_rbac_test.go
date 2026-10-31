package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/http/middleware"
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/jwt"
)

type mockAdminRepo struct {
	admins      map[uuid.UUID]models.AdminUser
	permissions map[uuid.UUID]map[string]bool
}

func newMockAdminRepo() *mockAdminRepo {
	return &mockAdminRepo{
		admins:      make(map[uuid.UUID]models.AdminUser),
		permissions: make(map[uuid.UUID]map[string]bool),
	}
}

func (m *mockAdminRepo) FindByID(_ context.Context, id uuid.UUID) (*models.AdminUser, error) {
	a, ok := m.admins[id]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	return &a, nil
}

func (m *mockAdminRepo) FindByEmail(_ context.Context, email string) (*models.AdminUser, error) {
	for _, a := range m.admins {
		if a.Email == email {
			return &a, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (m *mockAdminRepo) List(_ context.Context) ([]models.AdminUser, error) {
	var list []models.AdminUser
	for _, a := range m.admins {
		list = append(list, a)
	}
	return list, nil
}

func (m *mockAdminRepo) Create(_ context.Context, a models.AdminUser) (*models.AdminUser, error) {
	m.admins[a.ID] = a
	return &a, nil
}

func (m *mockAdminRepo) Update(_ context.Context, a models.AdminUser) (*models.AdminUser, error) {
	m.admins[a.ID] = a
	return &a, nil
}

func (m *mockAdminRepo) GetRolePermissions(_ context.Context, role enums.AdminRole) ([]string, error) {
	if role == enums.AdminRoleSupportL1 {
		return []string{"reservations.read", "audit.read"}, nil
	}
	return nil, nil
}

func (m *mockAdminRepo) GetPermissionsByAdmin(_ context.Context, adminID uuid.UUID) ([]models.AdminPermission, error) {
	var list []models.AdminPermission
	if perms, ok := m.permissions[adminID]; ok {
		for p, g := range perms {
			list = append(list, models.AdminPermission{AdminID: adminID, Permission: p, Granted: g})
		}
	}
	return list, nil
}

func (m *mockAdminRepo) SetPermission(_ context.Context, adminID uuid.UUID, permission string, granted bool) error {
	if _, ok := m.permissions[adminID]; !ok {
		m.permissions[adminID] = make(map[string]bool)
	}
	m.permissions[adminID][permission] = granted
	return nil
}

func (m *mockAdminRepo) HasPermission(_ context.Context, adminID uuid.UUID, role enums.AdminRole, permission string) (bool, error) {
	if overrides, ok := m.permissions[adminID]; ok {
		if g, exists := overrides[permission]; exists {
			return g, nil
		}
	}
	if role == enums.AdminRoleSuperAdmin {
		return true, nil
	}
	if role == enums.AdminRoleSupportL1 {
		return permission == "reservations.read" || permission == "audit.read", nil
	}
	return false, nil
}

func TestRequireAdminPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	clk := clock.NewMockClock(time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC))
	jwtMgr := jwt.NewManager("test-secret", time.Hour, time.Hour, clk)
	adminRepo := newMockAdminRepo()

	superAdminID := uuid.New()
	adminRepo.admins[superAdminID] = models.AdminUser{
		ID:    superAdminID,
		Email: "super@stayhub.dev",
		Role:  enums.AdminRoleSuperAdmin,
	}

	l1AdminID := uuid.New()
	adminRepo.admins[l1AdminID] = models.AdminUser{
		ID:    l1AdminID,
		Email: "l1@stayhub.dev",
		Role:  enums.AdminRoleSupportL1,
	}

	userID := uuid.New()

	router := gin.New()
	router.GET("/admin/refund", middleware.RequireAdminPermission(jwtMgr, adminRepo, "reservations.refund"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// 1. Missing token -> 401
	req, _ := http.NewRequest(http.MethodGet, "/admin/refund", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 on missing token, got %d", w.Code)
	}

	// 2. Regular user token -> 403
	userToken, _, _ := jwtMgr.GenerateUserAccessToken(userID, "user@stayhub.dev", true, false)
	req, _ = http.NewRequest(http.MethodGet, "/admin/refund", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 on user token, got %d", w.Code)
	}

	// 3. L1 admin without refund permission -> 403
	l1Token, _, _ := jwtMgr.GenerateAdminAccessToken(l1AdminID, "l1@stayhub.dev")
	req, _ = http.NewRequest(http.MethodGet, "/admin/refund", nil)
	req.Header.Set("Authorization", "Bearer "+l1Token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 on l1 admin without refund permission, got %d", w.Code)
	}

	// 4. Super admin -> 200
	superToken, _, _ := jwtMgr.GenerateAdminAccessToken(superAdminID, "super@stayhub.dev")
	req, _ = http.NewRequest(http.MethodGet, "/admin/refund", nil)
	req.Header.Set("Authorization", "Bearer "+superToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 on super admin, got %d", w.Code)
	}
}
