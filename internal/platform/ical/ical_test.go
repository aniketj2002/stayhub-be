package ical

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/models"
)

func TestValidateSSRF(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantErrIs error
		wantErr   bool
	}{
		{
			name:      "http scheme rejected",
			url:       "http://example.com/calendar.ics",
			wantErrIs: ErrInvalidScheme,
			wantErr:   true,
		},
		{
			name:      "ftp scheme rejected",
			url:       "ftp://example.com/calendar.ics",
			wantErrIs: ErrInvalidScheme,
			wantErr:   true,
		},
		{
			name:      "loopback ipv4 rejected",
			url:       "https://127.0.0.1/feed.ics",
			wantErrIs: ErrSSRFBlocked,
			wantErr:   true,
		},
		{
			name:      "localhost rejected",
			url:       "https://localhost/feed.ics",
			wantErrIs: ErrSSRFBlocked,
			wantErr:   true,
		},
		{
			name:      "rfc1918 10.x.x.x rejected",
			url:       "https://10.0.0.1/feed.ics",
			wantErrIs: ErrSSRFBlocked,
			wantErr:   true,
		},
		{
			name:      "rfc1918 192.168.x.x rejected",
			url:       "https://192.168.1.100/feed.ics",
			wantErrIs: ErrSSRFBlocked,
			wantErr:   true,
		},
		{
			name:      "rfc1918 172.16.x.x rejected",
			url:       "https://172.16.0.1/feed.ics",
			wantErrIs: ErrSSRFBlocked,
			wantErr:   true,
		},
		{
			name:      "cloud metadata link-local rejected",
			url:       "https://169.254.169.254/latest/meta-data",
			wantErrIs: ErrSSRFBlocked,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSSRF(tt.url)
			if (err != nil) != tt.wantErr {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
			if tt.wantErrIs != nil && err != tt.wantErrIs {
				t.Fatalf("expected error %v, got %v", tt.wantErrIs, err)
			}
		})
	}
}

func TestParseCalendar(t *testing.T) {
	icalContent := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Airbnb Inc//Hosting Calendar 0.8.8//EN
CALSCALE:GREGORIAN
METHOD:PUBLISH
BEGIN:VEVENT
DTSTAMP:20261031T120000Z
DTSTART;VALUE=DATE:20261101
DTEND;VALUE=DATE:20261105
UID:reservation-1@airbnb.com
SUMMARY:Airbnb (Not available)
END:VEVENT
BEGIN:VEVENT
DTSTAMP:20261031T120000Z
DTSTART:20261210T140000Z
DTEND:20261215T100000Z
UID:reservation-2@airbnb.com
SUMMARY:Reserved
END:VEVENT
END:VCALENDAR`

	events, err := ParseCalendar([]byte(icalContent))
	if err != nil {
		t.Fatalf("unexpected error parsing calendar: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	// First event: Nov 1 to Nov 5
	e1 := events[0]
	if e1.UID != "reservation-1@airbnb.com" {
		t.Errorf("expected UID reservation-1@airbnb.com, got %s", e1.UID)
	}
	expectedStart1 := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	expectedEnd1 := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	if !e1.Start.Equal(expectedStart1) {
		t.Errorf("expected start %v, got %v", expectedStart1, e1.Start)
	}
	if !e1.End.Equal(expectedEnd1) {
		t.Errorf("expected end %v, got %v", expectedEnd1, e1.End)
	}

	// Second event: Dec 10 to Dec 15
	e2 := events[1]
	if e2.UID != "reservation-2@airbnb.com" {
		t.Errorf("expected UID reservation-2@airbnb.com, got %s", e2.UID)
	}
	expectedStart2 := time.Date(2026, 12, 10, 0, 0, 0, 0, time.UTC)
	expectedEnd2 := time.Date(2026, 12, 15, 0, 0, 0, 0, time.UTC)
	if !e2.Start.Equal(expectedStart2) {
		t.Errorf("expected start %v, got %v", expectedStart2, e2.Start)
	}
	if !e2.End.Equal(expectedEnd2) {
		t.Errorf("expected end %v, got %v", expectedEnd2, e2.End)
	}
}

func TestSerializeCalendar(t *testing.T) {
	listingID := uuid.New()
	title := "Cozy Mountain Cabin, Aspen; CO"

	entries := []models.ListingCalendar{
		{
			ListingID: listingID,
			Night:     time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
			Status:    enums.CalendarStatusBooked,
		},
		{
			ListingID: listingID,
			Night:     time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
			Status:    enums.CalendarStatusBlocked,
		},
	}

	calStr := SerializeCalendar(listingID, title, entries)

	if !strings.Contains(calStr, "BEGIN:VCALENDAR") || !strings.Contains(calStr, "END:VCALENDAR") {
		t.Fatalf("calendar output missing VCALENDAR wrapper")
	}
	if !strings.Contains(calStr, "X-WR-CALNAME:Cozy Mountain Cabin\\, Aspen\\; CO") {
		t.Errorf("listing title was not escaped properly: %s", calStr)
	}

	// Round-trip parse
	events, err := ParseCalendar([]byte(calStr))
	if err != nil {
		t.Fatalf("failed to parse generated calendar: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events in generated calendar, got %d", len(events))
	}

	if events[0].Summary != "Reserved" {
		t.Errorf("expected summary Reserved, got %s", events[0].Summary)
	}
	if events[1].Summary != "Blocked" {
		t.Errorf("expected summary Blocked, got %s", events[1].Summary)
	}
}
