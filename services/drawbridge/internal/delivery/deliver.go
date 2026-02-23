// SPDX-License-Identifier: Apache-2.0
package delivery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tonyellard/drawbridge/internal/model"
)

// Deliverer sends matched events to targets (SQS, SNS, HTTP/S).
type Deliverer struct {
	logger      *slog.Logger
	sqsEndpoint string
	snsEndpoint string
	client      *http.Client
}

// NewDeliverer creates a Deliverer with configured endpoints.
func NewDeliverer(logger *slog.Logger, sqsEndpoint, snsEndpoint string) *Deliverer {
	return &Deliverer{
		logger:      logger,
		sqsEndpoint: sqsEndpoint,
		snsEndpoint: snsEndpoint,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Deliver sends the event JSON to a single target based on the target ARN.
func (d *Deliverer) Deliver(eventJSON string, target model.Target) error {
	// Apply input transformation
	payload := eventJSON
	if target.Input != "" {
		payload = target.Input
	}

	arn := target.Arn

	switch {
	case strings.Contains(arn, ":sqs:"):
		return d.deliverToSQS(arn, payload)
	case strings.Contains(arn, ":sns:"):
		return d.deliverToSNS(arn, payload)
	case strings.HasPrefix(arn, "arn:aws:execute-api:") || strings.HasPrefix(arn, "https://") || strings.HasPrefix(arn, "http://"):
		return d.deliverToHTTP(arn, payload)
	default:
		d.logger.Warn("unsupported target type", "arn", arn)
		return fmt.Errorf("unsupported target ARN: %s", arn)
	}
}

func (d *Deliverer) deliverToSQS(arn, message string) error {
	// Extract queue name from ARN: arn:aws:sqs:<region>:<account>:<queue-name>
	parts := strings.Split(arn, ":")
	if len(parts) < 6 {
		return fmt.Errorf("invalid SQS ARN: %s", arn)
	}
	queueName := parts[len(parts)-1]
	accountID := parts[len(parts)-2]

	queueURL := fmt.Sprintf("%s/%s/%s", d.sqsEndpoint, accountID, queueName)

	form := url.Values{}
	form.Set("Action", "SendMessage")
	form.Set("QueueUrl", queueURL)
	form.Set("MessageBody", message)

	resp, err := d.client.PostForm(queueURL, form)
	if err != nil {
		return fmt.Errorf("SQS delivery failed: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("SQS returned status %d for queue %s", resp.StatusCode, queueName)
	}

	d.logger.Info("delivered to SQS", "queue", queueName, "status", resp.StatusCode)
	return nil
}

func (d *Deliverer) deliverToSNS(arn, message string) error {
	form := url.Values{}
	form.Set("Action", "Publish")
	form.Set("TopicArn", arn)
	form.Set("Message", message)

	resp, err := d.client.PostForm(d.snsEndpoint, form)
	if err != nil {
		return fmt.Errorf("SNS delivery failed: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("SNS returned status %d for topic %s", resp.StatusCode, arn)
	}

	d.logger.Info("delivered to SNS", "topic", arn, "status", resp.StatusCode)
	return nil
}

func (d *Deliverer) deliverToHTTP(endpoint, message string) error {
	targetURL := endpoint
	// If it's an ARN, treat it as unsupported for now
	if strings.HasPrefix(endpoint, "arn:") {
		return fmt.Errorf("API Gateway ARN targets not yet supported: %s", endpoint)
	}

	body, err := json.Marshal(map[string]interface{}{
		"version":    "0",
		"detail":     json.RawMessage(message),
		"source":     "aws.events",
		"time":       time.Now().UTC().Format(time.RFC3339),
		"detail-type": "EventBridge Target Delivery",
	})
	if err != nil {
		return fmt.Errorf("failed to marshal HTTP payload: %w", err)
	}

	resp, err := d.client.Post(targetURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("HTTP delivery failed: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP target returned status %d", resp.StatusCode)
	}

	d.logger.Info("delivered to HTTP", "url", targetURL, "status", resp.StatusCode)
	return nil
}
