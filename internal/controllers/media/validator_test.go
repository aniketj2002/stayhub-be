package media_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/controllers/media"
)

func TestValidatePresignMediaInput(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("valid media presign defaults purpose to listing_photo", func(t *testing.T) {
		body := `{"filename":"villa.jpg","content_type":"IMAGE/JPEG","size_bytes":524288,"width":1600,"height":1200}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/media/presign", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")

		in, err := media.ValidatePresignMediaInput(c)
		if err != nil {
			t.Fatalf("unexpected validation error: %v", err)
		}
		if in.ContentType != "image/jpeg" || in.Purpose != "listing_photo" {
			t.Fatalf("unexpected parsed media input: %+v", in)
		}
	})

	t.Run("rejects unsupported content_type and excessive size", func(t *testing.T) {
		body := `{"content_type":"application/pdf","size_bytes":999999999}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/media/presign", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")

		_, err := media.ValidatePresignMediaInput(c)
		if err == nil || len(err.Fields) != 2 {
			t.Fatalf("expected 2 field errors, got: %+v", err)
		}
	})
}
