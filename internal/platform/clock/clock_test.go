package clock_test

import (
	"testing"
	"time"

	"github.com/stayhub/backend/internal/platform/clock"
)

func TestMockClock(t *testing.T) {
	initial := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	mc := clock.NewMockClock(initial)

	if !mc.Now().Equal(initial) {
		t.Fatalf("expected %v, got %v", initial, mc.Now())
	}

	advanced := mc.Advance(24 * time.Hour)
	expected := initial.Add(24 * time.Hour)
	if !advanced.Equal(expected) || !mc.Now().Equal(expected) {
		t.Fatalf("expected %v after 24h advance, got %v", expected, mc.Now())
	}
}
