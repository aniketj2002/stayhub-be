package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/models"
)

type MessagingRepository interface {
	GetOrCreateConversation(ctx context.Context, reservationID, listingID, guestID uuid.UUID) (*models.Conversation, error)
	FindConversationByID(ctx context.Context, id uuid.UUID) (*models.Conversation, error)
	FindConversationByReservation(ctx context.Context, reservationID uuid.UUID) (*models.Conversation, error)
	ListConversationsByUser(ctx context.Context, userID uuid.UUID) ([]models.Conversation, error)
	InsertMessage(ctx context.Context, msg models.Message) (*models.Message, error)
	ListMessagesByConversation(ctx context.Context, conversationID uuid.UUID, before *time.Time, limit int) ([]models.Message, error)
	GetLatestMessage(ctx context.Context, conversationID uuid.UUID) (*models.Message, error)
	GetUnreadCount(ctx context.Context, conversationID, userID uuid.UUID) (int, error)
	MarkConversationRead(ctx context.Context, conversationID, userID uuid.UUID, readAt time.Time) error
}

type MessageBroadcaster interface {
	BroadcastNewMessage(ctx context.Context, convID uuid.UUID, msg dtos.MessageResponse, participantIDs []uuid.UUID)
	BroadcastConversationRead(ctx context.Context, convID, userID uuid.UUID, participantIDs []uuid.UUID)
}

type MessagingService interface {
	GetOrCreateConversation(ctx context.Context, reservationID uuid.UUID) (*dtos.ConversationResponse, error)
	ListConversations(ctx context.Context, userID uuid.UUID) ([]dtos.ConversationResponse, error)
	GetConversation(ctx context.Context, userID, conversationID uuid.UUID) (*dtos.ConversationResponse, error)
	ListMessages(ctx context.Context, userID, conversationID uuid.UUID, before *time.Time, limit int) (*dtos.MessageListResponse, error)
	SendMessage(ctx context.Context, senderID, conversationID uuid.UUID, in dtos.SendMessageInput) (*dtos.MessageResponse, error)
	MarkRead(ctx context.Context, userID, conversationID uuid.UUID) error
	GetConversationParticipantIDs(ctx context.Context, conversationID uuid.UUID) ([]uuid.UUID, error)
}
