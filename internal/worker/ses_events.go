package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// sesEventPollErrorBackoff — how long to sleep after a receive failure
// before retrying. Short enough that transient errors don't stall
// processing, long enough not to thrash on persistent failures.
const sesEventPollErrorBackoff = 5 * time.Second

// SQSClient is the subset of the SQS API we use. Defined as an interface
// for testability and to keep the worker package's import surface minimal.
type SQSClient interface {
	ReceiveMessage(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
}

// snsEnvelope is the outer wrapper SNS adds when delivering to SQS.
// The Message field is itself a JSON string (escaped), containing the
// SES event we actually care about.
type snsEnvelope struct {
	Type    string `json:"Type"`
	Message string `json:"Message"`
}

// sesEventDiscriminator is the minimal shape we parse to figure out
// which job_type to enqueue. The full event is forwarded as-is to the
// handler, so we don't need to fully unmarshal it here.
type sesEventDiscriminator struct {
	EventType string `json:"eventType"`
}

// PollSESEvents receives SES bounce/complaint events from SQS and
// enqueues them as task_queue rows for the worker pool to process.
//
// The pipeline is: SES → SNS → SQS → this poller → task_queue → handler.
// Each step provides its own durability/retry guarantees; this loop only
// cares about correctly translating SQS messages into task_queue inserts.
//
// On any error processing a message, the message is left in SQS to be
// redelivered. After maxReceiveCount redeliveries (configured on the
// queue), it moves to the DLQ.
func (p *Pool) PollSESEvents(ctx context.Context, sqsClient SQSClient, queueURL string) {
	if sqsClient == nil || queueURL == "" {
		p.logger.Info("ses events poller disabled (no sqs client or queue url)")
		return
	}
	
	p.logger.Info("ses events poller started", "queue", queueURL)
	defer p.logger.Info("ses events poller stopped")

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		out, err := sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(queueURL),
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     20, // long polling; blocks up to 20s when empty
		})
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			p.logger.Error("sqs receive", "err", err)
			if !p.sleep(ctx, sesEventPollErrorBackoff) {
				return
			}
			continue
		}

		for _, msg := range out.Messages {
			if err := p.processSESMessage(ctx, msg); err != nil {
				// Leave the message in SQS for redelivery. Don't delete.
				p.logger.Error("processing ses message",
					"err", err, "message_id", aws.ToString(msg.MessageId))
				continue
			}

			if _, err := sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
				QueueUrl:      aws.String(queueURL),
				ReceiptHandle: msg.ReceiptHandle,
			}); err != nil {
				// Insert succeeded but delete failed. SQS will redeliver,
				// the handler is idempotent (same UPDATE produces same
				// state), so this is benign — just produces a duplicate
				// task_queue row that does nothing harmful.
				p.logger.Error("sqs delete after enqueue",
					"err", err, "message_id", aws.ToString(msg.MessageId))
			}
		}
	}
}

// processSESMessage parses the SNS envelope, extracts the SES event,
// and enqueues a task_queue row for the appropriate handler.
func (p *Pool) processSESMessage(ctx context.Context, msg types.Message) error {
	body := aws.ToString(msg.Body)

	var env snsEnvelope
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		return fmt.Errorf("unmarshaling SNS envelope: %w", err)
	}

	// The SES event lives in env.Message as a JSON string.
	var disc sesEventDiscriminator
	if err := json.Unmarshal([]byte(env.Message), &disc); err != nil {
		return fmt.Errorf("unmarshaling SES event discriminator: %w", err)
	}

	var jobType string
	switch disc.EventType {
	case "Bounce":
		jobType = "ses:bounce"
	case "Complaint":
		jobType = "ses:complaint"
	case "Reject", "Rendering Failure":
		// Subscribed to these for visibility but no automated action —
		// log and acknowledge without enqueuing a task.
		p.logger.Warn("received SES event with no handler", "event_type", disc.EventType)
		return nil
	default:
		// Unknown event type. Log it so we notice if subscriptions change,
		// but acknowledge it so we don't block the queue.
		p.logger.Warn("received unknown SES event type", "event_type", disc.EventType)
		return nil
	}

	// Forward the raw SES event JSON as the task payload. The handler
	// will unmarshal it into the type it expects.
	_, err := p.db.Exec(ctx, `
        INSERT INTO task_queue (job_type, payload, status, run_at)
        VALUES ($1, $2::jsonb, 'pending', now())
    `, jobType, env.Message)
	if err != nil {
		return fmt.Errorf("enqueuing %s task: %w", jobType, err)
	}

	return nil
}
