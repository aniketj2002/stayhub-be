package aws

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/credentials"
)

type roundTripFunc func(req *http.Request) *http.Response

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func TestSESEmailSender_SendCohostInvitation(t *testing.T) {
	var capturedPayload sesSendEmailPayload
	var capturedAuthHeader string
	var capturedPath string

	transport := roundTripFunc(func(r *http.Request) *http.Response {
		capturedPath = r.URL.Path
		capturedAuthHeader = r.Header.Get("Authorization")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		if err := json.Unmarshal(body, &capturedPayload); err != nil {
			t.Fatalf("failed to unmarshal SES payload: %v", err)
		}

		resBody := io.NopCloser(bytes.NewBufferString(`{"MessageId":"ses-msg-123"}`))
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       resBody,
		}
	})

	httpClient := &http.Client{Transport: transport}
	creds := credentials.NewStaticCredentialsProvider("AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "")
	sender := NewSESEmailSenderWithHTTPClient("us-east-1", "https://email.us-east-1.amazonaws.com", creds, httpClient)

	if sender.SenderEmail() != "aniketjindal2002@gmail.com" {
		t.Fatalf("expected sender email aniketjindal2002@gmail.com, got %s", sender.SenderEmail())
	}

	err := sender.SendCohostInvitation(context.Background(), CohostInvitationEmailInput{
		RecipientEmail: "cohost@example.com",
		HostName:       "Ines Costa",
		ListingTitle:   "Chic Waterfront Condo",
		InviteURL:      "https://stayhub.example.com/cohost-invitations/tok_abc123",
		PayoutShareBPS: 2000,
		Permissions:    []string{"Listing details", "Calendar & pricing", "Reservations"},
		ExpiresAt:      time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("unexpected error sending cohost invitation email: %v", err)
	}

	if capturedPath != "/v2/email/outbound-emails" {
		t.Errorf("expected path /v2/email/outbound-emails, got %s", capturedPath)
	}
	if !strings.HasPrefix(capturedAuthHeader, "AWS4-HMAC-SHA256") || !strings.Contains(capturedAuthHeader, "/us-east-1/ses/aws4_request") {
		t.Errorf("expected SigV4 SES Authorization header, got %q", capturedAuthHeader)
	}
	if capturedPayload.FromEmailAddress != "aniketjindal2002@gmail.com" {
		t.Errorf("expected FromEmailAddress aniketjindal2002@gmail.com, got %s", capturedPayload.FromEmailAddress)
	}
	if len(capturedPayload.Destination.ToAddresses) != 1 || capturedPayload.Destination.ToAddresses[0] != "cohost@example.com" {
		t.Errorf("expected recipient cohost@example.com, got %v", capturedPayload.Destination.ToAddresses)
	}
	if capturedPayload.Content.Simple == nil {
		t.Fatalf("expected Simple email content")
	}
	if !strings.Contains(capturedPayload.Content.Simple.Subject.Data, "Chic Waterfront Condo") {
		t.Errorf("expected subject to mention listing title, got %q", capturedPayload.Content.Simple.Subject.Data)
	}
	if !strings.Contains(capturedPayload.Content.Simple.Body.Html.Data, "https://stayhub.example.com/cohost-invitations/tok_abc123") {
		t.Errorf("expected HTML body to include invitation URL")
	}
	if !strings.Contains(capturedPayload.Content.Simple.Body.Text.Data, "20.0%") {
		t.Errorf("expected text body to include 20.0%% payout share")
	}
}
