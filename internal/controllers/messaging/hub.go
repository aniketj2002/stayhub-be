package messaging

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/dtos"
)

type OutgoingEvent struct {
	Type string `json:"type"`
	Data any    `json:"data,omitempty"`
}

type Hub struct {
	mu          sync.RWMutex
	userClients map[uuid.UUID]map[*Client]bool
	convClients map[uuid.UUID]map[*Client]bool
}

func NewHub() *Hub {
	return &Hub{
		userClients: make(map[uuid.UUID]map[*Client]bool),
		convClients: make(map[uuid.UUID]map[*Client]bool),
	}
}

func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.userClients[c.userID] == nil {
		h.userClients[c.userID] = make(map[*Client]bool)
	}
	h.userClients[c.userID][c] = true

	if c.activeConvID != uuid.Nil {
		if h.convClients[c.activeConvID] == nil {
			h.convClients[c.activeConvID] = make(map[*Client]bool)
		}
		h.convClients[c.activeConvID][c] = true
	}
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.userClients[c.userID]; ok {
		delete(clients, c)
		if len(clients) == 0 {
			delete(h.userClients, c.userID)
		}
	}

	if c.activeConvID != uuid.Nil {
		if clients, ok := h.convClients[c.activeConvID]; ok {
			delete(clients, c)
			if len(clients) == 0 {
				delete(h.convClients, c.activeConvID)
			}
		}
	}

	c.closeSend()
}

func (h *Hub) SetActiveConversation(c *Client, convID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if c.activeConvID != uuid.Nil && c.activeConvID != convID {
		if clients, ok := h.convClients[c.activeConvID]; ok {
			delete(clients, c)
			if len(clients) == 0 {
				delete(h.convClients, c.activeConvID)
			}
		}
	}

	c.activeConvID = convID

	if convID != uuid.Nil {
		if h.convClients[convID] == nil {
			h.convClients[convID] = make(map[*Client]bool)
		}
		h.convClients[convID][c] = true
	}
}

func (h *Hub) BroadcastNewMessage(_ context.Context, _ uuid.UUID, msg dtos.MessageResponse, participantIDs []uuid.UUID) {
	payload := OutgoingEvent{
		Type: "new_message",
		Data: msg,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	h.broadcastToUsers(participantIDs, data)
}

func (h *Hub) BroadcastConversationRead(_ context.Context, convID, userID uuid.UUID, participantIDs []uuid.UUID) {
	payload := OutgoingEvent{
		Type: "conversation_read",
		Data: map[string]any{
			"conversation_id": convID,
			"user_id":         userID,
			"read_at":         time.Now().UTC().Format(time.RFC3339Nano),
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	h.broadcastToUsers(participantIDs, data)
}

func (h *Hub) BroadcastTyping(convID, senderID uuid.UUID, isTyping bool, participantIDs []uuid.UUID) {
	payload := OutgoingEvent{
		Type: "typing",
		Data: map[string]any{
			"conversation_id": convID,
			"user_id":         senderID,
			"is_typing":       isTyping,
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	// Send typing status to other participants (not the sender)
	recipients := make([]uuid.UUID, 0, len(participantIDs))
	for _, p := range participantIDs {
		if p != senderID {
			recipients = append(recipients, p)
		}
	}
	h.broadcastToUsers(recipients, data)
}

func (h *Hub) broadcastToUsers(userIDs []uuid.UUID, data []byte) {
	h.mu.RLock()
	var targets []*Client
	for _, uID := range userIDs {
		for c := range h.userClients[uID] {
			targets = append(targets, c)
		}
	}
	h.mu.RUnlock()

	for _, c := range targets {
		c.trySend(data)
	}
}

func (h *Hub) UserClientCount(userID uuid.UUID) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.userClients[userID])
}

func (h *Hub) TotalClients() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	total := 0
	for _, clients := range h.userClients {
		total += len(clients)
	}
	return total
}
