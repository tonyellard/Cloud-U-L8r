// SPDX-License-Identifier: Apache-2.0
package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func newTestHandler() http.Handler {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewRouter(logger, Config{
		Region:      "us-east-1",
		AccountID:   "000000000000",
		SQSEndpoint: "http://localhost:9320",
		SNSEndpoint: "http://localhost:9330",
	})
}

func postJSON(handler http.Handler, target string, payload any) *httptest.ResponseRecorder {
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", target)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestHealthEndpoint(t *testing.T) {
	handler := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestCreateAndGetScheduleGroup(t *testing.T) {
	handler := newTestHandler()

	// Create group
	rr := postJSON(handler, "AWSScheduler.CreateScheduleGroup", map[string]string{
		"Name": "test-group",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("create group: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Get group
	rr = postJSON(handler, "AWSScheduler.GetScheduleGroup", map[string]string{
		"Name": "test-group",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("get group: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var group struct {
		Name  string `json:"Name"`
		State string `json:"State"`
	}
	json.NewDecoder(rr.Body).Decode(&group)
	if group.Name != "test-group" {
		t.Errorf("expected 'test-group', got %s", group.Name)
	}
}

func TestCreateAndGetSchedule(t *testing.T) {
	handler := newTestHandler()

	// Create schedule
	rr := postJSON(handler, "AWSScheduler.CreateSchedule", map[string]any{
		"Name":               "my-schedule",
		"ScheduleExpression": "rate(5 minutes)",
		"Target": map[string]string{
			"Arn": "arn:aws:sqs:us-east-1:000000000000:my-queue",
		},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("create schedule: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Get schedule
	rr = postJSON(handler, "AWSScheduler.GetSchedule", map[string]string{
		"Name": "my-schedule",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("get schedule: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var sched struct {
		Name               string `json:"Name"`
		ScheduleExpression string `json:"ScheduleExpression"`
		State              string `json:"State"`
	}
	json.NewDecoder(rr.Body).Decode(&sched)
	if sched.Name != "my-schedule" {
		t.Errorf("expected 'my-schedule', got %s", sched.Name)
	}
	if sched.ScheduleExpression != "rate(5 minutes)" {
		t.Errorf("expected 'rate(5 minutes)', got %s", sched.ScheduleExpression)
	}
}

func TestCreateSchedule_InvalidExpression(t *testing.T) {
	handler := newTestHandler()

	rr := postJSON(handler, "AWSScheduler.CreateSchedule", map[string]any{
		"Name":               "bad-schedule",
		"ScheduleExpression": "invalid(expression)",
	})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestListSchedules(t *testing.T) {
	handler := newTestHandler()

	postJSON(handler, "AWSScheduler.CreateSchedule", map[string]any{
		"Name":               "sched-a",
		"ScheduleExpression": "rate(1 minute)",
	})
	postJSON(handler, "AWSScheduler.CreateSchedule", map[string]any{
		"Name":               "sched-b",
		"ScheduleExpression": "rate(5 minutes)",
	})

	rr := postJSON(handler, "AWSScheduler.ListSchedules", map[string]string{})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp struct {
		Schedules []struct{ Name string } `json:"Schedules"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	if len(resp.Schedules) != 2 {
		t.Errorf("expected 2 schedules, got %d", len(resp.Schedules))
	}
}

func TestDeleteSchedule(t *testing.T) {
	handler := newTestHandler()

	postJSON(handler, "AWSScheduler.CreateSchedule", map[string]any{
		"Name":               "to-delete",
		"ScheduleExpression": "rate(1 minute)",
	})

	rr := postJSON(handler, "AWSScheduler.DeleteSchedule", map[string]string{
		"Name": "to-delete",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify gone
	rr = postJSON(handler, "AWSScheduler.GetSchedule", map[string]string{
		"Name": "to-delete",
	})
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestUpdateSchedule(t *testing.T) {
	handler := newTestHandler()

	postJSON(handler, "AWSScheduler.CreateSchedule", map[string]any{
		"Name":               "updatable",
		"ScheduleExpression": "rate(1 minute)",
	})

	rr := postJSON(handler, "AWSScheduler.UpdateSchedule", map[string]any{
		"Name":               "updatable",
		"ScheduleExpression": "rate(1 hour)",
		"State":              "DISABLED",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	rr = postJSON(handler, "AWSScheduler.GetSchedule", map[string]string{
		"Name": "updatable",
	})
	var sched struct {
		ScheduleExpression string `json:"ScheduleExpression"`
		State              string `json:"State"`
	}
	json.NewDecoder(rr.Body).Decode(&sched)
	if sched.ScheduleExpression != "rate(1 hour)" {
		t.Errorf("expected 'rate(1 hour)', got %s", sched.ScheduleExpression)
	}
	if sched.State != "DISABLED" {
		t.Errorf("expected DISABLED, got %s", sched.State)
	}
}

func TestAdminSummary(t *testing.T) {
	handler := newTestHandler()

	postJSON(handler, "AWSScheduler.CreateSchedule", map[string]any{
		"Name":               "s1",
		"ScheduleExpression": "rate(1 minute)",
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/api/summary", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	body, _ := io.ReadAll(rr.Body)
	var summary struct {
		Service        string `json:"service"`
		ScheduleGroups int    `json:"scheduleGroups"`
		Schedules      int    `json:"schedules"`
	}
	json.Unmarshal(body, &summary)
	if summary.Service != "scheduler" {
		t.Errorf("expected 'scheduler', got %s", summary.Service)
	}
	if summary.Schedules != 1 {
		t.Errorf("expected 1 schedule, got %d", summary.Schedules)
	}
}

func TestAdminResources(t *testing.T) {
	handler := newTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/admin/api/resources", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestMissingTarget(t *testing.T) {
	handler := newTestHandler()

	rr := postJSON(handler, "", nil)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing target, got %d", rr.Code)
	}
}

func TestUnsupportedTarget(t *testing.T) {
	handler := newTestHandler()

	rr := postJSON(handler, "AWSScheduler.Unknown", map[string]string{})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for unknown target, got %d", rr.Code)
	}
}
