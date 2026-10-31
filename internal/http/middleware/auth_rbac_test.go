package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/jwt"
)

type fakeUserRepo struct {
	users map[uuid.UUID]*models.User
}

func (f *fakeUserRepo) Create(_ context.Context, _ dtos.CreateUserParams) (*models.User, error) {
	return nil, nil
}
func (f *fakeUserRepo) FindByID(_ context.Context, id uuid.UUID) (*models.User, error) {
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return nil, apperrors.ErrNotFound
}
func (f *fakeUserRepo) FindByEmail(_ context.Context, _ string) (*models.User, error) {
	return nil, apperrors.ErrNotFound
}
func (f *fakeUserRepo) EnableHostRole(_ context.Context, _ uuid.UUID, _ time.Time) (*models.User, error) {
	return nil, nil
}
func (f *fakeUserRepo) UpdateProfile(_ context.Context, _ uuid.UUID, _ string, _ *uuid.UUID, _ time.Time) (*models.User, error) {
	return nil, nil
}

type fakeListingHostRepo struct {
	records map[string]*models.ListingHost
}

func key(listingID, userID uuid.UUID) string {
	return listingID.String() + ":" + userID.String()
}

func (f *fakeListingHostRepo) FindByListingAndUser(_ context.Context, listingID, userID uuid.UUID) (*models.ListingHost, error) {
	if lh, ok := f.records[key(listingID, userID)]; ok {
		return lh, nil
	}
	return nil, apperrors.ErrNotFound
}
func (f *fakeListingHostRepo) ListByListing(_ context.Context, _ uuid.UUID) ([]models.ListingHost, error) {
	return nil, nil
}
func (f *fakeListingHostRepo) ListByUser(_ context.Context, _ uuid.UUID) ([]models.ListingHost, error) {
	return nil, nil
}
func (f *fakeListingHostRepo) CreateOwner(_ context.Context, _, _ uuid.UUID, _ []byte) (*models.ListingHost, error) {
	return nil, nil
}
func (f *fakeListingHostRepo) UpdateConfig(_ context.Context, _, _ uuid.UUID, _ []byte) (*models.ListingHost, error) {
	return nil, nil
}
func (f *fakeListingHostRepo) CreateCohost(_ context.Context, _ models.ListingHost) (*models.ListingHost, error) {
	return nil, nil
}
func (f *fakeListingHostRepo) UpdateCohost(_ context.Context, _ models.ListingHost) (*models.ListingHost, error) {
	return nil, nil
}
func (f *fakeListingHostRepo) DeleteCohost(_ context.Context, _, _ uuid.UUID) error {
	return nil
}

func TestAuthAndListingRBACMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	clk := clock.NewMockClock(time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC))
	jwtMgr := jwt.NewManager("test-secret", 30*time.Minute, 24*time.Hour, clk)

	ownerID := uuid.New()
	cohostID := uuid.New()
	outsiderID := uuid.New()
	listingID := uuid.New()

	userRepo := &fakeUserRepo{
		users: map[uuid.UUID]*models.User{
			ownerID:    {ID: ownerID, Email: "owner@stayhub.dev", IsGuest: true, IsHost: true},
			cohostID:   {ID: cohostID, Email: "cohost@stayhub.dev", IsGuest: true, IsHost: true},
			outsiderID: {ID: outsiderID, Email: "guest@stayhub.dev", IsGuest: true, IsHost: false},
		},
	}

	lhRepo := &fakeListingHostRepo{
		records: map[string]*models.ListingHost{
			key(listingID, ownerID): {
				ID:                    uuid.New(),
				ListingID:             listingID,
				UserID:                ownerID,
				Role:                  enums.ListingHostRoleOwner,
				CanManageListing:      true,
				CanManageCalendar:     true,
				CanManageReservations: true,
				CanMessageGuests:      true,
				CanViewInsights:       true,
				PayoutShareBPS:        10000,
				Config:                json.RawMessage(`{}`),
			},
			key(listingID, cohostID): {
				ID:                    uuid.New(),
				ListingID:             listingID,
				UserID:                cohostID,
				Role:                  enums.ListingHostRoleCohost,
				CanManageListing:      false,
				CanManageCalendar:     true,
				CanManageReservations: true,
				CanMessageGuests:      true,
				CanViewInsights:       false,
				PayoutShareBPS:        1500,
				Config:                json.RawMessage(`{}`),
			},
		},
	}

	r := gin.New()
	r.PUT(
		"/api/v1/host/listings/:id/calendar/block",
		RequireHost(jwtMgr, userRepo),
		RequireListingPermission(lhRepo, "id", PermManageCalendar),
		func(c *gin.Context) {
			lh, _ := ListingHostFromGin(c)
			c.JSON(http.StatusOK, gin.H{"actor_type": lh.ActorType()})
		},
	)
	r.PATCH(
		"/api/v1/host/listings/:id/working-version",
		RequireHost(jwtMgr, userRepo),
		RequireListingPermission(lhRepo, "id", PermManageListing),
		func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		},
	)

	ownerTok, _, _ := jwtMgr.GenerateUserAccessToken(ownerID, "owner@stayhub.dev", true, true)
	cohostTok, _, _ := jwtMgr.GenerateUserAccessToken(cohostID, "cohost@stayhub.dev", true, false) // token has is_host=false, DB has is_host=true
	outsiderTok, _, _ := jwtMgr.GenerateUserAccessToken(outsiderID, "guest@stayhub.dev", true, false)

	// 1. Co-host can manage calendar (also verifies RequireHost DB fallback when token had is_host=false)
	req1 := httptest.NewRequest(http.MethodPut, "/api/v1/host/listings/"+listingID.String()+"/calendar/block", nil)
	req1.Header.Set("Authorization", "Bearer "+cohostTok)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for co-host calendar permission, got %d: %s", w1.Code, w1.Body.String())
	}

	// 2. Co-host without can_manage_listing is rejected with 403 on working-version edit
	req2 := httptest.NewRequest(http.MethodPatch, "/api/v1/host/listings/"+listingID.String()+"/working-version", nil)
	req2.Header.Set("Authorization", "Bearer "+cohostTok)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for co-host lacking can_manage_listing, got %d", w2.Code)
	}

	// 3. Owner is allowed on working-version edit
	req3 := httptest.NewRequest(http.MethodPatch, "/api/v1/host/listings/"+listingID.String()+"/working-version", nil)
	req3.Header.Set("Authorization", "Bearer "+ownerTok)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for owner, got %d", w3.Code)
	}

	// 4. Guest without host role is rejected with 403 HOST_ROLE_REQUIRED
	req4 := httptest.NewRequest(http.MethodPut, "/api/v1/host/listings/"+listingID.String()+"/calendar/block", nil)
	req4.Header.Set("Authorization", "Bearer "+outsiderTok)
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, req4)
	if w4.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for non-host user, got %d", w4.Code)
	}
}
