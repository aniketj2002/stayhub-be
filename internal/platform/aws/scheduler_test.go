package aws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

type mockScheduler struct {
	scheduled map[string]string
	deleted   map[string]bool
	fail      bool
}

func (m *mockScheduler) SchedulePayout(ctx context.Context, payoutID, reservationID uuid.UUID, releaseAt time.Time) error {
	if m.fail {
		return errors.New("aws scheduler connection refused")
	}
	name := fmt.Sprintf("stayhub-payout-%s", payoutID)
	payload, _ := json.Marshal(PayoutSchedulePayload{
		PayoutID:      payoutID.String(),
		ReservationID: reservationID.String(),
	})
	m.scheduled[name] = string(payload)
	return nil
}

func (m *mockScheduler) CancelPayoutSchedule(ctx context.Context, payoutID uuid.UUID) error {
	if m.fail {
		return errors.New("aws scheduler delete error")
	}
	name := fmt.Sprintf("stayhub-payout-%s", payoutID)
	m.deleted[name] = true
	return nil
}

func TestPayoutSchedulePayload_Marshal(t *testing.T) {
	pID := uuid.New()
	rID := uuid.New()
	p := PayoutSchedulePayload{
		PayoutID:      pID.String(),
		ReservationID: rID.String(),
	}

	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded PayoutSchedulePayload
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.PayoutID != pID.String() {
		t.Errorf("expected payout ID %s, got %s", pID.String(), decoded.PayoutID)
	}
	if decoded.ReservationID != rID.String() {
		t.Errorf("expected reservation ID %s, got %s", rID.String(), decoded.ReservationID)
	}
}

func TestMockSchedulerOperations(t *testing.T) {
	mock := &mockScheduler{
		scheduled: make(map[string]string),
		deleted:   make(map[string]bool),
	}

	ctx := context.Background()
	pID := uuid.New()
	rID := uuid.New()
	releaseAt := time.Now().Add(24 * time.Hour)

	if err := mock.SchedulePayout(ctx, pID, rID, releaseAt); err != nil {
		t.Fatalf("unexpected schedule error: %v", err)
	}

	expectedName := fmt.Sprintf("stayhub-payout-%s", pID)
	if _, ok := mock.scheduled[expectedName]; !ok {
		t.Errorf("schedule %s not found in mock", expectedName)
	}

	if err := mock.CancelPayoutSchedule(ctx, pID); err != nil {
		t.Fatalf("unexpected cancel error: %v", err)
	}
	if !mock.deleted[expectedName] {
		t.Errorf("expected schedule %s to be marked deleted", expectedName)
	}
}
