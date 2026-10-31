package ical

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/models"
)

var (
	ErrInvalidScheme   = errors.New("url must use https scheme")
	ErrSSRFBlocked     = errors.New("url resolves to private or disallowed IP range")
	ErrFeedTooLarge    = errors.New("feed size exceeds 5MB limit")
	ErrInvalidCalendar = errors.New("invalid icalendar format")
)

const maxFeedBytes = 5 * 1024 * 1024 // 5MB

type FeedEvent struct {
	UID     string
	Summary string
	Start   time.Time
	End     time.Time
}

// ValidateSSRF checks if the URL is https and does not resolve to loopback/private/link-local IP addresses.
func ValidateSSRF(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if strings.ToLower(u.Scheme) != "https" {
		return ErrInvalidScheme
	}

	host := u.Hostname()
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("lookup host: %w", err)
	}

	for _, ip := range ips {
		if isDisallowedIP(ip) {
			return ErrSSRFBlocked
		}
	}
	return nil
}

func isDisallowedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	// Check IPv4-mapped IPv6 addresses (e.g. ::ffff:127.0.0.1)
	if ip4 := ip.To4(); ip4 != nil {
		if ip4.IsLoopback() || ip4.IsPrivate() || ip4.IsLinkLocalUnicast() {
			return true
		}
	}
	return false
}

// FetchFeedWithGuard downloads an iCal feed with timeout, size limit, and SSRF check.
func FetchFeedWithGuard(ctx context.Context, feedURL string, timeout time.Duration) ([]byte, error) {
	if err := ValidateSSRF(feedURL); err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			return ValidateSSRF(req.URL.String())
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "StayHub-CalendarSync/2.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream returned http status %d", resp.StatusCode)
	}

	limitedReader := io.LimitReader(resp.Body, maxFeedBytes+1)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, err
	}
	if len(body) > maxFeedBytes {
		return nil, ErrFeedTooLarge
	}

	return body, nil
}

// ParseCalendar parses an RFC 5545 iCalendar byte slice into a list of FeedEvents.
func ParseCalendar(data []byte) ([]FeedEvent, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var events []FeedEvent
	var inEvent bool
	var current FeedEvent

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		key := strings.ToUpper(parts[0])
		val := ""
		if len(parts) > 1 {
			val = parts[1]
		}

		// Handle keys with params like DTSTART;VALUE=DATE:20261201
		if idx := strings.Index(key, ";"); idx != -1 {
			key = key[:idx]
		}

		switch key {
		case "BEGIN":
			if strings.ToUpper(val) == "VEVENT" {
				inEvent = true
				current = FeedEvent{}
			}
		case "END":
			if strings.ToUpper(val) == "VEVENT" && inEvent {
				if !current.Start.IsZero() {
					if current.End.IsZero() || current.End.Before(current.Start) {
						current.End = current.Start.Add(24 * time.Hour)
					}
					events = append(events, current)
				}
				inEvent = false
			}
		case "UID":
			if inEvent {
				current.UID = val
			}
		case "SUMMARY":
			if inEvent {
				current.Summary = val
			}
		case "DTSTART":
			if inEvent {
				t, err := parseICalDate(val)
				if err == nil {
					current.Start = t
				}
			}
		case "DTEND":
			if inEvent {
				t, err := parseICalDate(val)
				if err == nil {
					current.End = t
				}
			}
		}
	}

	return events, scanner.Err()
}

func parseICalDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	// Try formats: 20060102T150405Z, 20060102T150405, 20060102
	formats := []string{
		"20060102T150405Z",
		"20060102T150405",
		"20060102",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
		}
	}
	return time.Time{}, errors.New("unknown date format")
}

// SerializeCalendar formats calendar nights and reservations into an RFC 5545 iCalendar string.
func SerializeCalendar(listingID uuid.UUID, listingTitle string, calendarEntries []models.ListingCalendar) string {
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//StayHub//StayHub Calendar 2.0//EN\r\n")
	b.WriteString("CALSCALE:GREGORIAN\r\n")
	b.WriteString("METHOD:PUBLISH\r\n")
	if listingTitle != "" {
		b.WriteString(fmt.Sprintf("X-WR-CALNAME:%s\r\n", escapeICal(listingTitle)))
	}

	nowStr := time.Now().UTC().Format("20060102T150405Z")

	for _, entry := range calendarEntries {
		start := entry.Night.UTC()
		end := start.AddDate(0, 0, 1)

		summary := "Reserved"
		if entry.Status == "blocked" {
			summary = "Blocked"
		}

		b.WriteString("BEGIN:VEVENT\r\n")
		b.WriteString(fmt.Sprintf("UID:%s-%s@stayhub.dev\r\n", listingID.String(), start.Format("20060102")))
		b.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", nowStr))
		b.WriteString(fmt.Sprintf("DTSTART;VALUE=DATE:%s\r\n", start.Format("20060102")))
		b.WriteString(fmt.Sprintf("DTEND;VALUE=DATE:%s\r\n", end.Format("20060102")))
		b.WriteString(fmt.Sprintf("SUMMARY:%s\r\n", summary))
		b.WriteString("END:VEVENT\r\n")
	}

	b.WriteString("END:VCALENDAR\r\n")
	return b.String()
}

func escapeICal(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}
