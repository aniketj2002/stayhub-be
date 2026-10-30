package clock

import (
	"sync"
	"time"
)

// Clock abstracts time retrieval so business logic (24h pending expiry,
// refund hours_before check-in, and check_in_at + 24h payout release)
// can be deterministically tested.
type Clock interface {
	Now() time.Time
}

// RealClock returns the current system time in UTC.
type RealClock struct{}

func NewRealClock() *RealClock {
	return &RealClock{}
}

func (RealClock) Now() time.Time {
	return time.Now().UTC()
}

// MockClock is a thread-safe controllable clock for tests.
type MockClock struct {
	mu      sync.RWMutex
	current time.Time
}

func NewMockClock(initial time.Time) *MockClock {
	return &MockClock{current: initial.UTC()}
}

func (m *MockClock) Now() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

func (m *MockClock) Set(t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current = t.UTC()
}

func (m *MockClock) Advance(d time.Duration) time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current = m.current.Add(d)
	return m.current
}
