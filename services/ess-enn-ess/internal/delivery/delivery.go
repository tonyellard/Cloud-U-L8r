package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tonyellard/ess-enn-ess/internal/activity"
	"github.com/tonyellard/ess-enn-ess/internal/config"
	"github.com/tonyellard/ess-enn-ess/internal/message"
	"github.com/tonyellard/ess-enn-ess/internal/subscription"
)

// Deliverer handles message delivery to various protocols
type Deliverer struct {
	logger         *slog.Logger
	activityLogger *activity.Logger
	httpClient     *http.Client
	config         *config.Config
}

// NewDeliverer creates a new message deliverer
func NewDeliverer(logger *slog.Logger, activityLogger *activity.Logger, cfg *config.Config) *Deliverer {
	return &Deliverer{
		logger:         logger,
		activityLogger: activityLogger,
		config:         cfg,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Deliver sends a message to a subscription endpoint
func (d *Deliverer) Deliver(ctx context.Context, msg *message.Message, sub *subscription.Subscription) error {
	start := time.Now()

	// Only deliver to confirmed subscriptions
	if sub.Status != subscription.StatusConfirmed {
		d.logger.Debug("skipping delivery to non-confirmed subscription",
			"subscription_arn", sub.SubscriptionArn,
			"status", sub.Status)
		return nil
	}

	var err error
	switch sub.Protocol {
	case subscription.ProtocolHTTP:
		err = d.deliverHTTP(ctx, msg, sub)
	case subscription.ProtocolSQS:
		err = d.deliverSQS(ctx, msg, sub)
	case subscription.ProtocolEmail:
		err = d.deliverEmail(ctx, msg, sub)
	case subscription.ProtocolLambda:
		err = d.deliverLambda(ctx, msg, sub)
	default:
		err = fmt.Errorf("unsupported protocol: %s", sub.Protocol)
	}

	duration := time.Since(start)

	if err != nil {
		d.activityLogger.LogDelivery(msg.TopicArn, msg.MessageId, sub.SubscriptionArn,
			string(sub.Protocol), sub.Endpoint, activity.StatusFailed, duration, err.Error())
		d.logger.Error("message delivery failed",
			"message_id", msg.MessageId,
			"subscription_arn", sub.SubscriptionArn,
			"protocol", sub.Protocol,
			"endpoint", sub.Endpoint,
			"error", err)
		return err
	}

	d.activityLogger.LogDelivery(msg.TopicArn, msg.MessageId, sub.SubscriptionArn,
		string(sub.Protocol), sub.Endpoint, activity.StatusSuccess, duration, "")
	d.logger.Debug("message delivered successfully",
		"message_id", msg.MessageId,
		"subscription_arn", sub.SubscriptionArn,
		"protocol", sub.Protocol,
		"endpoint", sub.Endpoint)

	return nil
}

// deliverHTTP sends message to HTTP/HTTPS endpoint with retry logic
func (d *Deliverer) deliverHTTP(ctx context.Context, msg *message.Message, sub *subscription.Subscription) error {
	if !d.config.HTTP.Enabled {
		d.logger.Info("HTTP delivery skipped (disabled in config)",
			"message_id", msg.MessageId,
			"endpoint", sub.Endpoint)
		return nil
	}

	payload, err := msg.ToSNSJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	maxRetries := d.config.HTTP.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff with jitter
			backoffMs := d.config.HTTP.RetryBackoffMs * int(math.Pow(2, float64(attempt-1)))
			if backoffMs > 30000 { // Cap at 30 seconds
				backoffMs = 30000
			}

			d.logger.Debug("retrying HTTP delivery",
				"message_id", msg.MessageId,
				"attempt", attempt,
				"max_retries", maxRetries,
				"backoff_ms", backoffMs)

			// Log retry attempt
			d.activityLogger.LogDelivery(msg.TopicArn, msg.MessageId, sub.SubscriptionArn,
				string(sub.Protocol), sub.Endpoint, activity.StatusRetrying,
				time.Duration(backoffMs)*time.Millisecond, fmt.Sprintf("retry attempt %d/%d", attempt, maxRetries))

			// Wait before retry
			select {
			case <-time.After(time.Duration(backoffMs) * time.Millisecond):
			case <-ctx.Done():
				return fmt.Errorf("delivery cancelled: %w", ctx.Err())
			}
		}

		// Attempt delivery
		err := d.attemptHTTPDelivery(ctx, msg, sub, payload)
		if err == nil {
			if attempt > 0 {
				d.logger.Info("HTTP delivery succeeded after retries",
					"message_id", msg.MessageId,
					"endpoint", sub.Endpoint,
					"attempts", attempt+1)
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !d.isRetryableError(err) {
			d.logger.Warn("permanent error, not retrying",
				"message_id", msg.MessageId,
				"error", err)
			return err
		}

		// If this was the last retry, return error
		if attempt == maxRetries {
			d.logger.Error("HTTP delivery failed after all retries",
				"message_id", msg.MessageId,
				"endpoint", sub.Endpoint,
				"attempts", attempt+1,
				"error", lastErr)
			return fmt.Errorf("delivery failed after %d attempts: %w", attempt+1, lastErr)
		}
	}

	return lastErr
}

// attemptHTTPDelivery performs a single HTTP delivery attempt
func (d *Deliverer) attemptHTTPDelivery(ctx context.Context, msg *message.Message, sub *subscription.Subscription, payload string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Endpoint, bytes.NewBufferString(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "ess-enn-ess/1.0")
	req.Header.Set("X-Amz-Sns-Message-Type", "Notification")
	req.Header.Set("X-Amz-Sns-Topic-Arn", msg.TopicArn)
	req.Header.Set("X-Amz-Sns-Message-Id", msg.MessageId)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return &TransientError{Err: err, Message: "HTTP request failed"}
	}
	defer resp.Body.Close()

	// Read response body for error messages
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Determine if status code is retryable
		if d.isRetryableStatusCode(resp.StatusCode) {
			return &TransientError{
				Err:     fmt.Errorf("HTTP %d", resp.StatusCode),
				Message: fmt.Sprintf("endpoint returned status %d: %s", resp.StatusCode, string(body)),
			}
		}
		return &PermanentError{
			Err:     fmt.Errorf("HTTP %d", resp.StatusCode),
			Message: fmt.Sprintf("endpoint returned status %d: %s", resp.StatusCode, string(body)),
		}
	}

	return nil
}

