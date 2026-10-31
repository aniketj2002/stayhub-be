package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/models"
	platformaws "github.com/stayhub/backend/internal/platform/aws"
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/db"
)

type memCohostRepo struct {
	mu          sync.Mutex
	invites     map[uuid.UUID]models.CohostInvitation
	byTokenHash map[string]uuid.UUID
	splits      map[uuid.UUID][]models.PayoutSplit
}

func newMemCohostRepo() *memCohostRepo {
	return &memCohostRepo{
		invites:     make(map[uuid.UUID]models.CohostInvitation),
		byTokenHash: make(map[string]uuid.UUID),
		splits:      make(map[uuid.UUID][]models.PayoutSplit),
	}
}

func (m *memCohostRepo) CreateInvitation(_ context.Context, inv models.CohostInvitation) (*models.CohostInvitation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if inv.ID == uuid.Nil {
		inv.ID = uuid.New()
	}
	m.invites[inv.ID] = inv
	m.byTokenHash[inv.TokenHash] = inv.ID
	return &inv, nil
}

func (m *memCohostRepo) FindInvitationByTokenHash(_ context.Context, tokenHash string) (*models.CohostInvitation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byTokenHash[tokenHash]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	inv := m.invites[id]
	return &inv, nil
}

func (m *memCohostRepo) ListInvitationsByListing(_ context.Context, listingID uuid.UUID) ([]models.CohostInvitation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.CohostInvitation
	for _, inv := range m.invites {
		if inv.ListingID == listingID {
			out = append(out, inv)
		}
	}
	return out, nil
}

func (m *memCohostRepo) UpdateInvitationStatus(_ context.Context, id uuid.UUID, status enums.CohostInviteStatus, _ time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	inv, ok := m.invites[id]
	if !ok {
		return apperrors.ErrNotFound
	}
	inv.Status = status
	m.invites[id] = inv
	return nil
}

func (m *memCohostRepo) FindInvitationByID(_ context.Context, id uuid.UUID) (*models.CohostInvitation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	inv, ok := m.invites[id]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	return &inv, nil
}

func (m *memCohostRepo) CreatePayoutSplits(_ context.Context, splits []models.PayoutSplit) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range splits {
		m.splits[s.PayoutID] = append(m.splits[s.PayoutID], s)
	}
	return nil
}

func (m *memCohostRepo) ListPayoutSplits(_ context.Context, payoutID uuid.UUID) ([]models.PayoutSplit, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.splits[payoutID], nil
}

func (m *memCohostRepo) UpdatePayoutSplitsStatus(_ context.Context, payoutID uuid.UUID, status enums.PayoutSplitStatus, _ time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	splits := m.splits[payoutID]
	for i := range splits {
		splits[i].Status = status
	}
	m.splits[payoutID] = splits
	return nil
}

