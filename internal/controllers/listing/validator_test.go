package listing_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/controllers/listing"
)

func TestGuestAndHostListingValidators(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("validates search query with destination, dates, and guests", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(
			http.MethodGet,
			"/api/v1/listings/search?city=Lisbon&country=pt&check_in=2026-12-01&check_out=2026-12-05&guests=2&amenities=wifi,kitchen",
			nil,
		)

		f, err := listing.ValidateSearchListingsFilter(c)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.City != "Lisbon" || f.Country != "PT" || f.Guests != 2 || len(f.Amenities) != 2 {
			t.Fatalf("unexpected filter: %+v", f)
		}
		if f.CheckIn == nil || f.CheckOut == nil {
			t.Fatalf("expected check_in and check_out to be parsed")
		}
	})

	t.Run("rejects search when check_out <= check_in", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(
			http.MethodGet,
			"/api/v1/listings/search?check_in=2026-12-05&check_out=2026-12-01",
			nil,
		)

		_, err := listing.ValidateSearchListingsFilter(c)
		if err == nil {
			t.Fatalf("expected validation error when check_out <= check_in")
		}
	})

	t.Run("validates quote request input", func(t *testing.T) {
		body := `{"check_in":"2026-12-10","check_out":"2026-12-14","guests":3}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/listings/123/quote", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")

		in, err := listing.ValidateQuoteRequestInput(c)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if in.Guests != 3 || in.CheckIn.Format("2006-01-02") != "2026-12-10" {
			t.Fatalf("unexpected parsed quote input: %+v", in)
		}
	})

	t.Run("validates working version patch input", func(t *testing.T) {
		body := `{"wizard_step":3,"country":"pt","timezone":"Europe/Lisbon","max_guests":4,"base_price_minor":15000,"check_in_time":"15:00","check_out_time":"11:00"}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/host/listings/123/working-version", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")

		in, err := listing.ValidateUpdateWorkingVersionInput(c)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if in.WizardStep == nil || *in.WizardStep != 3 || *in.MaxGuests != 4 {
			t.Fatalf("unexpected working version input: %+v", in)
		}
	})
}
