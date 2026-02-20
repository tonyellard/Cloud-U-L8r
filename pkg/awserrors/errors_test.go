// SPDX-License-Identifier: Apache-2.0

package awserrors

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteXML(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test-bucket/key", nil)
	req.Header.Set("X-Request-ID", "req-123")
	rr := httptest.NewRecorder()

	WriteXML(rr, req, "NoSuchKey", "The specified key does not exist.", http.StatusNotFound)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/xml" {
		t.Errorf("expected Content-Type application/xml, got %s", ct)
	}

	var xmlErr XMLError
	if err := xml.Unmarshal(rr.Body.Bytes(), &xmlErr); err != nil {
		t.Fatalf("failed to unmarshal XML: %v", err)
	}
	if xmlErr.Code != "NoSuchKey" {
		t.Errorf("expected Code NoSuchKey, got %s", xmlErr.Code)
	}
	if xmlErr.Resource != "/test-bucket/key" {
		t.Errorf("expected Resource /test-bucket/key, got %s", xmlErr.Resource)
	}
}

func TestWriteXMLWrapped(t *testing.T) {
	rr := httptest.NewRecorder()

	WriteXMLWrapped(rr, "InvalidParameterValue", "bad value", http.StatusBadRequest)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "text/xml" {
		t.Errorf("expected Content-Type text/xml, got %s", ct)
	}

	var resp XMLErrorResponse
	if err := xml.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal XML: %v", err)
	}
	if resp.Error.Type != "Sender" {
		t.Errorf("expected Type Sender, got %s", resp.Error.Type)
	}
	if resp.Error.Code != "InvalidParameterValue" {
		t.Errorf("expected Code InvalidParameterValue, got %s", resp.Error.Code)
	}
}

func TestWriteCloudFrontXML(t *testing.T) {
	rr := httptest.NewRecorder()

	WriteCloudFrontXML(rr, "AccessDenied", "Access Denied", "CF-REQ-123", http.StatusForbidden)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "AccessDenied") {
		t.Error("expected body to contain AccessDenied")
	}
	if !strings.Contains(body, "CF-REQ-123") {
		t.Error("expected body to contain request ID")
	}
}

func TestWriteJSON(t *testing.T) {
	rr := httptest.NewRecorder()

	WriteJSON(rr, http.StatusBadRequest, "ValidationException", "1 validation error")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/x-amz-json-1.1" {
		t.Errorf("expected Content-Type application/x-amz-json-1.1, got %s", ct)
	}

	var jsonErr JSONError
	if err := json.Unmarshal(rr.Body.Bytes(), &jsonErr); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if jsonErr.Type != "ValidationException" {
		t.Errorf("expected __type ValidationException, got %s", jsonErr.Type)
	}
	if jsonErr.Message != "1 validation error" {
		t.Errorf("expected message '1 validation error', got '%s'", jsonErr.Message)
	}
}

func TestWriteJSONGeneric(t *testing.T) {
	rr := httptest.NewRecorder()

	WriteJSONGeneric(rr, http.StatusInternalServerError, errors.New("something broke"))

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}

	var result map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if result["error"] != "something broke" {
		t.Errorf("expected error 'something broke', got '%s'", result["error"])
	}
}

func TestMapGoErrorToAWS(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantType   string
	}{
		{
			name:       "nil error",
			err:        nil,
			wantStatus: http.StatusInternalServerError,
			wantType:   "InternalFailure",
		},
		{
			name:       "validation exception",
			err:        errors.New("ValidationException: param is required"),
			wantStatus: http.StatusBadRequest,
			wantType:   "ValidationException",
		},
		{
			name:       "not found",
			err:        errors.New("ParameterNotFound: param does not exist"),
			wantStatus: http.StatusNotFound,
			wantType:   "ParameterNotFound",
		},
		{
			name:       "unknown error type",
			err:        errors.New("something went wrong"),
			wantStatus: http.StatusInternalServerError,
			wantType:   "InternalFailure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			MapGoErrorToAWS(rr, tt.err)

			if rr.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rr.Code)
			}

			var jsonErr JSONError
			if err := json.Unmarshal(rr.Body.Bytes(), &jsonErr); err != nil {
				t.Fatalf("failed to unmarshal JSON: %v", err)
			}
			if jsonErr.Type != tt.wantType {
				t.Errorf("expected __type %s, got %s", tt.wantType, jsonErr.Type)
			}
		})
	}
}
