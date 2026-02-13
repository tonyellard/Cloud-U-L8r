package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	router := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
	if body["service"] != "kay-vee" {
		t.Fatalf("expected service kay-vee, got %q", body["service"])
	}
}

func TestAWSJSONRequiresTargetHeader(t *testing.T) {
	router := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{}`))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}

	var awsErr map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &awsErr); err != nil {
		t.Fatalf("failed to parse error body: %v", err)
	}
	if awsErr["__type"] != "ValidationException" {
		t.Fatalf("expected ValidationException, got %v", awsErr["__type"])
	}
}

func TestUnsupportedTargetReturnsValidationException(t *testing.T) {
	router := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{}`))
	req.Header.Set("X-Amz-Target", "AmazonSSM.NotReal")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}

	var awsErr map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &awsErr); err != nil {
		t.Fatalf("failed to parse error body: %v", err)
	}
	if awsErr["__type"] != "ValidationException" {
		t.Fatalf("expected ValidationException, got %v", awsErr["__type"])
	}
}

func TestPutThenGetParameterViaTargets(t *testing.T) {
	router := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))

	putBody := `{"Name":"/app/dev/url","Type":"String","Value":"http://localhost","Overwrite":true}`
	putReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(putBody))
	putReq.Header.Set("X-Amz-Target", "AmazonSSM.PutParameter")
	putRR := httptest.NewRecorder()
	router.ServeHTTP(putRR, putReq)

	if putRR.Code != http.StatusOK {
		t.Fatalf("expected put status 200, got %d body=%s", putRR.Code, putRR.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"Name":"/app/dev/url","WithDecryption":false}`))
	getReq.Header.Set("X-Amz-Target", "AmazonSSM.GetParameter")
	getRR := httptest.NewRecorder()
	router.ServeHTTP(getRR, getReq)

	if getRR.Code != http.StatusOK {
		t.Fatalf("expected get status 200, got %d body=%s", getRR.Code, getRR.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(getRR.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse get response: %v", err)
	}
	param, ok := payload["Parameter"].(map[string]any)
	if !ok {
		t.Fatalf("missing Parameter object in response: %v", payload)
	}
	if param["Name"] != "/app/dev/url" {
		t.Fatalf("expected parameter name /app/dev/url, got %v", param["Name"])
	}
}

func TestGetParameterNotFoundMapsTo404(t *testing.T) {
	router := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"Name":"/does/not/exist"}`))
	req.Header.Set("X-Amz-Target", "AmazonSSM.GetParameter")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rr.Code)
	}

	var awsErr map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &awsErr); err != nil {
		t.Fatalf("failed to parse error body: %v", err)
	}
	if awsErr["__type"] != "ParameterNotFound" {
		t.Fatalf("expected ParameterNotFound, got %v", awsErr["__type"])
	}
}

func TestDescribeSecretNotFoundMapsTo404(t *testing.T) {
	router := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"SecretId":"missing"}`))
	req.Header.Set("X-Amz-Target", "secretsmanager.DescribeSecret")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rr.Code)
	}

	var awsErr map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &awsErr); err != nil {
		t.Fatalf("failed to parse error body: %v", err)
	}
	if awsErr["__type"] != "ResourceNotFoundException" {
		t.Fatalf("expected ResourceNotFoundException, got %v", awsErr["__type"])
	}
}
