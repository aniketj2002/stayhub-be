package aws

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/stayhub/backend/internal/config"
)

// DefaultSESSenderEmail is the mandatory sender email address for all StayHub SES emails.
const DefaultSESSenderEmail = "aniketjindal2002@gmail.com"

// CohostInvitationEmailInput contains the details needed to render and send a co-host invitation email.
type CohostInvitationEmailInput struct {
	RecipientEmail string
	HostName       string
	ListingTitle   string
	InviteURL      string
	PayoutShareBPS int
	Permissions    []string
	ExpiresAt      time.Time
}

// EmailSender sends transactional emails via AWS SES.
type EmailSender interface {
	SendCohostInvitation(ctx context.Context, in CohostInvitationEmailInput) error
	SenderEmail() string
}

// SESEmailSender sends outbound emails via AWS SES v2 API signed with SigV4.
type SESEmailSender struct {
	region      string
	endpoint    string
	senderEmail string
	creds       aws.CredentialsProvider
	signer      *v4.Signer
	httpClient  *http.Client
}

type sesSendEmailPayload struct {
	FromEmailAddress string          `json:"FromEmailAddress"`
	Destination      sesDestination  `json:"Destination"`
	Content          sesEmailContent `json:"Content"`
}

type sesDestination struct {
	ToAddresses []string `json:"ToAddresses"`
}

type sesEmailContent struct {
	Simple *sesMessage `json:"Simple,omitempty"`
}

type sesMessage struct {
	Subject sesContentPart `json:"Subject"`
	Body    sesBody        `json:"Body"`
}

type sesBody struct {
	Html *sesContentPart `json:"Html,omitempty"`
	Text *sesContentPart `json:"Text,omitempty"`
}

type sesContentPart struct {
	Data    string `json:"Data"`
	Charset string `json:"Charset"`
}

// NewSESEmailSender creates an AWS SES email sender configured from the application environment.
// It always uses aniketjindal2002@gmail.com as the sender email.
func NewSESEmailSender(ctx context.Context, cfg *config.Config) (*SESEmailSender, error) {
	region := "us-east-1"
	if cfg != nil && cfg.AWSRegion != "" {
		region = cfg.AWSRegion
	}

	var optFns []func(*awsconfig.LoadOptions) error
	optFns = append(optFns, awsconfig.WithRegion(region))
	if cfg != nil && cfg.S3AccessKey != "" && cfg.S3SecretKey != "" && cfg.S3AccessKey != "minioadmin" {
		optFns = append(optFns, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3SessionToken),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for SES: %w", err)
	}

	endpoint := fmt.Sprintf("https://email.%s.amazonaws.com", region)
	if cfg != nil && cfg.AWSEndpoint != "" {
		endpoint = strings.TrimRight(cfg.AWSEndpoint, "/")
	}

	return &SESEmailSender{
		region:      region,
		endpoint:    endpoint,
		senderEmail: DefaultSESSenderEmail,
		creds:       awsCfg.Credentials,
		signer:      v4.NewSigner(),
		httpClient:  &http.Client{Timeout: 15 * time.Second},
	}, nil
}

// NewSESEmailSenderWithHTTPClient creates an SESEmailSender with an injected HTTP client and endpoint (for testing).
func NewSESEmailSenderWithHTTPClient(region, endpoint string, creds aws.CredentialsProvider, httpClient *http.Client) *SESEmailSender {
	if region == "" {
		region = "us-east-1"
	}
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://email.%s.amazonaws.com", region)
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &SESEmailSender{
		region:      region,
		endpoint:    strings.TrimRight(endpoint, "/"),
		senderEmail: DefaultSESSenderEmail,
		creds:       creds,
		signer:      v4.NewSigner(),
		httpClient:  httpClient,
	}
}

// SenderEmail returns the configured SES sender address (always aniketjindal2002@gmail.com).
func (s *SESEmailSender) SenderEmail() string {
	return DefaultSESSenderEmail
}

