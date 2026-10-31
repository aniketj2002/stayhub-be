package services

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/clock"
)

type memMessagingRepo struct {
	mu            sync.Mutex
	conversations map[uuid.UUID]models.Conversation
	byReservation map[uuid.UUID]uuid.UUID
	messages      map[uuid.UUID]models.Message
	listMessages  []models.Message
	reads         map[string]time.Time
}

func newMemMessagingRepo() *memMessagingRepo {
	return &memMessagingRepo{
		conversations: make(map[uuid.UUID]models.Conversation),
		byReservation: make(map[uuid.UUID]uuid.UUID),
		messages:      make(map[uuid.UUID]models.Message),
		reads:         make(map[string]time.Time),
	}
}

func (m *memMessagingRepo) GetOrCreateConversation(_ context.Context, reservationID, listingID, guestID uuid.UUID) (*models.Conversation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id, ok := m.byReservation[reservationID]; ok {
		c := m.conversations[id]
		return &c, nil
	}
	id := uuid.New()
	conv := models.Conversation{
		ID:            id,
		ReservationID: reservationID,
		ListingID:     listingID,
		GuestID:       guestID,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	m.conversations[id] = conv
	m.byReservation[reservationID] = id
	return &conv, nil
}

func (m *memMessagingRepo) FindConversationByID(_ context.Context, id uuid.UUID) (*models.Conversation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	conv, ok := m.conversations[id]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	return &conv, nil
}

func (m *memMessagingRepo) FindConversationByReservation(_ context.Context, reservationID uuid.UUID) (*models.Conversation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byReservation[reservationID]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	conv := m.conversations[id]
	return &conv, nil
}

func (m *memMessagingRepo) ListConversationsByUser(_ context.Context, userID uuid.UUID) ([]models.Conversation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.Conversation
	for _, c := range m.conversations {
		if c.GuestID == userID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (m *memMessagingRepo) InsertMessage(_ context.Context, msg models.Message) (*models.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if msg.ID == uuid.Nil {
		msg.ID = uuid.New()
	}
	m.messages[msg.ID] = msg
	m.listMessages = append(m.listMessages, msg)
	return &msg, nil
}

func (m *memMessagingRepo) ListMessagesByConversation(_ context.Context, conversationID uuid.UUID, before *time.Time, limit int) ([]models.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.Message
	for _, msg := range m.listMessages {
		if msg.ConversationID == conversationID {
			if before != nil && !msg.CreatedAt.Before(*before) {
				continue
			}
			out = append(out, msg)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (m *memMessagingRepo) GetLatestMessage(_ context.Context, conversationID uuid.UUID) (*models.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.listMessages) - 1; i >= 0; i-- {
		if m.listMessages[i].ConversationID == conversationID {
			msg := m.listMessages[i]
			return &msg, nil
		}
	}
	return nil, nil
}

func (m *memMessagingRepo) MarkConversationRead(_ context.Context, conversationID, userID uuid.UUID, readAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := conversationID.String() + ":" + userID.String()
	m.reads[key] = readAt
	return nil
}

func (m *memMessagingRepo) GetUnreadCount(_ context.Context, conversationID, userID uuid.UUID) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := conversationID.String() + ":" + userID.String()
	readAt := m.reads[key]

	count := 0
	for _, msg := range m.listMessages {
		if msg.ConversationID == conversationID && msg.SenderID != userID && msg.CreatedAt.After(readAt) {
			count++
		}
	}
	return count, nil
}

func TestMessagingService(t *testing.T) {
	now := time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)
	clk := clock.NewMockClock(now)

	guestID := uuid.New()
	hostID := uuid.New()
	cohostAllowedID := uuid.New()
	cohostBlockedID := uuid.New()
	listingID := uuid.New()
	resID := uuid.New()

	users := map[uuid.UUID]models.User{
		guestID:         {ID: guestID, FullName: "Guest User", Email: "guest@example.com"},
		hostID:          {ID: hostID, FullName: "Host User", Email: "host@example.com"},
		cohostAllowedID: {ID: cohostAllowedID, FullName: "Cohost Allowed", Email: "cohost1@example.com"},
		cohostBlockedID: {ID: cohostBlockedID, FullName: "Cohost Blocked", Email: "cohost2@example.com"},
	}

	msgRepo := newMemMessagingRepo()
	resRepo := newMemReservationRepo()
	resRepo.rows[resID] = models.Reservation{
		ID:        resID,
		ListingID: listingID,
		GuestID:   guestID,
		HostID:    hostID,
		Status:    enums.ReservationStatusConfirmed,
	}

	listingRepo := &memListingRepo{
		listings: map[uuid.UUID]models.Listing{
			listingID: {ID: listingID, HostID: hostID},
		},
	}
	listingHostRepo := &memListingHostRepo{
		members: map[string]models.ListingHost{
			listingID.String() + ":" + hostID.String(): {
				ListingID: listingID,
				UserID:    hostID,
				Role:      enums.ListingHostRoleOwner,
			},
			listingID.String() + ":" + cohostAllowedID.String(): {
				ListingID:        listingID,
				UserID:           cohostAllowedID,
				Role:             enums.ListingHostRoleCohost,
				CanMessageGuests: true,
			},
			listingID.String() + ":" + cohostBlockedID.String(): {
				ListingID:        listingID,
				UserID:           cohostBlockedID,
				Role:             enums.ListingHostRoleCohost,
				CanMessageGuests: false,
			},
		},
	}
	userRepo := &memUserRepo{users: users}

	svc := NewMessagingService(nil, msgRepo, resRepo, listingRepo, listingHostRepo, userRepo, clk)

	t.Run("Create conversation for reservation", func(t *testing.T) {
		conv, err := svc.GetOrCreateConversation(context.Background(), resID)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if conv.ReservationID != resID {
			t.Errorf("expected resID %s, got %s", resID, conv.ReservationID)
		}
		if conv.HostName != "Host User" {
			t.Errorf("expected HostName 'Host User', got %q", conv.HostName)
		}
		if conv.GuestName != "Guest User" {
			t.Errorf("expected GuestName 'Guest User', got %q", conv.GuestName)
		}
	})

	t.Run("Send photo-only message and verify non-nil slices", func(t *testing.T) {
		conv, _ := svc.GetOrCreateConversation(context.Background(), resID)
		mediaID := uuid.New()
		resp, err := svc.SendMessage(context.Background(), guestID, conv.ID, dtos.SendMessageInput{
			Body:     "",
			MediaIDs: []uuid.UUID{mediaID},
		})
		if err != nil {
			t.Fatalf("expected photo-only message to succeed, got: %v", err)
		}
		if len(resp.MediaIDs) != 1 || resp.MediaIDs[0] != mediaID {
			t.Errorf("expected MediaIDs [%s], got %v", mediaID, resp.MediaIDs)
		}
		if len(resp.MediaURLs) != 1 {
			t.Errorf("expected 1 MediaURL, got %v", resp.MediaURLs)
		}
		if resp.Flags == nil {
			t.Errorf("expected non-nil Flags slice")
		}
	})

	t.Run("Cohost without can_message_guests is forbidden to send or view", func(t *testing.T) {
		conv, _ := svc.GetOrCreateConversation(context.Background(), resID)
		_, err := svc.GetConversation(context.Background(), cohostBlockedID, conv.ID)
		if err == nil {
			t.Fatal("expected forbidden error for cohost without permissions")
		}

		_, err = svc.SendMessage(context.Background(), cohostBlockedID, conv.ID, dtos.SendMessageInput{
			Body: "Hello",
		})
		if err == nil {
			t.Fatal("expected forbidden error when sending message")
		}
	})

	t.Run("Cohost with can_message_guests is authorized to view conversation", func(t *testing.T) {
		conv, _ := svc.GetOrCreateConversation(context.Background(), resID)
		c, err := svc.GetConversation(context.Background(), cohostAllowedID, conv.ID)
		if err != nil {
			t.Fatalf("expected no error for allowed cohost, got: %v", err)
		}
		if c.ReservationID != resID {
			t.Errorf("expected resID %s, got %s", resID, c.ReservationID)
		}
	})

	t.Run("Send clean message", func(t *testing.T) {
		conv, _ := svc.GetOrCreateConversation(context.Background(), resID)
		resp, err := svc.SendMessage(context.Background(), guestID, conv.ID, dtos.SendMessageInput{
			Body: "Hello host, looking forward to staying at your place!",
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(resp.Flags) > 0 {
			t.Errorf("expected no flags for clean message, got: %v", resp.Flags)
		}
		if resp.Warning != nil {
			t.Errorf("expected no warning for clean message, got: %v", *resp.Warning)
		}
	})

	t.Run("Send message with contact info triggers off-platform warning", func(t *testing.T) {
		conv, _ := svc.GetOrCreateConversation(context.Background(), resID)
		resp, err := svc.SendMessage(context.Background(), guestID, conv.ID, dtos.SendMessageInput{
			Body: "Text me on WhatsApp at +1-555-019-2834 or email secret@gmail.com to pay outside StayHub directly!",
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		hasOffPlatformFlag := false
		for _, f := range resp.Flags {
			if f == "off_platform" {
				hasOffPlatformFlag = true
				break
			}
		}
		if !hasOffPlatformFlag {
			t.Errorf("expected off_platform flag to be present")
		}
		if resp.Warning == nil {
			t.Fatalf("expected warning message to be returned")
		}
		if *resp.Warning == "" {
			t.Errorf("expected non-empty warning message")
		}
	})

	t.Run("Read receipt and unread count", func(t *testing.T) {
		conv, _ := svc.GetOrCreateConversation(context.Background(), resID)

		// Check conversation unread count for host
		c, err := svc.GetConversation(context.Background(), hostID, conv.ID)
		if err != nil {
			t.Fatalf("expected no error getting conv: %v", err)
		}
		if c.UnreadCount != 3 {
			t.Errorf("expected 3 unread messages for host, got %d", c.UnreadCount)
		}

		// Host marks conversation as read
		clk.Advance(1 * time.Minute)
		if err := svc.MarkRead(context.Background(), hostID, conv.ID); err != nil {
			t.Fatalf("expected no error marking read: %v", err)
		}

		// Host checks conversation again -> unread should be 0
		c, err = svc.GetConversation(context.Background(), hostID, conv.ID)
		if err != nil {
			t.Fatalf("expected no error: %v", err)
		}
		if c.UnreadCount != 0 {
			t.Errorf("expected 0 unread messages, got %d", c.UnreadCount)
		}
	})
}
