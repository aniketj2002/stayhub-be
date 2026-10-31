package messaging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/controllers/messaging"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/models"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type mockMessagingService struct {
	getOrCreateFn func(ctx context.Context, resID uuid.UUID) (*dtos.ConversationResponse, error)
	getConvFn     func(ctx context.Context, userID, convID uuid.UUID) (*dtos.ConversationResponse, error)
	listConvFn    func(ctx context.Context, userID uuid.UUID) ([]dtos.ConversationResponse, error)
	listMsgFn     func(ctx context.Context, userID, convID uuid.UUID, before *time.Time, limit int) (*dtos.MessageListResponse, error)
	sendMsgFn     func(ctx context.Context, senderID, convID uuid.UUID, in dtos.SendMessageInput) (*dtos.MessageResponse, error)
	markReadFn    func(ctx context.Context, userID, convID uuid.UUID) error
}

func (m *mockMessagingService) GetOrCreateConversation(ctx context.Context, reservationID uuid.UUID) (*dtos.ConversationResponse, error) {
	if m.getOrCreateFn != nil {
		return m.getOrCreateFn(ctx, reservationID)
	}
	return nil, nil
}

func (m *mockMessagingService) ListConversations(ctx context.Context, userID uuid.UUID) ([]dtos.ConversationResponse, error) {
	if m.listConvFn != nil {
		return m.listConvFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockMessagingService) GetConversation(ctx context.Context, userID, conversationID uuid.UUID) (*dtos.ConversationResponse, error) {
	if m.getConvFn != nil {
		return m.getConvFn(ctx, userID, conversationID)
	}
	return nil, nil
}

func (m *mockMessagingService) ListMessages(ctx context.Context, userID, conversationID uuid.UUID, before *time.Time, limit int) (*dtos.MessageListResponse, error) {
	if m.listMsgFn != nil {
		return m.listMsgFn(ctx, userID, conversationID, before, limit)
	}
	return nil, nil
}

func (m *mockMessagingService) SendMessage(ctx context.Context, senderID, conversationID uuid.UUID, in dtos.SendMessageInput) (*dtos.MessageResponse, error) {
	if m.sendMsgFn != nil {
		return m.sendMsgFn(ctx, senderID, conversationID, in)
	}
	return nil, nil
}

func (m *mockMessagingService) MarkRead(ctx context.Context, userID, conversationID uuid.UUID) error {
	if m.markReadFn != nil {
		return m.markReadFn(ctx, userID, conversationID)
	}
	return nil
}

func (m *mockMessagingService) GetConversationParticipantIDs(ctx context.Context, conversationID uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}

func TestGetOrCreateConversation(t *testing.T) {
	actorID := uuid.New()
	resID := uuid.New()
	convID := uuid.New()

	mockSvc := &mockMessagingService{
		getOrCreateFn: func(ctx context.Context, rID uuid.UUID) (*dtos.ConversationResponse, error) {
			if rID != resID {
				return nil, apperrors.ErrNotFound
			}
			return &dtos.ConversationResponse{
				ID:            convID,
				ReservationID: rID,
				ListingTitle:  "Lisbon Townhouse",
			}, nil
		},
		getConvFn: func(ctx context.Context, uID, cID uuid.UUID) (*dtos.ConversationResponse, error) {
			if uID != actorID {
				return nil, apperrors.ErrForbidden
			}
			return &dtos.ConversationResponse{
				ID:            convID,
				ReservationID: resID,
				ListingTitle:  "Lisbon Townhouse",
			}, nil
		},
	}

	ctrl := messaging.NewController(mockSvc)

	t.Run("success", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body, _ := json.Marshal(dtos.CreateConversationInput{ReservationID: resID})
		req := httptest.NewRequest(http.MethodPost, "/conversations", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		actor := models.AuthActor{SubjectID: actorID, SubjectType: "user"}
		req = req.WithContext(models.WithAuthActor(req.Context(), actor))
		c.Request = req

		ctrl.GetOrCreateConversation(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d (body: %s)", w.Code, w.Body.String())
		}
	})

	t.Run("missing reservation_id", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest(http.MethodPost, "/conversations", bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		actor := models.AuthActor{SubjectID: actorID, SubjectType: "user"}
		req = req.WithContext(models.WithAuthActor(req.Context(), actor))
		c.Request = req

		ctrl.GetOrCreateConversation(c)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("reservation not found", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body, _ := json.Marshal(dtos.CreateConversationInput{ReservationID: uuid.New()})
		req := httptest.NewRequest(http.MethodPost, "/conversations", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		actor := models.AuthActor{SubjectID: actorID, SubjectType: "user"}
		req = req.WithContext(models.WithAuthActor(req.Context(), actor))
		c.Request = req

		ctrl.GetOrCreateConversation(c)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", w.Code)
		}
	})

	t.Run("unauthorized actor context missing", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body, _ := json.Marshal(dtos.CreateConversationInput{ReservationID: resID})
		c.Request = httptest.NewRequest(http.MethodPost, "/conversations", bytes.NewBuffer(body))
		c.Request.Header.Set("Content-Type", "application/json")

		ctrl.GetOrCreateConversation(c)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d", w.Code)
		}
	})
}