// SendCohostInvitation builds and sends the co-host invitation email via AWS SES.
func (s *SESEmailSender) SendCohostInvitation(ctx context.Context, in CohostInvitationEmailInput) error {
	toEmail := strings.TrimSpace(in.RecipientEmail)
	if toEmail == "" {
		return fmt.Errorf("recipient email is required")
	}

	hostName := strings.TrimSpace(in.HostName)
	if hostName == "" {
		hostName = "A StayHub Host"
	}

	listingTitle := strings.TrimSpace(in.ListingTitle)
	if listingTitle == "" {
		listingTitle = "StayHub Listing"
	}

	payoutPct := fmt.Sprintf("%.1f%%", float64(in.PayoutShareBPS)/100.0)
	permsText := "View listing"
	if len(in.Permissions) > 0 {
		permsText = strings.Join(in.Permissions, ", ")
	}
	expiryStr := in.ExpiresAt.UTC().Format("January 2, 2006")

	subject := fmt.Sprintf("You've been invited to co-host %s on StayHub", listingTitle)

	textBody := fmt.Sprintf(
		"Hello,\n\n"+
			"%s has invited you to become a co-host for \"%s\" on StayHub.\n\n"+
			"Invitation Details:\n"+
			"- Listing: %s\n"+
			"- Invited by: %s\n"+
			"- Payout share: %s\n"+
			"- Permissions: %s\n"+
			"- Expires on: %s\n\n"+
			"Accept your invitation by visiting the link below:\n"+
			"%s\n\n"+
			"Best regards,\n"+
			"The StayHub Team\n",
		hostName, listingTitle, listingTitle, hostName, payoutPct, permsText, expiryStr, in.InviteURL,
	)

	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8" />
  <title>%s</title>
</head>
<body style="margin:0;padding:24px;background-color:#f7f7f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#222222;">
  <div style="max-width:560px;margin:0 auto;background:#ffffff;border-radius:12px;padding:32px;border:1px solid #ebebeb;">
    <h2 style="margin-top:0;color:#ff385c;font-size:22px;">StayHub Co-Host Invitation</h2>
    <p style="font-size:16px;line-height:1.5;">
      <strong>%s</strong> has invited you to co-host <strong>%s</strong> on StayHub.
    </p>
    <div style="background:#f9fafb;border:1px solid #e5e7eb;border-radius:8px;padding:16px;margin:20px 0;">
      <p style="margin:6px 0;font-size:14px;"><strong>Listing:</strong> %s</p>
      <p style="margin:6px 0;font-size:14px;"><strong>Payout Share:</strong> %s</p>
      <p style="margin:6px 0;font-size:14px;"><strong>Permissions:</strong> %s</p>
      <p style="margin:6px 0;font-size:14px;"><strong>Expires:</strong> %s</p>
    </div>
    <p style="margin:28px 0;">
      <a href="%s" style="display:inline-block;background-color:#ff385c;color:#ffffff;text-decoration:none;font-weight:600;padding:12px 24px;border-radius:8px;font-size:15px;">
        Review &amp; Accept Invitation
      </a>
    </p>
    <p style="font-size:13px;color:#717171;line-height:1.4;word-break:break-all;">
      Or copy and paste this link into your browser:<br />
      <a href="%s" style="color:#222222;">%s</a>
    </p>
  </div>
</body>
</html>`,
		html.EscapeString(subject),
		html.EscapeString(hostName),
		html.EscapeString(listingTitle),
		html.EscapeString(listingTitle),
		html.EscapeString(payoutPct),
		html.EscapeString(permsText),
		html.EscapeString(expiryStr),
		html.EscapeString(in.InviteURL),
		html.EscapeString(in.InviteURL),
		html.EscapeString(in.InviteURL),
	)

	return s.SendEmail(ctx, toEmail, subject, htmlBody, textBody)
}

// SendEmail dispatches an email via AWS SES v2 (/v2/email/outbound-emails) using DefaultSESSenderEmail.
func (s *SESEmailSender) SendEmail(ctx context.Context, toEmail, subject, htmlBody, textBody string) error {
	payload := sesSendEmailPayload{
		FromEmailAddress: DefaultSESSenderEmail,
		Destination: sesDestination{
			ToAddresses: []string{toEmail},
		},
		Content: sesEmailContent{
			Simple: &sesMessage{
				Subject: sesContentPart{
					Data:    subject,
					Charset: "UTF-8",
				},
				Body: sesBody{
					Html: &sesContentPart{
						Data:    htmlBody,
						Charset: "UTF-8",
					},
					Text: &sesContentPart{
						Data:    textBody,
						Charset: "UTF-8",
					},
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal SES email payload: %w", err)
	}

	urlStr := s.endpoint + "/v2/email/outbound-emails"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create SES HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	hashSum := sha256.Sum256(bodyBytes)
	payloadHash := hex.EncodeToString(hashSum[:])

	if s.creds != nil {
		awsCreds, credErr := s.creds.Retrieve(ctx)
		if credErr == nil && awsCreds.HasKeys() {
			if signErr := s.signer.SignHTTP(ctx, awsCreds, req, payloadHash, "ses", s.region, time.Now().UTC()); signErr != nil {
				return fmt.Errorf("failed to sign SES HTTP request: %w", signErr)
			}
		}
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute SES SendEmail request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("SES SendEmail failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	return nil
}

