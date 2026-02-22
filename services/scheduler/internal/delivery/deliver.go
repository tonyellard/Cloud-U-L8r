// SPDX-License-Identifier: Apache-2.0
package delivery

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tonyellard/scheduler/internal/model"
)

// Deliverer sends scheduled payloads to targets (SQS, SNS).
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

// Deliver sends the payload to the given target.
func (d *Deliverer) Deliver(payload string, target model.ScheduleTarget) error {
	arn := target.Arn

	switch {
	case strings.Contains(arn, ":sqs:"):
		return d.deliverToSQS(arn, payload, target.SqsParameters)
	case strings.Contains(arn, ":sns:"):
		return d.deliverToSNS(arn, payload)
	default:
		d.logger.Warn("unsupported scheduler target type", "arn", arn)
		return fmt.Errorf("unsupported target ARN: %s", arn)
	}
}

func (d *Deliverer) deliverToSQS(arn, message string, sqsParams *model.SqsParameters) error {
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
	if sqsParams != nil && sqsParams.MessageGroupId != "" {
		form.Set("MessageGroupId", sqsParams.MessageGroupId)
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

	d.logger.Info("scheduler delivered to SQS", "queue", queueName, "status", resp.StatusCode)
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

	d.logger.Info("scheduler delivered to SNS", "topic", arn, "status", resp.StatusCode)
	return nil
}
