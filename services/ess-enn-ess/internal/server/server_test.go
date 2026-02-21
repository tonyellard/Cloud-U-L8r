// SPDX-License-Identifier: Apache-2.0

package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/tonyellard/ess-enn-ess/internal/config"
)

// newTestServer creates a Server instance for testing
func newTestServer() *Server {
	cfg := config.Default()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewServer(cfg, logger)
}

// postSNS is a helper to send a form-encoded SNS action
func postSNS(t *testing.T, s *Server, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)
	return rr
}

// --- Health endpoint ---

func TestHealth(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "ess-enn-ess") {
		t.Errorf("expected service name in health response, got: %s", rr.Body.String())
	}
}

// --- CreateTopic ---

func TestCreateTopic(t *testing.T) {
	s := newTestServer()

	form := url.Values{}
	form.Set("Action", "CreateTopic")
	form.Set("Name", "test-topic")

	rr := postSNS(t, s, form)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := rr.Body.String()
	if !strings.Contains(body, "<TopicArn>") {
		t.Errorf("expected TopicArn in response, got: %s", body)
	}
	if !strings.Contains(body, "test-topic") {
		t.Errorf("expected topic name in ARN, got: %s", body)
	}
}

func TestCreateTopic_MissingName(t *testing.T) {
	s := newTestServer()

	form := url.Values{}
	form.Set("Action", "CreateTopic")

	rr := postSNS(t, s, form)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// --- ListTopics ---

func TestListTopics_Empty(t *testing.T) {
	s := newTestServer()

	form := url.Values{}
	form.Set("Action", "ListTopics")

	rr := postSNS(t, s, form)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "ListTopicsResponse") {
		t.Errorf("expected ListTopicsResponse, got: %s", rr.Body.String())
	}
}

func TestListTopics_WithTopics(t *testing.T) {
	s := newTestServer()

	// Create two topics
	for _, name := range []string{"topic-a", "topic-b"} {
		form := url.Values{}
		form.Set("Action", "CreateTopic")
		form.Set("Name", name)
		postSNS(t, s, form)
	}

	form := url.Values{}
	form.Set("Action", "ListTopics")
	rr := postSNS(t, s, form)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "topic-a") || !strings.Contains(body, "topic-b") {
		t.Errorf("expected both topics in response, got: %s", body)
	}
}

// --- DeleteTopic ---

func TestDeleteTopic(t *testing.T) {
	s := newTestServer()

	// Create a topic first
	form := url.Values{}
	form.Set("Action", "CreateTopic")
	form.Set("Name", "to-delete")
	rr := postSNS(t, s, form)

	// Extract topic ARN from response
	body := rr.Body.String()
	arnStart := strings.Index(body, "<TopicArn>") + len("<TopicArn>")
	arnEnd := strings.Index(body, "</TopicArn>")
	topicArn := body[arnStart:arnEnd]

	// Delete it
	form = url.Values{}
	form.Set("Action", "DeleteTopic")
	form.Set("TopicArn", topicArn)
	rr = postSNS(t, s, form)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify it's gone by listing
	form = url.Values{}
	form.Set("Action", "ListTopics")
	rr = postSNS(t, s, form)

	if strings.Contains(rr.Body.String(), "to-delete") {
		t.Error("topic should have been deleted")
	}
}

