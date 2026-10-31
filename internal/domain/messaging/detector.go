package messaging

import (
	"regexp"
	"strings"
)

var (
	// Email regex
	emailRegex = regexp.MustCompile(`(?i)\b[A-Za-z0-9._%+-]+(?:\s*@\s*|\s*\[at\]\s*)[A-Za-z0-9.-]+\s*\.\s*[A-Za-z]{2,}\b`)

	// Phone numbers (e.g. +1 555 123 4567, 07123 456789, +44-20-7946-0991, 555.123.4567)
	phoneRegex = regexp.MustCompile(`(?:\+?\d{1,3}[-.\s]?)?\(?\d{2,4}\)?[-.\s]?\d{3,4}[-.\s]?\d{3,4}`)

	// IBAN / UPI handles (e.g. GB29NWBK60161331926819, user@okhdfcbank, someone@upi)
	ibanRegex = regexp.MustCompile(`(?i)\b[A-Z]{2}\d{2}[A-Z0-9]{11,30}\b`)
	upiRegex  = regexp.MustCompile(`(?i)\b[a-zA-Z0-9.\-_]{2,256}@(upi|okhdfcbank|okaxis|oksbi|okicici|paytm|ybl|ibl)\b`)

	// Suspicious phrases & keywords
	keywordsRegex = regexp.MustCompile(`(?i)\b(?:pay\s+outside|wire\s+transfer|cash\s+on\s+arrival|bank\s+transfer|western\s+union|moneygram|venmo|zelle|whatsapp|telegram|signal|contact\s+me\s+directly)\b`)
)

const OffPlatformFlag = "off_platform"
const OffPlatformWarning = "Warning: Keeping communication and payments on StayHub protects your reservations, identity, and refunds under our Guest & Host Guarantees."

// DetectOffPlatform scans a message body for off-platform contact or payment attempts.
func DetectOffPlatform(body string) (hasFlag bool, warning string) {
	clean := strings.TrimSpace(body)
	if clean == "" {
		return false, ""
	}

	if emailRegex.MatchString(clean) {
		return true, OffPlatformWarning
	}
	if phoneRegex.MatchString(clean) {
		// Only flag if it looks like a phone number with at least 8 digits
		digits := extractDigits(clean)
		if len(digits) >= 8 {
			return true, OffPlatformWarning
		}
	}
	if ibanRegex.MatchString(clean) || upiRegex.MatchString(clean) {
		return true, OffPlatformWarning
	}
	if keywordsRegex.MatchString(clean) {
		return true, OffPlatformWarning
	}

	return false, ""
}

func extractDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