// isRetryableError determines if an error should trigger a retry
func (d *Deliverer) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's a transient error
	var transientErr *TransientError
	if errors, ok := err.(*TransientError); ok {
		transientErr = errors
		_ = transientErr // Use it
		return true
	}

	// Check if it's a permanent error
	var permanentErr *PermanentError
	if errors, ok := err.(*PermanentError); ok {
		permanentErr = errors
		_ = permanentErr // Use it
		return false
	}

	// Default: network errors are retryable
	return true
}

// isRetryableStatusCode determines if an HTTP status code is retryable
func (d *Deliverer) isRetryableStatusCode(statusCode int) bool {
	// 5xx errors are typically retryable (server errors)
	if statusCode >= 500 && statusCode < 600 {
		return true
	}

	// 429 Too Many Requests is retryable
	if statusCode == 429 {
		return true
	}

	// 408 Request Timeout is retryable
	if statusCode == 408 {
		return true
	}

	// 4xx errors are typically permanent (client errors)
	return false
}

// TransientError represents a temporary error that can be retried
type TransientError struct {
	Err     error
	Message string
}

func (e *TransientError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Err.Error()
}

func (e *TransientError) Unwrap() error {
	return e.Err
}

// PermanentError represents a permanent error that should not be retried
type PermanentError struct {
	Err     error
	Message string
}

func (e *PermanentError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Err.Error()
}

func (e *PermanentError) Unwrap() error {
	return e.Err
}

// deliverSQS sends message to SQS queue
func (d *Deliverer) deliverSQS(ctx context.Context, msg *message.Message, sub *subscription.Subscription) error {
	if !d.config.SQS.Enabled {
		d.logger.Info("SQS delivery skipped (disabled in config)",
			"message_id", msg.MessageId,
			"queue_url", sub.Endpoint)
		return nil
	}

	// Get the SNS message in JSON format to send to SQS
	snsMessage, err := msg.ToSNSJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize SNS message: %w", err)
	}

	// Prepare SQS SendMessage request
	queueURL := sub.Endpoint
	sqsEndpoint := d.config.SQS.Endpoint

	// Build the form data for SQS SendMessage API
	formData := url.Values{}
	formData.Set("Action", "SendMessage")
	formData.Set("QueueUrl", queueURL)
	formData.Set("MessageBody", snsMessage)

	// Create request to SQS emulator
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sqsEndpoint, strings.NewReader(formData.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create SQS request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "ess-enn-ess/1.0")

	// Send request to SQS
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("SQS request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Try to parse error response
		var errBody bytes.Buffer
		errBody.ReadFrom(resp.Body)
		return fmt.Errorf("SQS endpoint returned status %d: %s", resp.StatusCode, errBody.String())
	}

	// Parse response to get MessageId
	var sqsResp struct {
		MessageId string `json:"MessageId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&sqsResp); err != nil {
		// Response might be XML, but we got success so just log
		d.logger.Debug("SQS delivery successful (could not parse response)",
			"message_id", msg.MessageId,
			"queue_url", queueURL)
	} else {
		d.logger.Debug("SQS delivery successful",
			"sns_message_id", msg.MessageId,
			"sqs_message_id", sqsResp.MessageId,
			"queue_url", queueURL)
	}

	return nil
}

// deliverEmail simulates email delivery
func (d *Deliverer) deliverEmail(ctx context.Context, msg *message.Message, sub *subscription.Subscription) error {
	// Email delivery is simulated in dev mode
	d.logger.Info("email delivery (simulated)",
		"message_id", msg.MessageId,
		"to", sub.Endpoint,
		"subject", msg.Subject,
		"message", msg.Message)
	return nil
}

// deliverLambda simulates Lambda invocation
func (d *Deliverer) deliverLambda(ctx context.Context, msg *message.Message, sub *subscription.Subscription) error {
	// Lambda invocation is simulated
	d.logger.Info("lambda delivery (simulated)",
		"message_id", msg.MessageId,
		"function_arn", sub.Endpoint)
	return nil
}
