package admin_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/controllers/admin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func makeJSONContext(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	return c, w
}

func TestValidateAdminCancelReservation(t *testing.T) {
	// 1. Missing reason
	c, _ := makeJSONContext(http.MethodPost, "/cancel", `{"reason":"  ","refund_mode":"full"}`)
	_, valErr := admin.ValidateAdminCancelReservation(c)
	if valErr == nil {
		t.Fatalf("expected validation error on whitespace reason")
	}
	if len(valErr.Fields) == 0 || valErr.Fields[0].Field != "reason" {
		t.Errorf("expected field error on 'reason', got %+v", valErr.Fields)
	}

	// 2. Invalid refund mode
	c, _ = makeJSONContext(http.MethodPost, "/cancel", `{"reason":"valid","refund_mode":"random_mode"}`)
	_, valErr = admin.ValidateAdminCancelReservation(c)
	if valErr == nil {
		t.Fatalf("expected validation error on invalid refund mode")
	}

	// 3. Custom mode without amount_minor
	c, _ = makeJSONContext(http.MethodPost, "/cancel", `{"reason":"valid","refund_mode":"custom"}`)
	_, valErr = admin.ValidateAdminCancelReservation(c)
	if valErr == nil {
		t.Fatalf("expected validation error on custom refund mode without amount_minor")
	}

	// 4. Valid custom mode
	c, _ = makeJSONContext(http.MethodPost, "/cancel", `{"reason":"valid","refund_mode":"custom","amount_minor":1234}`)
	in, valErr := admin.ValidateAdminCancelReservation(c)
	if valErr != nil {
		t.Fatalf("unexpected error on valid custom input: %v", valErr)
	}
	if in.RefundMode != "custom" || *in.AmountMinor != 1234 {
		t.Errorf("unexpected parsed input: %+v", in)
	}
}

func TestValidateAdminRefundReservation(t *testing.T) {
	// 1. Missing reason
	c, _ := makeJSONContext(http.MethodPost, "/refund", `{"reason":"","amount_minor":5000}`)
	_, valErr := admin.ValidateAdminRefundReservation(c)
	if valErr == nil {
		t.Fatalf("expected validation error on empty reason")
	}

	// 2. Zero amount
	c, _ = makeJSONContext(http.MethodPost, "/refund", `{"reason":"good reason","amount_minor":0}`)
	_, valErr = admin.ValidateAdminRefundReservation(c)
	if valErr == nil {
		t.Fatalf("expected validation error on zero amount")
	}

	// 3. Valid refund input
	c, _ = makeJSONContext(http.MethodPost, "/refund", `{"reason":"valid reason","amount_minor":5000}`)
	in, valErr := admin.ValidateAdminRefundReservation(c)
	if valErr != nil {
		t.Fatalf("unexpected validation error: %v", valErr)
	}
	if in.AmountMinor != 5000 || in.Reason != "valid reason" {
		t.Errorf("unexpected parsed input: %+v", in)
	}
}

func TestValidatePayoutHoldAndRelease(t *testing.T) {
	// Hold: empty reason
	c, _ := makeJSONContext(http.MethodPost, "/hold", `{"reason":"   "}`)
	_, valErr := admin.ValidatePayoutHold(c)
	if valErr == nil {
		t.Fatalf("expected validation error on empty hold reason")
	}

	// Hold: valid reason
	c, _ = makeJSONContext(http.MethodPost, "/hold", `{"reason":"Investigating chargeback"}`)
	holdIn, valErr := admin.ValidatePayoutHold(c)
	if valErr != nil {
		t.Fatalf("unexpected validation error: %v", valErr)
	}
	if holdIn.Reason != "Investigating chargeback" {
		t.Errorf("expected reason 'Investigating chargeback', got %s", holdIn.Reason)
	}

	// Release: empty reason
	c, _ = makeJSONContext(http.MethodPost, "/release", `{"reason":""}`)
	_, valErr = admin.ValidatePayoutRelease(c)
	if valErr == nil {
		t.Fatalf("expected validation error on empty release reason")
	}

	// Release: valid reason
	c, _ = makeJSONContext(http.MethodPost, "/release", `{"reason":"Resolved dispute"}`)
	relIn, valErr := admin.ValidatePayoutRelease(c)
	if valErr != nil {
		t.Fatalf("unexpected validation error: %v", valErr)
	}
	if relIn.Reason != "Resolved dispute" {
		t.Errorf("expected reason 'Resolved dispute', got %s", relIn.Reason)
	}
}

func TestValidateListingReviewDecision(t *testing.T) {
	// Empty note
	c, _ := makeJSONContext(http.MethodPost, "/decide", `{"note":"   "}`)
	_, valErr := admin.ValidateListingReviewDecision(c)
	if valErr == nil {
		t.Fatalf("expected validation error on empty review decision note")
	}

	// Valid note
	c, _ = makeJSONContext(http.MethodPost, "/decide", `{"note":"Approved after address verification"}`)
	in, valErr := admin.ValidateListingReviewDecision(c)
	if valErr != nil {
		t.Fatalf("unexpected validation error: %v", valErr)
	}
	if in.Note != "Approved after address verification" {
		t.Errorf("unexpected note: %s", in.Note)
	}
}

func TestValidateProposeFeeConfig(t *testing.T) {
	// Negative fee BPS
	c, _ := makeJSONContext(http.MethodPost, "/propose", `{"guest_fee_bps":-100,"host_fee_bps":300,"reason":"test"}`)
	_, valErr := admin.ValidateProposeFeeConfig(c)
	if valErr == nil {
		t.Fatalf("expected validation error on negative guest fee BPS")
	}

	// Empty reason
	c, _ = makeJSONContext(http.MethodPost, "/propose", `{"guest_fee_bps":1400,"host_fee_bps":300,"reason":""}`)
	_, valErr = admin.ValidateProposeFeeConfig(c)
	if valErr == nil {
		t.Fatalf("expected validation error on empty propose reason")
	}

	// Valid proposal
	c, _ = makeJSONContext(http.MethodPost, "/propose", `{"guest_fee_bps":1400,"host_fee_bps":300,"reason":"Annual adjustment"}`)
	in, valErr := admin.ValidateProposeFeeConfig(c)
	if valErr != nil {
		t.Fatalf("unexpected validation error: %v", valErr)
	}
	if in.GuestFeeBPS != 1400 || in.HostFeeBPS != 300 || in.Reason != "Annual adjustment" {
		t.Errorf("unexpected parsed input: %+v", in)
	}
}
