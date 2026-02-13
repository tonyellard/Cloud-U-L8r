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

func TestDeleteParameterViaTarget(t *testing.T) {
	router := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))

	putReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"Name":"/tmp/delete","Type":"String","Value":"1","Overwrite":true}`))
	putReq.Header.Set("X-Amz-Target", "AmazonSSM.PutParameter")
	putRR := httptest.NewRecorder()
	router.ServeHTTP(putRR, putReq)
	if putRR.Code != http.StatusOK {
		t.Fatalf("expected put status 200, got %d", putRR.Code)
	}

	delReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"Name":"/tmp/delete"}`))
	delReq.Header.Set("X-Amz-Target", "AmazonSSM.DeleteParameter")
	delRR := httptest.NewRecorder()
	router.ServeHTTP(delRR, delReq)
	if delRR.Code != http.StatusOK {
		t.Fatalf("expected delete status 200, got %d body=%s", delRR.Code, delRR.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"Name":"/tmp/delete"}`))
	getReq.Header.Set("X-Amz-Target", "AmazonSSM.GetParameter")
	getRR := httptest.NewRecorder()
	router.ServeHTTP(getRR, getReq)
	if getRR.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 after delete, got %d", getRR.Code)
	}
}

func TestDeleteAndRestoreSecretViaTargets(t *testing.T) {
	router := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))

	createReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"Name":"svc/route-delete","SecretString":"v1"}`))
	createReq.Header.Set("X-Amz-Target", "secretsmanager.CreateSecret")
	createRR := httptest.NewRecorder()
	router.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusOK {
		t.Fatalf("expected create status 200, got %d body=%s", createRR.Code, createRR.Body.String())
	}

	delReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"SecretId":"svc/route-delete"}`))
	delReq.Header.Set("X-Amz-Target", "secretsmanager.DeleteSecret")
	delRR := httptest.NewRecorder()
	router.ServeHTTP(delRR, delReq)
	if delRR.Code != http.StatusOK {
		t.Fatalf("expected delete secret status 200, got %d body=%s", delRR.Code, delRR.Body.String())
	}

	restoreReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"SecretId":"svc/route-delete"}`))
	restoreReq.Header.Set("X-Amz-Target", "secretsmanager.RestoreSecret")
	restoreRR := httptest.NewRecorder()
	router.ServeHTTP(restoreRR, restoreReq)
	if restoreRR.Code != http.StatusOK {
		t.Fatalf("expected restore secret status 200, got %d body=%s", restoreRR.Code, restoreRR.Body.String())
	}
}

func TestAdminSummaryEndpoint(t *testing.T) {
	router := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))

	putReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"Name":"/sum/item","Type":"String","Value":"1","Overwrite":true}`))
	putReq.Header.Set("X-Amz-Target", "AmazonSSM.PutParameter")
	putRR := httptest.NewRecorder()
	router.ServeHTTP(putRR, putReq)
	if putRR.Code != http.StatusOK {
		t.Fatalf("expected put status 200, got %d", putRR.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/api/summary", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected admin summary status 200, got %d", rr.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse summary payload: %v", err)
	}
	if payload["parameters"] == nil {
		t.Fatalf("expected parameters count in summary payload")
	}
}
