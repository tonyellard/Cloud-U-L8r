// SPDX-License-Identifier: Apache-2.0
package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func newTestRouter() http.Handler {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewRouter(logger, Config{
		Region:      "us-east-1",
		AccountID:   "123456789012",
		SQSEndpoint: "http://localhost:9320",
		SNSEndpoint: "http://localhost:9330",
	})
}

func postJSON(handler http.Handler, target string, body interface{}) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
	req.Header.Set("X-Amz-Target", target)
	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestHealth(t *testing.T) {
	handler := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]string
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["service"] != "drawbridge" {
		t.Errorf("expected service=drawbridge, got %s", resp["service"])
	}
}

func TestDefaultEventBusExists(t *testing.T) {
	handler := newTestRouter()
	rr := postJSON(handler, "AWSEvents.ListEventBuses", map[string]interface{}{})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		EventBuses []struct {
			Name string `json:"Name"`
		} `json:"EventBuses"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.EventBuses) != 1 {
		t.Fatalf("expected 1 default bus, got %d", len(resp.EventBuses))
	}
	if resp.EventBuses[0].Name != "default" {
		t.Errorf("expected bus name=default, got %s", resp.EventBuses[0].Name)
	}
}

func TestCreateAndDescribeEventBus(t *testing.T) {
	handler := newTestRouter()

	// Create
	rr := postJSON(handler, "AWSEvents.CreateEventBus", map[string]string{"Name": "my-bus"})
	if rr.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var createResp struct {
		EventBusArn string `json:"EventBusArn"`
	}
	json.Unmarshal(rr.Body.Bytes(), &createResp)
	if createResp.EventBusArn == "" {
		t.Error("expected non-empty ARN")
	}

	// Describe
	rr = postJSON(handler, "AWSEvents.DescribeEventBus", map[string]string{"Name": "my-bus"})
	if rr.Code != http.StatusOK {
		t.Fatalf("describe: expected 200, got %d", rr.Code)
	}
}

func TestCreateEventBusDuplicate(t *testing.T) {
	handler := newTestRouter()
	postJSON(handler, "AWSEvents.CreateEventBus", map[string]string{"Name": "dup-bus"})
	rr := postJSON(handler, "AWSEvents.CreateEventBus", map[string]string{"Name": "dup-bus"})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for duplicate, got %d", rr.Code)
	}
}

func TestDeleteEventBus(t *testing.T) {
	handler := newTestRouter()
	postJSON(handler, "AWSEvents.CreateEventBus", map[string]string{"Name": "del-bus"})
	rr := postJSON(handler, "AWSEvents.DeleteEventBus", map[string]string{"Name": "del-bus"})
	if rr.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d", rr.Code)
	}

	// Verify it's gone
	rr = postJSON(handler, "AWSEvents.DescribeEventBus", map[string]string{"Name": "del-bus"})
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", rr.Code)
	}
}

func TestCannotDeleteDefaultBus(t *testing.T) {
	handler := newTestRouter()
	rr := postJSON(handler, "AWSEvents.DeleteEventBus", map[string]string{"Name": "default"})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for default bus delete, got %d", rr.Code)
	}
}

func TestPutRuleAndListRules(t *testing.T) {
	handler := newTestRouter()

	rr := postJSON(handler, "AWSEvents.PutRule", map[string]interface{}{
		"Name":         "test-rule",
		"EventPattern": `{"source": ["my.app"]}`,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("put rule: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var putResp struct {
		RuleArn string `json:"RuleArn"`
	}
	json.Unmarshal(rr.Body.Bytes(), &putResp)
	if putResp.RuleArn == "" {
		t.Error("expected non-empty rule ARN")
	}

	// List
	rr = postJSON(handler, "AWSEvents.ListRules", map[string]interface{}{})
	if rr.Code != http.StatusOK {
		t.Fatalf("list rules: expected 200, got %d", rr.Code)
	}
	var listResp struct {
		Rules []struct {
			Name string `json:"Name"`
		} `json:"Rules"`
	}
	json.Unmarshal(rr.Body.Bytes(), &listResp)
	if len(listResp.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(listResp.Rules))
	}
}

func TestPutRuleRequiresPattern(t *testing.T) {
	handler := newTestRouter()
	rr := postJSON(handler, "AWSEvents.PutRule", map[string]interface{}{
		"Name": "no-pattern",
	})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 without pattern, got %d", rr.Code)
	}
}

func TestEnableDisableRule(t *testing.T) {
	handler := newTestRouter()
	postJSON(handler, "AWSEvents.PutRule", map[string]interface{}{
		"Name":         "toggle-rule",
		"EventPattern": `{"source": ["x"]}`,
	})

	rr := postJSON(handler, "AWSEvents.DisableRule", map[string]string{"Name": "toggle-rule"})
	if rr.Code != http.StatusOK {
		t.Fatalf("disable: expected 200, got %d", rr.Code)
	}

	rr = postJSON(handler, "AWSEvents.DescribeRule", map[string]string{"Name": "toggle-rule"})
	var rule struct {
		State string `json:"State"`
	}
	json.Unmarshal(rr.Body.Bytes(), &rule)
	if rule.State != "DISABLED" {
		t.Errorf("expected DISABLED, got %s", rule.State)
	}

	postJSON(handler, "AWSEvents.EnableRule", map[string]string{"Name": "toggle-rule"})
	rr = postJSON(handler, "AWSEvents.DescribeRule", map[string]string{"Name": "toggle-rule"})
	json.Unmarshal(rr.Body.Bytes(), &rule)
	if rule.State != "ENABLED" {
		t.Errorf("expected ENABLED, got %s", rule.State)
	}
}

func TestPutAndListTargets(t *testing.T) {
	handler := newTestRouter()
	postJSON(handler, "AWSEvents.PutRule", map[string]interface{}{
		"Name":         "targets-rule",
		"EventPattern": `{"source": ["x"]}`,
	})

	rr := postJSON(handler, "AWSEvents.PutTargets", map[string]interface{}{
		"Rule": "targets-rule",
		"Targets": []map[string]string{
			{"Id": "t1", "Arn": "arn:aws:sqs:us-east-1:123456789012:my-queue"},
		},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("put targets: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// List
	rr = postJSON(handler, "AWSEvents.ListTargetsByRule", map[string]string{"Rule": "targets-rule"})
	if rr.Code != http.StatusOK {
		t.Fatalf("list targets: expected 200, got %d", rr.Code)
	}
	var resp struct {
		Targets []struct {
			Id  string `json:"Id"`
			Arn string `json:"Arn"`
		} `json:"Targets"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(resp.Targets))
	}
	if resp.Targets[0].Id != "t1" {
		t.Errorf("expected target id=t1, got %s", resp.Targets[0].Id)
	}
}

