package messaging

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/http/middleware"
	"github.com/stayhub/backend/internal/http/response"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	Subprotocols: []string{"access_token", "Bearer"},
}

type Controller struct {
	service interfaces.MessagingService
	hub     *Hub
}

func NewController(service interfaces.MessagingService, hubs ...*Hub) *Controller {
	var h *Hub
	if len(hubs) > 0 && hubs[0] != nil {
		h = hubs[0]
	} else {
		h = NewHub()
	}
	return &Controller{service: service, hub: h}
}

func (ctrl *Controller) Hub() *Hub {
	return ctrl.hub
}

// WebSocketHandler handles GET /conversations/ws and GET /conversations/:id/ws
func (ctrl *Controller) WebSocketHandler(c *gin.Context) {
	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Forbidden(c, "Unauthorized")
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	client := &Client{
		hub:    ctrl.hub,
		ctrl:   ctrl,
		conn:   conn,
		send:   make(chan []byte, 256),
		userID: actor.SubjectID,
	}

	if convIDStr := c.Param("id"); convIDStr != "" {
		if id, err := uuid.Parse(convIDStr); err == nil {
			client.activeConvID = id
		}
	}

	ctrl.hub.Register(client)

	client.sendEvent("connected", map[string]any{
		"user_id": actor.SubjectID,
	})

	go client.writePump()
	go client.readPump()
}

// ListConversations handles GET /conversations
func (ctrl *Controller) ListConversations(c *gin.Context) {
	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Forbidden(c, "Unauthorized")
		return
	}

	res, err := ctrl.service.ListConversations(c.Request.Context(), actor.SubjectID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"conversations": res})
}

// GetOrCreateConversation handles POST /conversations
func (ctrl *Controller) GetOrCreateConversation(c *gin.Context) {
	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Forbidden(c, "Unauthorized")
		return
	}

	var in dtos.CreateConversationInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "INVALID_JSON", "Invalid reservation ID payload", nil)
		return
	}

	conv, err := ctrl.service.GetOrCreateConversation(c.Request.Context(), in.ReservationID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Reservation not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	res, err := ctrl.service.GetConversation(c.Request.Context(), actor.SubjectID, conv.ID)
	if err != nil {
		if errors.Is(err, apperrors.ErrForbidden) {
			response.Forbidden(c, "You do not have access to this conversation")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, res)
}

// GetConversation handles GET /conversations/:id
func (ctrl *Controller) GetConversation(c *gin.Context) {
	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Forbidden(c, "Unauthorized")
		return
	}

	convID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid conversation ID", nil)
		return
	}

	res, err := ctrl.service.GetConversation(c.Request.Context(), actor.SubjectID, convID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Conversation not found")
			return
		}
		if errors.Is(err, apperrors.ErrForbidden) {
			response.Forbidden(c, "You do not have access to this conversation")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, res)
}

// ListMessages handles GET /conversations/:id/messages
func (ctrl *Controller) ListMessages(c *gin.Context) {
	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Forbidden(c, "Unauthorized")
		return
	}

	convID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid conversation ID", nil)
		return
	}

	var before *time.Time
	if beforeStr := c.Query("before"); beforeStr != "" {
		if t, err := time.Parse(time.RFC3339Nano, beforeStr); err == nil {
			before = &t
		} else if t, err := time.Parse(time.RFC3339, beforeStr); err == nil {
			before = &t
		}
	}

	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	res, err := ctrl.service.ListMessages(c.Request.Context(), actor.SubjectID, convID, before, limit)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Conversation not found")
			return
		}
		if errors.Is(err, apperrors.ErrForbidden) {
			response.Forbidden(c, "Forbidden")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, res)
}

// SendMessage handles POST /conversations/:id/messages
func (ctrl *Controller) SendMessage(c *gin.Context) {
	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Forbidden(c, "Unauthorized")
		return
	}

	convID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid conversation ID", nil)
		return
	}

	var in dtos.SendMessageInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "INVALID_JSON", "Invalid message payload", nil)
		return
	}

	res, err := ctrl.service.SendMessage(c.Request.Context(), actor.SubjectID, convID, in)
	if err != nil {
		if errors.Is(err, apperrors.ErrValidation) {
			response.BadRequest(c, "VALIDATION_FAILED", err.Error(), nil)
			return
		}
		if errors.Is(err, apperrors.ErrForbidden) {
			response.Forbidden(c, "Forbidden")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusCreated, res)
}

// MarkRead handles POST /conversations/:id/read
func (ctrl *Controller) MarkRead(c *gin.Context) {
	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Forbidden(c, "Unauthorized")
		return
	}

	convID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "Invalid conversation ID", nil)
		return
	}

	if err := ctrl.service.MarkRead(c.Request.Context(), actor.SubjectID, convID); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Conversation not found")
			return
		}
		if errors.Is(err, apperrors.ErrForbidden) {
			response.Forbidden(c, "Forbidden")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
