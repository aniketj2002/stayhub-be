package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/models"
)

type mockPayoutService struct {
	processed []uuid.UUID
	failWith  error
}

func (m *mockPayoutService) SchedulePayout(_ context.Context, _ models.Reservation, _ string) (*models.Payout, error) {
	return nil, nil
}
func (m *mockPayoutService) HoldPayout(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ string) (*models.Payout, error) {
	return nil, nil
}
func (m *mockPayoutService) ReleasePayoutHold(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ string) (*models.Payout, error) {
	return nil, nil
}
func (m *mockPayoutService) ReleaseDuePayouts(_ context.Context, _ int) (int, error) {
	return 0, nil
}
func (m *mockPayoutService) AdjustPayoutOnRefund(_ context.Context, _ models.Reservation, _ int64) error {
	return nil
}
func (m *mockPayoutService) ProcessPayoutRelease(_ context.Context, payoutID uuid.UUID) error {
	if m.failWith != nil {
		return m.failWith
	}
	m.processed = append(m.processed, payoutID)
	return nil
}

func TestPayoutConsumer_ProcessMessage_Valid(t *testing.T) {
	pID := uuid.New()
	mockSvc := &mockPayoutService{}
	consumer := NewPayoutConsumer(nil, "https://sqs.fake/queue", mockSvc)

	body := `{"payout_id":"` + pID.String() + `","reservation_id":"` + uuid.New().String() + `"}`
	msg := sqstypes.Message{
		Body: aws.String(body),
	}

	err := consumer.processMessage(context.Background(), msg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(mockSvc.processed) != 1 || mockSvc.processed[0] != pID {
		t.Fatalf("expected processed payout ID %s, got %v", pID, mockSvc.processed)
	}
}

func TestPayoutConsumer_ProcessMessage_EmptyBody(t *testing.T) {
	consumer := NewPayoutConsumer(nil, "https://sqs.fake/queue", &mockPayoutService{})
	err := consumer.processMessage(context.Background(), sqstypes.Message{Body: nil})
	if err == nil {
		t.Fatal("expected error for empty body, got nil")
	}
}

func TestPayoutConsumer_ProcessMessage_InvalidJSON(t *testing.T) {
	consumer := NewPayoutConsumer(nil, "https://sqs.fake/queue", &mockPayoutService{})
	err := consumer.processMessage(context.Background(), sqstypes.Message{
		Body: aws.String("not-json"),
	})
	if err == nil {
		t.Fatal("expected error for invalid json, got nil")
	}
}

func TestPayoutConsumer_ProcessMessage_InvalidUUID(t *testing.T) {
	consumer := NewPayoutConsumer(nil, "https://sqs.fake/queue", &mockPayoutService{})
	err := consumer.processMessage(context.Background(), sqstypes.Message{
		Body: aws.String(`{"payout_id":"invalid-uuid"}`),
	})
	if err == nil {
		t.Fatal("expected error for invalid uuid, got nil")
	}
}

func TestPayoutConsumer_ProcessMessage_ServiceFailure(t *testing.T) {
	mockSvc := &mockPayoutService{
		failWith: errors.New("db connection failure"),
	}
	consumer := NewPayoutConsumer(nil, "https://sqs.fake/queue", mockSvc)
	pID := uuid.New()
	err := consumer.processMessage(context.Background(), sqstypes.Message{
		Body: aws.String(`{"payout_id":"` + pID.String() + `"}`),
	})
	if err == nil {
		t.Fatal("expected error when service fails, got nil")
	}
	if err.Error() != "db connection failure" {
		t.Fatalf("unexpected error message: %v", err)
	}
}