func TestRemoveTargets(t *testing.T) {
	handler := newTestRouter()
	postJSON(handler, "AWSEvents.PutRule", map[string]interface{}{
		"Name":         "rm-rule",
		"EventPattern": `{"source": ["x"]}`,
	})
	postJSON(handler, "AWSEvents.PutTargets", map[string]interface{}{
		"Rule": "rm-rule",
		"Targets": []map[string]string{
			{"Id": "t1", "Arn": "arn:aws:sqs:us-east-1:123456789012:q"},
		},
	})

	rr := postJSON(handler, "AWSEvents.RemoveTargets", map[string]interface{}{
		"Rule": "rm-rule",
		"Ids":  []string{"t1"},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("remove targets: expected 200, got %d", rr.Code)
	}

	// Verify empty
	rr = postJSON(handler, "AWSEvents.ListTargetsByRule", map[string]string{"Rule": "rm-rule"})
	var resp struct {
		Targets []interface{} `json:"Targets"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Targets) != 0 {
		t.Errorf("expected 0 targets after removal, got %d", len(resp.Targets))
	}
}

func TestDeleteRuleWithTargetsFails(t *testing.T) {
	handler := newTestRouter()
	postJSON(handler, "AWSEvents.PutRule", map[string]interface{}{
		"Name":         "has-targets",
		"EventPattern": `{"source": ["x"]}`,
	})
	postJSON(handler, "AWSEvents.PutTargets", map[string]interface{}{
		"Rule": "has-targets",
		"Targets": []map[string]string{
			{"Id": "t1", "Arn": "arn:aws:sqs:us-east-1:123456789012:q"},
		},
	})

	rr := postJSON(handler, "AWSEvents.DeleteRule", map[string]string{"Name": "has-targets"})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 deleting rule with targets, got %d", rr.Code)
	}
}

func TestPutEvents(t *testing.T) {
	handler := newTestRouter()

	rr := postJSON(handler, "AWSEvents.PutEvents", map[string]interface{}{
		"Entries": []map[string]string{
			{
				"Source":     "my.app",
				"DetailType": "TestEvent",
				"Detail":     `{"key": "value"}`,
			},
		},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("put events: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		FailedEntryCount int `json:"FailedEntryCount"`
		Entries          []struct {
			EventId string `json:"EventId"`
		} `json:"Entries"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.FailedEntryCount != 0 {
		t.Errorf("expected 0 failures, got %d", resp.FailedEntryCount)
	}
	if len(resp.Entries) != 1 || resp.Entries[0].EventId == "" {
		t.Error("expected 1 entry with non-empty EventId")
	}
}

func TestTestEventPattern(t *testing.T) {
	handler := newTestRouter()

	rr := postJSON(handler, "AWSEvents.TestEventPattern", map[string]string{
		"EventPattern": `{"source": ["my.app"]}`,
		"Event":        `{"source": "my.app", "detail-type": "Test"}`,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("test event pattern: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Result bool `json:"Result"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if !resp.Result {
		t.Error("expected Result=true")
	}
}

func TestTestEventPatternNoMatch(t *testing.T) {
	handler := newTestRouter()

	rr := postJSON(handler, "AWSEvents.TestEventPattern", map[string]string{
		"EventPattern": `{"source": ["other.app"]}`,
		"Event":        `{"source": "my.app"}`,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp struct {
		Result bool `json:"Result"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Result {
		t.Error("expected Result=false")
	}
}

func TestMissingTarget(t *testing.T) {
	handler := newTestRouter()
	rr := postJSON(handler, "AWSEvents.UnknownAction", map[string]string{})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for unknown target, got %d", rr.Code)
	}
}

func TestMissingXAmzTarget(t *testing.T) {
	handler := newTestRouter()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{}`)))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing X-Amz-Target, got %d", rr.Code)
	}
}

func TestAdminSummary(t *testing.T) {
	handler := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/admin/api/summary", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp struct {
		Service    string `json:"service"`
		EventBuses int    `json:"eventBuses"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Service != "drawbridge" {
		t.Errorf("expected service=drawbridge, got %s", resp.Service)
	}
	if resp.EventBuses != 1 {
		t.Errorf("expected 1 bus (default), got %d", resp.EventBuses)
	}
}

func TestAdminResources(t *testing.T) {
	handler := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/admin/api/resources", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestAdminExportImport(t *testing.T) {
	handler := newTestRouter()

	// Create a rule first
	postJSON(handler, "AWSEvents.PutRule", map[string]interface{}{
		"Name":         "export-rule",
		"EventPattern": `{"source": ["x"]}`,
	})

	// Export
	req := httptest.NewRequest(http.MethodGet, "/admin/api/export", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("export: expected 200, got %d", rr.Code)
	}

	// Import into fresh handler
	handler2 := newTestRouter()
	importReq := httptest.NewRequest(http.MethodPost, "/admin/api/import", bytes.NewReader(rr.Body.Bytes()))
	importRR := httptest.NewRecorder()
	handler2.ServeHTTP(importRR, importReq)
	if importRR.Code != http.StatusOK {
		t.Fatalf("import: expected 200, got %d: %s", importRR.Code, importRR.Body.String())
	}
}
