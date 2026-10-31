package messaging

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stayhub/backend/internal/domain/dtos"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 65536
)

type Client struct {
	hub          *Hub
	ctrl         *Controller
	conn         *websocket.Conn
	send         chan []byte
	sendMu       sync.Mutex
	isClosed     bool
	userID       uuid.UUID
	activeConvID uuid.UUID
}

func (c *Client) trySend(data []byte) {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	if c.isClosed {
		return
	}
	select {
	case c.send <- data:
	default:
		// Queue full; skip or let buffer drain
	}
}

func (c *Client) closeSend() {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	if !c.isClosed {
		c.isClosed = true
		close(c.send)
	}
}

func (c *Client) sendEvent(eventType string, data any) {
	msg, err := json.Marshal(OutgoingEvent{
		Type: eventType,
		Data: data,
	})
	if err == nil {
		c.trySend(msg)
	}
}

func (c *Client) sendError(errMsg string) {
	c.sendEvent("error", map[string]string{
		"error": errMsg,
	})
}

type IncomingAction struct {
	Action         string      `json:"action"`
	ConversationID *uuid.UUID  `json:"conversation_id,omitempty"`
	Body           string      `json:"body,omitempty"`
	MediaIDs       []uuid.UUID `json:"media_ids,omitempty"`
	IsTyping       *bool       `json:"is_typing,omitempty"`
}

func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		var incoming IncomingAction
		if err := json.Unmarshal(raw, &incoming); err != nil {
			c.sendError("invalid JSON payload")
			continue
		}

		c.handleAction(incoming)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			if _, err := w.Write(message); err != nil {
				return
			}
			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleAction(action IncomingAction) {
	ctx := context.Background()

	switch action.Action {
	case "ping":
		c.sendEvent("pong", map[string]string{"status": "ok"})

	case "join", "subscribe":
		if action.ConversationID != nil {
			c.hub.SetActiveConversation(c, *action.ConversationID)
			c.sendEvent("subscribed", map[string]any{"conversation_id": *action.ConversationID})
		}

	case "leave", "unsubscribe":
		c.hub.SetActiveConversation(c, uuid.Nil)
		c.sendEvent("unsubscribed", map[string]any{})

	case "send_message":
		if action.ConversationID == nil {
			c.sendError("missing conversation_id")
			return
		}
		res, err := c.ctrl.service.SendMessage(ctx, c.userID, *action.ConversationID, dtos.SendMessageInput{
			Body:     action.Body,
			MediaIDs: action.MediaIDs,
		})
		if err != nil {
			c.sendError(err.Error())
			return
		}
		c.sendEvent("message_sent", res)

	case "mark_read":
		if action.ConversationID == nil {
			c.sendError("missing conversation_id")
			return
		}
		if err := c.ctrl.service.MarkRead(ctx, c.userID, *action.ConversationID); err != nil {
			c.sendError(err.Error())
			return
		}
		c.sendEvent("read_confirmed", map[string]any{"conversation_id": *action.ConversationID})

	case "typing":
		if action.ConversationID == nil {
			return
		}
		isTyping := true
		if action.IsTyping != nil {
			isTyping = *action.IsTyping
		}
		participantIDs, err := c.ctrl.service.GetConversationParticipantIDs(ctx, *action.ConversationID)
		if err == nil {
			c.hub.BroadcastTyping(*action.ConversationID, c.userID, isTyping, participantIDs)
		}
	}
}
