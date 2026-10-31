package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/interfaces"
	platformaws "github.com/stayhub/backend/internal/platform/aws"
)

// PayoutConsumer consumes one-time payout release events delivered by AWS EventBridge Scheduler to SQS.
type PayoutConsumer struct {
	sqsClient     *sqs.Client
	queueURL      string
	payoutService interfaces.PayoutService
}

// NewPayoutConsumer creates a new PayoutConsumer.
func NewPayoutConsumer(
	sqsClient *sqs.Client,
	queueURL string,
	payoutService interfaces.PayoutService,
) *PayoutConsumer {
	return &PayoutConsumer{
		sqsClient:     sqsClient,
		queueURL:      queueURL,
		payoutService: payoutService,
	}
}

// Run executes the long-polling loop against SQS until ctx is cancelled.
func (c *PayoutConsumer) Run(ctx context.Context) error {
	if c.sqsClient == nil || c.queueURL == "" {
		log.Println("Payout consumer disabled: SQS client or queue URL not configured")
		<-ctx.Done()
		return nil
	}

	log.Printf("StayHub SQS payout consumer started (queue=%s)", c.queueURL)

	for {
		select {
		case <-ctx.Done():
			log.Println("StayHub SQS payout consumer stopping...")
			return nil
		default:
			out, err := c.sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
				QueueUrl:            aws.String(c.queueURL),
				MaxNumberOfMessages: 10,
				WaitTimeSeconds:     20, // SQS Long Polling
			})
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				log.Printf("SQS ReceiveMessage error: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			for _, msg := range out.Messages {
				if err := c.processMessage(ctx, msg); err != nil {
					log.Printf("failed to process payout message (id=%s): %v; leaving in queue for redrive/DLQ", aws.ToString(msg.MessageId), err)
					// Fail-safe: do not delete, message returns to queue and routes to DLQ after 3 failures
					continue
				}

				// Acknowledge and delete message on successful or idempotent handling
				_, delErr := c.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
					QueueUrl:      aws.String(c.queueURL),
					ReceiptHandle: msg.ReceiptHandle,
				})
				if delErr != nil {
					log.Printf("failed to delete SQS message (id=%s): %v", aws.ToString(msg.MessageId), delErr)
				}
			}
		}
	}
}

func (c *PayoutConsumer) processMessage(ctx context.Context, msg sqstypes.Message) error {
	if msg.Body == nil {
		return errors.New("empty message body")
	}

	var payload platformaws.PayoutSchedulePayload
	if err := json.Unmarshal([]byte(*msg.Body), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payout payload: %w", err)
	}

	payoutID, err := uuid.Parse(payload.PayoutID)
	if err != nil {
		return fmt.Errorf("invalid payout_id uuid '%s': %w", payload.PayoutID, err)
	}

	return c.payoutService.ProcessPayoutRelease(ctx, payoutID)
}
