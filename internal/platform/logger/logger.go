package logger

import (
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/domain/models"
)

func NewJSONLogger(appEnv string) *slog.Logger {
	level := slog.LevelInfo
	if appEnv == "development" || appEnv == "local" || appEnv == "test" {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}
	handler := slog.NewJSONHandler(os.Stdout, opts)
	return slog.New(handler)
}

func RequestLogger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)

		status := c.Writer.Status()
		reqID := c.Writer.Header().Get("X-Request-ID")
		if reqID == "" {
			reqID = c.GetString("request_id")
		}

		var actorType string
		var actorID string
		if actor, ok := models.AuthActorFromContext(c.Request.Context()); ok {
			actorType = string(actor.DefaultActorType())
			actorID = actor.SubjectID.String()
		}

		attrs := []slog.Attr{
			slog.String("request_id", reqID),
			slog.String("method", c.Request.Method),
			slog.String("route", c.FullPath()),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", status),
			slog.Int64("latency_ms", latency.Milliseconds()),
		}
		if actorType != "" {
			attrs = append(attrs, slog.String("actor_type", actorType))
			attrs = append(attrs, slog.String("actor_id", actorID))
		}

		ctx := c.Request.Context()
		switch {
		case status >= 500:
			log.LogAttrs(ctx, slog.LevelError, "http.5xx", attrs...)
		case status >= 400:
			log.LogAttrs(ctx, slog.LevelWarn, "http.4xx", attrs...)
		default:
			log.LogAttrs(ctx, slog.LevelInfo, "http.request", attrs...)
		}
	}
}