func TestCohostService_InviteAndAccept(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	mockClk := clock.NewMockClock(now)

	cohostRepo := newMemCohostRepo()
	hostRepo := &memListingHostRepo{members: make(map[string]models.ListingHost)}
	userRepo := &memUserRepo{users: make(map[uuid.UUID]models.User)}

	listingID := uuid.New()
	primaryHostID := uuid.New()
	cohostUserID := uuid.New()

	listingTitle := "Chic Waterfront Condo"
	verID := uuid.New()
	listingRepo := &memListingRepo{
		listings: map[uuid.UUID]models.Listing{
			listingID: {ID: listingID, LiveVersionID: &verID},
		},
		versions: map[uuid.UUID]models.ListingVersion{
			verID: {ID: verID, Title: &listingTitle},
		},
	}

	svc := NewCohostService(&db.DB{}, cohostRepo, hostRepo, userRepo, listingRepo, mockClk)
	ctx := context.Background()

	// Setup primary host
	hostRepo.members[listingID.String()+":"+primaryHostID.String()] = models.ListingHost{
		ListingID:      listingID,
		UserID:         primaryHostID,
		Role:           enums.ListingHostRoleOwner,
		PayoutShareBPS: 0,
	}

	// Setup cohost user
	userRepo.users[cohostUserID] = models.User{
		ID:     cohostUserID,
		Email:  "cohost@example.com",
		IsHost: false,
	}

	// 1. Validation error: invalid email
	_, _, err := svc.InviteCohost(ctx, primaryHostID, listingID, dtos.CreateCohostInviteInput{
		Email: "invalid-email",
	})
	if err == nil {
		t.Fatalf("expected error for invalid email")
	}

	// 2. Validation error: payout share > 10000
	_, _, err = svc.InviteCohost(ctx, primaryHostID, listingID, dtos.CreateCohostInviteInput{
		Email:          "cohost@example.com",
		PayoutShareBPS: 10001,
	})
	if err == nil {
		t.Fatalf("expected error for payout share > 10000")
	}

	// 3. Valid invitation creation
	inviteResp, rawToken, err := svc.InviteCohost(ctx, primaryHostID, listingID, dtos.CreateCohostInviteInput{
		Email:          "cohost@example.com",
		PayoutShareBPS: 2000, // 20%
		Permissions: dtos.CohostPermissions{
			CanManageListing:      true,
			CanManageCalendar:     true,
			CanManageReservations: true,
			CanMessageGuests:      true,
			CanViewInsights:       false,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error creating invite: %v", err)
	}
	if rawToken == "" {
		t.Fatalf("expected non-empty raw token")
	}
	if inviteResp.Status != string(enums.CohostInvitePending) {
		t.Errorf("expected pending status, got %s", inviteResp.Status)
	}
	if inviteResp.PayoutShareBPS != 2000 {
		t.Errorf("expected 2000 BPS, got %d", inviteResp.PayoutShareBPS)
	}

	// 3b. Read-only metadata check for landing screen
	details, err := svc.GetInvitationDetails(ctx, rawToken)
	if err != nil {
		t.Fatalf("expected no error getting invitation details: %v", err)
	}
	if details.ListingTitle != "Chic Waterfront Condo" {
		t.Errorf("expected listing title 'Chic Waterfront Condo', got '%s'", details.ListingTitle)
	}
	if details.InvitedEmail != "cohost@example.com" {
		t.Errorf("expected invited email 'cohost@example.com', got '%s'", details.InvitedEmail)
	}
	if details.PayoutShareBPS != 2000 {
		t.Errorf("expected payout share 2000, got %d", details.PayoutShareBPS)
	}

	// Verify token hash stored matches SHA256 of rawToken
	h := sha256.Sum256([]byte(rawToken))
	expectedHash := hex.EncodeToString(h[:])
	storedInv, err := cohostRepo.FindInvitationByTokenHash(ctx, expectedHash)
	if err != nil {
		t.Fatalf("failed to find invitation by hash: %v", err)
	}
	if storedInv.TokenHash != expectedHash {
		t.Errorf("expected hash %s, got %s", expectedHash, storedInv.TokenHash)
	}

	// 4. Accept invitation with wrong email returns forbidden
	wrongUser := models.User{
		ID:    uuid.New(),
		Email: "stranger@example.com",
	}
	_, err = svc.AcceptInvitation(ctx, wrongUser, rawToken)
	if err == nil {
		t.Fatalf("expected forbidden error for mismatched email")
	}

	// 5. Accept invitation with matching user succeeds
	acceptResp, err := svc.AcceptInvitation(ctx, userRepo.users[cohostUserID], rawToken)
	if err != nil {
		t.Fatalf("unexpected error accepting invite: %v", err)
	}
	if acceptResp.Status != "accepted" {
		t.Errorf("expected accepted status, got %s", acceptResp.Status)
	}

	// Verify user was promoted to host
	u := userRepo.users[cohostUserID]
	if !u.IsHost {
		t.Errorf("expected user to be promoted to host")
	}

	// Verify listing_hosts has cohost
	lhKey := listingID.String() + ":" + cohostUserID.String()
	lh, exists := hostRepo.members[lhKey]
	if !exists {
		t.Fatalf("expected cohost in listing_hosts")
	}
	if lh.Role != enums.ListingHostRoleCohost {
		t.Errorf("expected cohost role, got %s", lh.Role)
	}
	if lh.PayoutShareBPS != 2000 {
		t.Errorf("expected payout share 2000, got %d", lh.PayoutShareBPS)
	}
	if !lh.CanManageListing || !lh.CanManageCalendar || !lh.CanManageReservations {
		t.Errorf("expected permissions to be set")
	}

	// Verify invitation status updated to accepted
	updatedInv, _ := cohostRepo.FindInvitationByID(ctx, storedInv.ID)
	if updatedInv.Status != enums.CohostInviteAccepted {
		t.Errorf("expected invitation status accepted, got %s", updatedInv.Status)
	}

	// 6. Cannot accept already accepted invitation
	_, err = svc.AcceptInvitation(ctx, userRepo.users[cohostUserID], rawToken)
	if err == nil {
		t.Fatalf("expected error accepting already accepted invitation")
	}
}

func TestCohostService_UpdateAndRemoveCohost(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	mockClk := clock.NewMockClock(now)

	cohostRepo := newMemCohostRepo()
	hostRepo := &memListingHostRepo{members: make(map[string]models.ListingHost)}
	userRepo := &memUserRepo{users: make(map[uuid.UUID]models.User)}

	listingID := uuid.New()
	cohostID := uuid.New()

	listingTitle := "Chic Waterfront Condo"
	verID := uuid.New()
	listingRepo := &memListingRepo{
		listings: map[uuid.UUID]models.Listing{
			listingID: {ID: listingID, LiveVersionID: &verID},
		},
		versions: map[uuid.UUID]models.ListingVersion{
			verID: {ID: verID, Title: &listingTitle},
		},
	}

	svc := NewCohostService(&db.DB{}, cohostRepo, hostRepo, userRepo, listingRepo, mockClk)
	ctx := context.Background()

	hostRepo.members[listingID.String()+":"+cohostID.String()] = models.ListingHost{
		ListingID:      listingID,
		UserID:         cohostID,
		Role:           enums.ListingHostRoleCohost,
		PayoutShareBPS: 1500,
	}

	// Update cohost share to 30% (3000 BPS)
	newShare := 3000
	upResp, err := svc.UpdateCohost(ctx, listingID, cohostID, dtos.UpdateCohostInput{
		PayoutShareBPS: &newShare,
	})
	if err != nil {
		t.Fatalf("failed to update cohost: %v", err)
	}
	if upResp.PayoutShareBPS != 3000 {
		t.Errorf("expected 3000 BPS, got %d", upResp.PayoutShareBPS)
	}

	// Remove cohost
	if err := svc.RemoveCohost(ctx, listingID, cohostID); err != nil {
		t.Fatalf("failed to remove cohost: %v", err)
	}
	cohosts, err := svc.ListCohosts(ctx, listingID)
	if err != nil {
		t.Fatalf("failed to list cohosts: %v", err)
	}
	if len(cohosts) != 0 {
		t.Errorf("expected 0 cohosts after removal, got %d", len(cohosts))
	}
}

type mockEmailSender struct {
	mu        sync.Mutex
	sentCalls []platformaws.CohostInvitationEmailInput
}

func (m *mockEmailSender) SendCohostInvitation(_ context.Context, in platformaws.CohostInvitationEmailInput) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentCalls = append(m.sentCalls, in)
	return nil
}

func (m *mockEmailSender) SenderEmail() string {
	return platformaws.DefaultSESSenderEmail
}

func TestCohostService_InviteCohost_SendsEmailViaSES(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	mockClk := clock.NewMockClock(now)

	cohostRepo := newMemCohostRepo()
	hostRepo := &memListingHostRepo{members: make(map[string]models.ListingHost)}
	userRepo := &memUserRepo{users: make(map[uuid.UUID]models.User)}

	listingID := uuid.New()
	primaryHostID := uuid.New()
	userRepo.users[primaryHostID] = models.User{
		ID:       primaryHostID,
		Email:    "ines.costa@stayhub.dev",
		FullName: "Inês Costa",
	}

	listingTitle := "Luxury Lisbon Penthouse"
	verID := uuid.New()
	listingRepo := &memListingRepo{
		listings: map[uuid.UUID]models.Listing{
			listingID: {ID: listingID, LiveVersionID: &verID},
		},
		versions: map[uuid.UUID]models.ListingVersion{
			verID: {ID: verID, Title: &listingTitle},
		},
	}

	mockSender := &mockEmailSender{}
	svc := NewCohostService(&db.DB{}, cohostRepo, hostRepo, userRepo, listingRepo, mockClk, mockSender)
	if cs, ok := svc.(*CohostService); ok {
		cs.SetEmailConfig(mockSender, "https://stayhub.dev")
	}

	ctx := context.Background()
	inviteResp, rawToken, err := svc.InviteCohost(ctx, primaryHostID, listingID, dtos.CreateCohostInviteInput{
		Email:          "partner@example.com",
		PayoutShareBPS: 1500,
		Permissions: dtos.CohostPermissions{
			CanManageListing:      true,
			CanManageCalendar:     true,
			CanManageReservations: true,
			CanMessageGuests:      true,
			CanViewInsights:       false,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error inviting cohost: %v", err)
	}
	if inviteResp == nil || rawToken == "" {
		t.Fatalf("expected non-empty invite response and raw token")
	}

	if len(mockSender.sentCalls) != 1 {
		t.Fatalf("expected exactly 1 email sent, got %d", len(mockSender.sentCalls))
	}

	call := mockSender.sentCalls[0]
	if call.RecipientEmail != "partner@example.com" {
		t.Errorf("expected recipient partner@example.com, got %s", call.RecipientEmail)
	}
	if call.HostName != "Inês Costa" {
		t.Errorf("expected host name Inês Costa, got %s", call.HostName)
	}
	if call.ListingTitle != "Luxury Lisbon Penthouse" {
		t.Errorf("expected listing title Luxury Lisbon Penthouse, got %s", call.ListingTitle)
	}
	if call.PayoutShareBPS != 1500 {
		t.Errorf("expected payout share 1500, got %d", call.PayoutShareBPS)
	}
	expectedURL := "https://stayhub.dev/cohost-invitations/" + rawToken
	if call.InviteURL != expectedURL {
		t.Errorf("expected invite URL %s, got %s", expectedURL, call.InviteURL)
	}
	if mockSender.SenderEmail() != "aniketjindal2002@gmail.com" {
		t.Errorf("expected sender email aniketjindal2002@gmail.com, got %s", mockSender.SenderEmail())
	}
}