func TestDeleteTopic_MissingArn(t *testing.T) {
	s := newTestServer()

	form := url.Values{}
	form.Set("Action", "DeleteTopic")

	rr := postSNS(t, s, form)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// --- Subscribe ---

func TestSubscribe(t *testing.T) {
	s := newTestServer()

	// Create topic
	form := url.Values{}
	form.Set("Action", "CreateTopic")
	form.Set("Name", "sub-topic")
	rr := postSNS(t, s, form)

	body := rr.Body.String()
	arnStart := strings.Index(body, "<TopicArn>") + len("<TopicArn>")
	arnEnd := strings.Index(body, "</TopicArn>")
	topicArn := body[arnStart:arnEnd]

	// Subscribe
	form = url.Values{}
	form.Set("Action", "Subscribe")
	form.Set("TopicArn", topicArn)
	form.Set("Protocol", "http")
	form.Set("Endpoint", "http://example.com/webhook")

	rr = postSNS(t, s, form)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	if !strings.Contains(rr.Body.String(), "<SubscriptionArn>") {
		t.Errorf("expected SubscriptionArn in response, got: %s", rr.Body.String())
	}
}

func TestSubscribe_MissingParams(t *testing.T) {
	s := newTestServer()

	form := url.Values{}
	form.Set("Action", "Subscribe")
	// Missing TopicArn, Protocol, Endpoint

	rr := postSNS(t, s, form)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestSubscribe_InvalidProtocol(t *testing.T) {
	s := newTestServer()

	// Create topic
	form := url.Values{}
	form.Set("Action", "CreateTopic")
	form.Set("Name", "proto-topic")
	rr := postSNS(t, s, form)

	body := rr.Body.String()
	arnStart := strings.Index(body, "<TopicArn>") + len("<TopicArn>")
	arnEnd := strings.Index(body, "</TopicArn>")
	topicArn := body[arnStart:arnEnd]

	// Subscribe with invalid protocol
	form = url.Values{}
	form.Set("Action", "Subscribe")
	form.Set("TopicArn", topicArn)
	form.Set("Protocol", "carrier-pigeon")
	form.Set("Endpoint", "the-roof")

	rr = postSNS(t, s, form)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// --- GetTopicAttributes ---

func TestGetTopicAttributes(t *testing.T) {
	s := newTestServer()

	// Create topic
	form := url.Values{}
	form.Set("Action", "CreateTopic")
	form.Set("Name", "attr-topic")
	rr := postSNS(t, s, form)

	body := rr.Body.String()
	arnStart := strings.Index(body, "<TopicArn>") + len("<TopicArn>")
	arnEnd := strings.Index(body, "</TopicArn>")
	topicArn := body[arnStart:arnEnd]

	// Get attributes
	form = url.Values{}
	form.Set("Action", "GetTopicAttributes")
	form.Set("TopicArn", topicArn)
	rr = postSNS(t, s, form)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	if !strings.Contains(rr.Body.String(), "GetTopicAttributesResponse") {
		t.Errorf("expected GetTopicAttributesResponse, got: %s", rr.Body.String())
	}

	// Verify that actual attributes are returned (not empty)
	if !strings.Contains(rr.Body.String(), "<key>TopicArn</key>") {
		t.Errorf("expected TopicArn attribute in response, got: %s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "attr-topic") {
		t.Errorf("expected topic ARN value in response, got: %s", rr.Body.String())
	}
}

// --- SetTopicAttributes ---

func TestSetTopicAttributes(t *testing.T) {
	s := newTestServer()

	// Create topic
	form := url.Values{}
	form.Set("Action", "CreateTopic")
	form.Set("Name", "set-attr-topic")
	rr := postSNS(t, s, form)

	body := rr.Body.String()
	arnStart := strings.Index(body, "<TopicArn>") + len("<TopicArn>")
	arnEnd := strings.Index(body, "</TopicArn>")
	topicArn := body[arnStart:arnEnd]

	// Set attribute
	form = url.Values{}
	form.Set("Action", "SetTopicAttributes")
	form.Set("TopicArn", topicArn)
	form.Set("AttributeName", "DisplayName")
	form.Set("AttributeValue", "My Topic")
	rr = postSNS(t, s, form)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

// --- Unknown action ---

func TestUnknownAction(t *testing.T) {
	s := newTestServer()

	form := url.Values{}
	form.Set("Action", "DoSomethingWeird")
	rr := postSNS(t, s, form)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// --- Method not allowed ---

func TestMethodNotAllowed(t *testing.T) {
	s := newTestServer()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}
