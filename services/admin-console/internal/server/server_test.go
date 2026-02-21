// SPDX-License-Identifier: Apache-2.0

package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testRouter() http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewRouter(logger)
}

func TestHealthEndpoint(t *testing.T) {
	router := testRouter()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status=ok, got %s", resp["status"])
	}
	if resp["service"] != "admin-console" {
		t.Errorf("expected service=admin-console, got %s", resp["service"])
	}
}

func TestWriteJSON(t *testing.T) {
	rr := httptest.NewRecorder()

	payload := map[string]string{"key": "value"}
	writeJSON(rr, http.StatusOK, payload)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if resp["key"] != "value" {
		t.Errorf("expected key=value, got %s", resp["key"])
	}
}

func TestWriteJSON_StatusCodes(t *testing.T) {
	tests := []struct {
		code int
	}{
		{http.StatusOK},
		{http.StatusCreated},
		{http.StatusBadRequest},
		{http.StatusInternalServerError},
	}

	for _, tt := range tests {
		rr := httptest.NewRecorder()
		writeJSON(rr, tt.code, map[string]string{"test": "true"})
		if rr.Code != tt.code {
			t.Errorf("expected %d, got %d", tt.code, rr.Code)
		}
	}
}

// TestDashboardSummary tests the dashboard summary endpoint.
// Since the handler calls external services, it will get connection-refused errors
// but should still return a valid JSON response with services showing as offline.
func TestDashboardSummary(t *testing.T) {
	router := testRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/summary", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	// The response should be valid JSON even if services are offline
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode dashboard summary: %v", err)
	}

	// Should contain services section
	if _, ok := resp["services"]; !ok {
		t.Error("expected 'services' key in dashboard summary")
	}
}
