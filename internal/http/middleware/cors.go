package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowedSet := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowedSet[o] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if _, ok := allowedSet[origin]; ok || len(allowedSet) == 0 {
			if origin != "" {
				c.Header("Access-Control-Allow-Origin", origin)
			} else {
				c.Header("Access-Control-Allow-Origin", "*")
			}
		}
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key, X-Request-ID")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
