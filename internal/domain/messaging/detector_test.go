package messaging

import (
	"testing"
)

func TestDetectOffPlatform(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		wantOffPlate bool
	}{
		{
			name:         "innocent message",
			body:         "Hi! What time is check-in tomorrow? Looking forward to our stay.",
			wantOffPlate: false,
		},
		{
			name:         "email detected",
			body:         "Reach me at host@example.com for faster response",
			wantOffPlate: true,
		},
		{
			name:         "obfuscated email detected",
			body:         "Reach me at host [at] gmail.com",
			wantOffPlate: true,
		},
		{
			name:         "phone number detected",
			body:         "Call me at +1-555-234-5678 when you arrive",
			wantOffPlate: true,
		},
		{
			name:         "whatsapp keyword detected",
			body:         "Let's chat on WhatsApp instead",
			wantOffPlate: true,
		},
		{
			name:         "telegram keyword detected",
			body:         "My telegram username is @coolhost",
			wantOffPlate: true,
		},
		{
			name:         "pay outside phrase detected",
			body:         "Can we pay outside StayHub to avoid fees?",
			wantOffPlate: true,
		},
		{
			name:         "wire transfer phrase detected",
			body:         "Please send a wire transfer for deposit",
			wantOffPlate: true,
		},
		{
			name:         "zelle / venmo detected",
			body:         "Just send it to my Zelle account",
			wantOffPlate: true,
		},
		{
			name:         "UPI handle detected",
			body:         "Send payment to myaccount@okhdfcbank",
			wantOffPlate: true,
		},
		{
			name:         "IBAN detected",
			body:         "Transfer to GB29NWBK60161331926819 please",
			wantOffPlate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flagged, warning := DetectOffPlatform(tt.body)
			if flagged != tt.wantOffPlate {
				t.Fatalf("DetectOffPlatform(%q) flagged = %v, want %v", tt.body, flagged, tt.wantOffPlate)
			}
			if flagged && warning == "" {
				t.Errorf("expected warning when message is flagged")
			}
		})
	}
}
