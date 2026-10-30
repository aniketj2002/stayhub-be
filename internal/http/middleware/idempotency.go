package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stayhub/backend/internal/http/response"
	"github.com/stayhub/backend/internal/platform/db"
	"github.com/stayhub/backend/internal/platform/redis"
)

const IdempotencyHeader = "Idempotency-Key"

type bodyCaptureWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *bodyCaptureWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// RequireIdempotency enforces an Idempotency-Key header on state/money mutating endpoints.
// It uses Redis for an in-flight distributed lock and PostgreSQL idempotency_keys for durable replay.
func RequireIdempotency(database *db.DB, rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		idemKey := c.GetHeader(IdempotencyHeader)
		if idemKey == "" {
			response.BadRequest(c, "MISSING_IDEMPOTENCY_KEY", "Idempotency-Key header is required for this operation", nil)
			return
		}

		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			response.BadRequest(c, "INVALID_REQUEST_BODY", "Failed to read request body", nil)
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		hasher := sha256.New()
		hasher.Write([]byte(c.Request.Method + ":" + c.Request.URL.Path + ":"))
		hasher.Write(bodyBytes)
		reqHash := hex.EncodeToString(hasher.Sum(nil))

		scope := fmt.Sprintf("%s %s", c.Request.Method, c.FullPath())
		ctx := c.Request.Context()

		// 1. Check PostgreSQL idempotency_keys for an existing completed response
		if database != nil {
			var storedHash string
			var storedCode *int
			var storedBody []byte
			const selectQ = `
				SELECT request_hash, response_code, response_body
				FROM idempotency_keys
				WHERE scope = $1 AND key = $2
			`
			err = database.Conn(ctx).QueryRow(ctx, selectQ, scope, idemKey).Scan(&storedHash, &storedCode, &storedBody)
			if err == nil {
				if storedHash != reqHash {
					response.Fail(
						c,
						http.StatusUnprocessableEntity,
						"IDEMPOTENCY_KEY_PAYLOAD_MISMATCH",
						"Idempotency-Key was already used with a different request payload",
						nil,
					)
					return
				}
				if storedCode != nil && len(storedBody) > 0 {
					c.Header("X-Idempotent-Replay", "true")
					c.Data(*storedCode, "application/json; charset=utf-8", storedBody)
					c.Abort()
					return
				}
			} else if err != pgx.ErrNoRows {
				response.InternalError(c, "Failed to verify idempotency key")
				return
			}
		}

		// 2. Acquire short-lived Redis in-flight lock if Redis is configured
		lockKey := fmt.Sprintf("stayhub:idemp:%s:%s", scope, idemKey)
		lockToken := uuid.NewString()
		if rdb != nil {
			locked, lockErr := rdb.TryLock(ctx, lockKey, lockToken, 30*time.Second)
			if lockErr == nil && !locked {
				response.Conflict(c, "IDEMPOTENCY_IN_FLIGHT", "A request with this Idempotency-Key is currently being processed", nil)
				return
			}
			defer func() {
				_ = rdb.Unlock(context.Background(), lockKey, lockToken)
			}()
		}

		// 3. Capture downstream response
		bw := &bodyCaptureWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBuffer(nil),
		}
		c.Writer = bw

		c.Next()

		// 4. Persist successful or client-deterministic responses (2xx / 4xx except 401/429)
		statusCode := c.Writer.Status()
		if database != nil && statusCode >= 200 && statusCode < 500 && statusCode != http.StatusUnauthorized && statusCode != http.StatusTooManyRequests {
			var subjectID *uuid.UUID
			if subVal, ok := c.Get("subject_id"); ok {
				if parsed, ok := subVal.(uuid.UUID); ok {
					subjectID = &parsed
				}
			}

			const upsertQ = `
				INSERT INTO idempotency_keys (scope, key, subject_id, request_hash, response_code, response_body)
				VALUES ($1, $2, $3, $4, $5, $6)
				ON CONFLICT (scope, key) DO UPDATE
				SET response_code = EXCLUDED.response_code,
				    response_body = EXCLUDED.response_body
			`
			_, _ = database.Conn(ctx).Exec(
				ctx,
				upsertQ,
				scope,
				idemKey,
				subjectID,
				reqHash,
				statusCode,
				bw.body.Bytes(),
			)
		}
	}
}
