// SPDX-License-Identifier: Apache-2.0
package poller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tonyellard/cloud-u-l8r/pkg/matching"
	"github.com/tonyellard/pipes/internal/delivery"
	"github.com/tonyellard/pipes/internal/model"
	"github.com/tonyellard/pipes/internal/store"
)

// Poller polls SQS sources for running pipes and delivers to targets.
type Poller struct {
	logger      *slog.Logger
	store       *store.Store
	deliverer   *delivery.Deliverer
	sqsEndpoint string
	client      *http.Client
}

// New creates a new Poller.
func New(logger *slog.Logger, st *store.Store, d *delivery.Deliverer, sqsEndpoint string) *Poller {
	return &Poller{
		logger:      logger,
		store:       st,
		deliverer:   d,
		sqsEndpoint: sqsEndpoint,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Start begins the polling loop. Blocks until ctx is cancelled.
func (p *Poller) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	p.logger.Info("poller started", "interval", "5s")

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("poller stopped")
			return
		case <-ticker.C:
			p.poll()
		}
	}
}

func (p *Poller) poll() {
	pipes := p.store.RunningPipes()
	for _, pipe := range pipes {
		if !strings.Contains(pipe.Source, ":sqs:") {
			continue
		}
		p.pollSQS(pipe)
	}
}

// sqsReceiveResponse represents the SQS ReceiveMessage JSON response.
type sqsReceiveResponse struct {
	Messages []sqsMessage `json:"Messages"`
}

type sqsMessage struct {
	MessageId     string `json:"MessageId"`
	ReceiptHandle string `json:"ReceiptHandle"`
	Body          string `json:"Body"`
	MD5OfBody     string `json:"MD5OfBody"`
}

func (p *Poller) pollSQS(pipe *model.Pipe) {
	parts := strings.Split(pipe.Source, ":")
	if len(parts) < 6 {
		p.logger.Warn("invalid SQS source ARN", "arn", pipe.Source)
		return
	}
	queueName := parts[len(parts)-1]
	accountID := parts[len(parts)-2]
	queueURL := fmt.Sprintf("%s/%s/%s", p.sqsEndpoint, accountID, queueName)

	batchSize := 1
	if pipe.SourceParameters != nil && pipe.SourceParameters.SqsQueueParameters != nil {
		if pipe.SourceParameters.SqsQueueParameters.BatchSize > 0 {
			batchSize = pipe.SourceParameters.SqsQueueParameters.BatchSize
		}
	}

	// Use JSON protocol for SQS ReceiveMessage
	reqBody := map[string]interface{}{
		"QueueUrl":            queueURL,
		"MaxNumberOfMessages": batchSize,
		"WaitTimeSeconds":     0,
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequest(http.MethodPost, p.sqsEndpoint, bytes.NewReader(body))
	if err != nil {
		p.logger.Warn("failed to create SQS receive request", "pipe", pipe.Name, "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	req.Header.Set("X-Amz-Target", "AmazonSQS.ReceiveMessage")

	resp, err := p.client.Do(req)
	if err != nil {
		p.logger.Warn("SQS receive failed", "pipe", pipe.Name, "queue", queueName, "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		p.logger.Warn("SQS receive returned error", "pipe", pipe.Name, "status", resp.StatusCode, "body", string(respBody))
		return
	}

	var sqsResp sqsReceiveResponse
	if err := json.NewDecoder(resp.Body).Decode(&sqsResp); err != nil {
		p.logger.Warn("failed to decode SQS response", "pipe", pipe.Name, "error", err)
		return
	}

	for _, msg := range sqsResp.Messages {
		p.processMessage(pipe, queueURL, msg)
	}
}

func (p *Poller) processMessage(pipe *model.Pipe, queueURL string, msg sqsMessage) {
	// Apply filter criteria
	if !p.matchesFilter(pipe, msg.Body) {
		// Message doesn't match filter — delete it (filtered out)
		p.deleteSQSMessage(pipe.Name, queueURL, msg.ReceiptHandle)
		return
	}

	payload := msg.Body

	// Apply enrichment if configured
	if pipe.Enrichment != "" {
		enriched, err := p.enrich(pipe, payload)
		if err != nil {
			p.logger.Warn("enrichment failed, skipping message",
				"pipe", pipe.Name, "messageId", msg.MessageId, "error", err)
			return
		}
		payload = enriched
	}

	// Deliver to target
	if err := p.deliverer.Deliver(payload, pipe); err != nil {
		p.logger.Warn("delivery failed",
			"pipe", pipe.Name, "messageId", msg.MessageId, "error", err)
		return
	}

	// Delete from SQS on success
	p.deleteSQSMessage(pipe.Name, queueURL, msg.ReceiptHandle)
}

func (p *Poller) matchesFilter(pipe *model.Pipe, messageBody string) bool {
	if pipe.FilterCriteria == nil || len(pipe.FilterCriteria.Filters) == 0 {
		return true
	}

	// Wrap message body in an object with "body" key for pattern matching
	// This matches AWS behavior where SQS message body is available as "body"
	event := fmt.Sprintf(`{"body":%s}`, messageBody)

	for _, f := range pipe.FilterCriteria.Filters {
		matched, err := matching.Match(f.Pattern, event)
		if err != nil {
			p.logger.Warn("filter pattern match error", "pipe", pipe.Name, "error", err)
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

func (p *Poller) enrich(pipe *model.Pipe, payload string) (string, error) {
	enrichURL := pipe.Enrichment

	body := payload
	if pipe.EnrichmentParameters != nil && pipe.EnrichmentParameters.InputTemplate != "" {
		body = pipe.EnrichmentParameters.InputTemplate
	}

	req, err := http.NewRequest(http.MethodPost, enrichURL, strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("enrichment request creation failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if pipe.EnrichmentParameters != nil && pipe.EnrichmentParameters.HttpParameters != nil {
		for k, v := range pipe.EnrichmentParameters.HttpParameters.HeaderParameters {
			req.Header.Set(k, v)
		}
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("enrichment call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("enrichment returned status %d", resp.StatusCode)
	}

	enrichedBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("enrichment read failed: %w", err)
	}

	return string(enrichedBody), nil
}

func (p *Poller) deleteSQSMessage(pipeName, queueURL, receiptHandle string) {
	form := url.Values{}
	form.Set("Action", "DeleteMessage")
	form.Set("QueueUrl", queueURL)
	form.Set("ReceiptHandle", receiptHandle)

	resp, err := p.client.PostForm(queueURL, form)
	if err != nil {
		p.logger.Warn("SQS delete failed", "pipe", pipeName, "error", err)
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		p.logger.Warn("SQS delete returned error", "pipe", pipeName, "status", resp.StatusCode)
	}
}
