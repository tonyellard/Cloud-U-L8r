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

	"github.com/tonyellard/pipes/internal/model"
)

// Deliverer sends pipe payloads to targets (SQS, SNS, HTTP, EventBridge).
type Deliverer struct {
	logger              *slog.Logger
	sqsEndpoint         string
	snsEndpoint         string
	eventbridgeEndpoint string
	client              *http.Client
}

// NewDeliverer creates a Deliverer with configured endpoints.
func NewDeliverer(logger *slog.Logger, sqsEndpoint, snsEndpoint, eventbridgeEndpoint string) *Deliverer {
	return &Deliverer{
		logger:              logger,
		sqsEndpoint:         sqsEndpoint,
		snsEndpoint:         snsEndpoint,
		eventbridgeEndpoint: eventbridgeEndpoint,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Deliver sends the payload to the pipe's target.
func (d *Deliverer) Deliver(payload string, pipe *model.Pipe) error {
	// Apply input template if configured
	message := payload
	if pipe.TargetParameters != nil && pipe.TargetParameters.InputTemplate != "" {
		message = pipe.TargetParameters.InputTemplate
	}

	arn := pipe.Target

	switch {
	case strings.Contains(arn, ":sqs:"):
		return d.deliverToSQS(arn, message, pipe.TargetParameters)
	case strings.Contains(arn, ":sns:"):
		return d.deliverToSNS(arn, message)
	case strings.HasPrefix(arn, "https://") || strings.HasPrefix(arn, "http://"):
		return d.deliverToHTTP(arn, message, pipe.TargetParameters)
	case strings.Contains(arn, ":events:") || strings.Contains(arn, ":event-bus/"):
		return d.deliverToEventBridge(message, pipe.TargetParameters)
	default:
		d.logger.Warn("unsupported pipe target type", "arn", arn)
		return fmt.Errorf("unsupported target ARN: %s", arn)
	}
}

func (d *Deliverer) deliverToSQS(arn, message string, params *model.TargetParameters) error {
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

	if params != nil && params.SqsQueueParameters != nil {
		if params.SqsQueueParameters.MessageGroupId != "" {
			form.Set("MessageGroupId", params.SqsQueueParameters.MessageGroupId)
		}
		if params.SqsQueueParameters.MessageDeduplicationId != "" {
			form.Set("MessageDeduplicationId", params.SqsQueueParameters.MessageDeduplicationId)
		}
	}

	resp, err := d.client.PostForm(queueURL, form)
	if err != nil {
		return fmt.Errorf("SQS delivery failed: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("SQS returned status %d for queue %s", resp.StatusCode, queueName)
	}

	d.logger.Info("pipe delivered to SQS", "queue", queueName, "status", resp.StatusCode)
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

	d.logger.Info("pipe delivered to SNS", "topic", arn, "status", resp.StatusCode)
	return nil
}

func (d *Deliverer) deliverToHTTP(targetURL, message string, params *model.TargetParameters) error {
	req, err := http.NewRequest(http.MethodPost, targetURL, strings.NewReader(message))
	if err != nil {
		return fmt.Errorf("HTTP delivery request creation failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if params != nil && params.HttpParameters != nil {
		for k, v := range params.HttpParameters.HeaderParameters {
			req.Header.Set(k, v)
		}
		if len(params.HttpParameters.QueryStringParameters) > 0 {
			q := req.URL.Query()
			for k, v := range params.HttpParameters.QueryStringParameters {
				q.Set(k, v)
			}
			req.URL.RawQuery = q.Encode()
		}
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP delivery failed: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP target returned status %d", resp.StatusCode)
	}

	d.logger.Info("pipe delivered to HTTP", "url", targetURL, "status", resp.StatusCode)
	return nil
}

func (d *Deliverer) deliverToEventBridge(message string, params *model.TargetParameters) error {
	// Build PutEvents request
	entry := map[string]interface{}{
		"Detail": message,
	}

	if params != nil && params.EventBridgeEventBusParameters != nil {
		eb := params.EventBridgeEventBusParameters
		if eb.Source != "" {
			entry["Source"] = eb.Source
		}
		if eb.DetailType != "" {
			entry["DetailType"] = eb.DetailType
		}
		if len(eb.Resources) > 0 {
			entry["Resources"] = eb.Resources
		}
	}

	if _, ok := entry["Source"]; !ok {
		entry["Source"] = "pipes"
	}
	if _, ok := entry["DetailType"]; !ok {
		entry["DetailType"] = "PipeForwarded"
	}

	putEventsReq := map[string]interface{}{
		"Entries": []interface{}{entry},
	}

	body, err := json.Marshal(putEventsReq)
	if err != nil {
		return fmt.Errorf("EventBridge marshal failed: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, d.eventbridgeEndpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("EventBridge request creation failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", "AWSEvents.PutEvents")

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("EventBridge delivery failed: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("EventBridge returned status %d", resp.StatusCode)
	}

	d.logger.Info("pipe delivered to EventBridge", "status", resp.StatusCode)
	return nil
}
