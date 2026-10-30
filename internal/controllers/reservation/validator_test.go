package reservation_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/controllers/reservation"
)

func TestReservationValidators(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("validates create reservation input", func(t *testing.T) {
		listingID := uuid.New()
		futureStart := time.Now().UTC().AddDate(0, 1, 0).Format("2006-01-02")
		futureEnd := time.Now().UTC().AddDate(0, 1, 3).Format("2006-01-02")
		body := `{"listing_id":"` + listingID.String() + `","check_in":"` + futureStart + `","check_out":"` + futureEnd + `","guests":2,"guest_message":"Looking forward to our stay!"}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/reservations", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")

		in, err := reservation.ValidateCreateReservationInput(c)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if in.ListingID != listingID || in.Guests != 2 || in.PaymentMethodToken != "tok_simulated_default" {
			t.Fatalf("unexpected parsed input: %+v", in)
		}
	})

	t.Run("rejects check_in date in the past", func(t *testing.T) {
		listingID := uuid.New()
		pastStart := time.Now().UTC().AddDate(0, 0, -2).Format("2006-01-02")
		pastEnd := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")
		body := `{"listing_id":"` + listingID.String() + `","check_in":"` + pastStart + `","check_out":"` + pastEnd + `","guests":2}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/reservations", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")

		_, err := reservation.ValidateCreateReservationInput(c)
		if err == nil {
			t.Fatalf("expected validation error for past check_in date")
		}
		found := false
		for _, f := range err.Fields {
			if f.Field == "check_in" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected field error on check_in, got fields: %+v", err.Fields)
		}
	})

	t.Run("rejects decline without decline_reason", func(t *testing.T) {
		body := `{"decline_reason":"   "}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/host/reservations/123/decline", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")

		_, err := reservation.ValidateDeclineReservationInput(c)
		if err == nil {
			t.Fatalf("expected validation error for empty decline_reason")
		}
	})
}
