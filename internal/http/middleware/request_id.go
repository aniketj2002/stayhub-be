package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/models"
)

const RequestIDHeader = "X-Request-ID"

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader(RequestIDHeader)
		if reqID == "" {
			reqID = uuid.NewString()
		}
		c.Header(RequestIDHeader, reqID)
		c.Set("request_id", reqID)

		ctx := models.WithRequestID(c.Request.Context(), reqID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
