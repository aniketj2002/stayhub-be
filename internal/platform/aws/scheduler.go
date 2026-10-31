package aws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/scheduler"
	schedulertypes "github.com/aws/aws-sdk-go-v2/service/scheduler/types"
	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/config"
)

// PayoutScheduler schedules or cancels one-time payout releases via AWS EventBridge Scheduler.
type PayoutScheduler interface {
	SchedulePayout(ctx context.Context, payoutID, reservationID uuid.UUID, releaseAt time.Time) error
	CancelPayoutSchedule(ctx context.Context, payoutID uuid.UUID) error
}

// PayoutSchedulePayload is the message payload delivered by EventBridge Scheduler to the SQS queue.
type PayoutSchedulePayload struct {
	PayoutID      string `json:"payout_id"`
	ReservationID string `json:"reservation_id"`
}

// EventBridgePayoutScheduler is the AWS EventBridge Scheduler implementation of PayoutScheduler.
type EventBridgePayoutScheduler struct {
	client   *scheduler.Client
	roleARN  string
	queueARN string
}

// NewEventBridgePayoutScheduler instantiates a scheduler client using application config.
func NewEventBridgePayoutScheduler(ctx context.Context, cfg *config.Config) (*EventBridgePayoutScheduler, error) {
	var optFns []func(*awsconfig.LoadOptions) error
	if cfg.AWSRegion != "" {
		optFns = append(optFns, awsconfig.WithRegion(cfg.AWSRegion))
	}
	if cfg.S3AccessKey != "" && cfg.S3SecretKey != "" {
		optFns = append(optFns, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3SessionToken),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for scheduler: %w", err)
	}

	client := scheduler.NewFromConfig(awsCfg, func(o *scheduler.Options) {
		if cfg.AWSEndpoint != "" {
			o.BaseEndpoint = aws.String(cfg.AWSEndpoint)
		}
	})

	return NewEventBridgePayoutSchedulerWithClient(client, cfg.EventBridgeRoleARN, cfg.PayoutQueueARN), nil
}

// NewEventBridgePayoutSchedulerWithClient allows injecting an existing scheduler.Client (e.g. for testing).
func NewEventBridgePayoutSchedulerWithClient(client *scheduler.Client, roleARN, queueARN string) *EventBridgePayoutScheduler {
	return &EventBridgePayoutScheduler{
		client:   client,
		roleARN:  roleARN,
		queueARN: queueARN,
	}
}

// SchedulePayout creates or updates a one-time AWS EventBridge schedule to fire into SQS at releaseAt.
func (s *EventBridgePayoutScheduler) SchedulePayout(
	ctx context.Context,
	payoutID, reservationID uuid.UUID,
	releaseAt time.Time,
) error {
	scheduleName := fmt.Sprintf("stayhub-payout-%s", payoutID.String())
	scheduleExpr := fmt.Sprintf("at(%s)", releaseAt.UTC().Format("2006-01-02T15:04:05"))

	payload, err := json.Marshal(PayoutSchedulePayload{
		PayoutID:      payoutID.String(),
		ReservationID: reservationID.String(),
	})
	if err != nil {
		return fmt.Errorf("failed to marshal payout payload: %w", err)
	}
	inputStr := string(payload)

	createInput := &scheduler.CreateScheduleInput{
		Name:               aws.String(scheduleName),
		ScheduleExpression: aws.String(scheduleExpr),
		FlexibleTimeWindow: &schedulertypes.FlexibleTimeWindow{
			Mode: schedulertypes.FlexibleTimeWindowModeOff,
		},
		Target: &schedulertypes.Target{
			Arn:     aws.String(s.queueARN),
			RoleArn: aws.String(s.roleARN),
			Input:   aws.String(inputStr),
		},
		ActionAfterCompletion: schedulertypes.ActionAfterCompletionDelete,
	}

	_, err = s.client.CreateSchedule(ctx, createInput)
	if err != nil {
		// If schedule already exists (e.g. reschedule on partial refund), update it
		var conflictErr *schedulertypes.ConflictException
		if errors.As(err, &conflictErr) {
			updateInput := &scheduler.UpdateScheduleInput{
				Name:               aws.String(scheduleName),
				ScheduleExpression: aws.String(scheduleExpr),
				FlexibleTimeWindow: &schedulertypes.FlexibleTimeWindow{
					Mode: schedulertypes.FlexibleTimeWindowModeOff,
				},
				Target: &schedulertypes.Target{
					Arn:     aws.String(s.queueARN),
					RoleArn: aws.String(s.roleARN),
					Input:   aws.String(inputStr),
				},
				ActionAfterCompletion: schedulertypes.ActionAfterCompletionDelete,
			}
			_, updateErr := s.client.UpdateSchedule(ctx, updateInput)
			if updateErr != nil {
				return fmt.Errorf("failed to update existing EventBridge schedule %s: %w", scheduleName, updateErr)
			}
			return nil
		}
		return fmt.Errorf("failed to create EventBridge schedule %s: %w", scheduleName, err)
	}

	return nil
}

// CancelPayoutSchedule deletes the EventBridge schedule for the given payout.
// If the schedule is not found, it treats it as success (idempotent).
func (s *EventBridgePayoutScheduler) CancelPayoutSchedule(ctx context.Context, payoutID uuid.UUID) error {
	scheduleName := fmt.Sprintf("stayhub-payout-%s", payoutID.String())
	_, err := s.client.DeleteSchedule(ctx, &scheduler.DeleteScheduleInput{
		Name: aws.String(scheduleName),
	})
	if err != nil {
		var notFoundErr *schedulertypes.ResourceNotFoundException
		if errors.As(err, &notFoundErr) {
			return nil
		}
		return fmt.Errorf("failed to delete EventBridge schedule %s: %w", scheduleName, err)
	}
	return nil
}
