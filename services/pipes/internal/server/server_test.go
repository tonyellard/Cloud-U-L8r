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

	"github.com/tonyellard/pipes/internal/model"
)

func testRouter() http.Handler {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := Config{
		Region:              "us-east-1",
		AccountID:           "000000000000",
		SQSEndpoint:         "http://localhost:9320",
		SNSEndpoint:         "http://localhost:9330",
		EventBridgeEndpoint: "http://localhost:9340",
	}
	return NewRouter(logger, cfg)
}

func TestHealthEndpoint(t *testing.T) {
	router := testRouter()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestCreateAndDescribePipe(t *testing.T) {
	router := testRouter()

	// Create
	createReq := model.CreatePipeRequest{
		Source:  "arn:aws:sqs:us-east-1:000000000000:my-source",
		Target:  "arn:aws:sqs:us-east-1:000000000000:my-target",
		RoleArn: "arn:aws:iam::000000000000:role/pipe-role",
	}
	body, _ := json.Marshal(createReq)
	req := httptest.NewRequest(http.MethodPost, "/v1/pipes/test-pipe", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var createResp model.CreatePipeResponse
	json.NewDecoder(rr.Body).Decode(&createResp)
	if createResp.Name != "test-pipe" {
		t.Errorf("expected name test-pipe, got %s", createResp.Name)
	}
	if createResp.CurrentState != "RUNNING" {
		t.Errorf("expected RUNNING, got %s", createResp.CurrentState)
	}

	// Describe
	req = httptest.NewRequest(http.MethodGet, "/v1/pipes/test-pipe", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("describe: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var descResp model.DescribePipeResponse
	json.NewDecoder(rr.Body).Decode(&descResp)
	if descResp.Source != "arn:aws:sqs:us-east-1:000000000000:my-source" {
		t.Errorf("wrong source: %s", descResp.Source)
	}
	if descResp.SourceParameters == nil {
		t.Error("SourceParameters should not be nil")
	}
	if descResp.TargetParameters == nil {
		t.Error("TargetParameters should not be nil")
	}
}

func TestCreatePipeConflict(t *testing.T) {
	router := testRouter()

	createReq := model.CreatePipeRequest{
		Source:  "arn:aws:sqs:us-east-1:000000000000:q",
		Target:  "arn:aws:sqs:us-east-1:000000000000:q2",
		RoleArn: "arn:aws:iam::000000000000:role/r",
	}
	body, _ := json.Marshal(createReq)

	// First create
	req := httptest.NewRequest(http.MethodPost, "/v1/pipes/dupe", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("first create failed: %d", rr.Code)
	}

	// Second create — conflict
	req = httptest.NewRequest(http.MethodPost, "/v1/pipes/dupe", bytes.NewReader(body))
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestDescribePipeNotFound(t *testing.T) {
	router := testRouter()
	req := httptest.NewRequest(http.MethodGet, "/v1/pipes/nonexistent", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestUpdatePipe(t *testing.T) {
	router := testRouter()

	// Create first
	createBody, _ := json.Marshal(model.CreatePipeRequest{
		Source:  "arn:aws:sqs:us-east-1:000000000000:q",
		Target:  "arn:aws:sqs:us-east-1:000000000000:q2",
		RoleArn: "arn:aws:iam::000000000000:role/r",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/pipes/upd-pipe", bytes.NewReader(createBody))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Update
	updateBody, _ := json.Marshal(model.UpdatePipeRequest{
		Description:  "updated desc",
		DesiredState: "STOPPED",
		RoleArn:      "arn:aws:iam::000000000000:role/r",
	})
	req = httptest.NewRequest(http.MethodPut, "/v1/pipes/upd-pipe", bytes.NewReader(updateBody))
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp model.UpdatePipeResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.CurrentState != "STOPPED" {
		t.Errorf("expected STOPPED, got %s", resp.CurrentState)
	}
}

func TestDeletePipe(t *testing.T) {
	router := testRouter()

	// Create
	createBody, _ := json.Marshal(model.CreatePipeRequest{
		Source:  "arn:aws:sqs:us-east-1:000000000000:q",
		Target:  "arn:aws:sqs:us-east-1:000000000000:q2",
		RoleArn: "arn:aws:iam::000000000000:role/r",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/pipes/del-pipe", bytes.NewReader(createBody))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Delete
	req = httptest.NewRequest(http.MethodDelete, "/v1/pipes/del-pipe", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify gone
	req = httptest.NewRequest(http.MethodGet, "/v1/pipes/del-pipe", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", rr.Code)
	}
}

func TestListPipes(t *testing.T) {
	router := testRouter()

	// Create two pipes
	for _, name := range []string{"alpha", "beta"} {
		body, _ := json.Marshal(model.CreatePipeRequest{
			Source:  "arn:aws:sqs:us-east-1:000000000000:q",
			Target:  "arn:aws:sqs:us-east-1:000000000000:q2",
			RoleArn: "arn:aws:iam::000000000000:role/r",
		})
		req := httptest.NewRequest(http.MethodPost, "/v1/pipes/"+name, bytes.NewReader(body))
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
	}

	// List
	req := httptest.NewRequest(http.MethodGet, "/v1/pipes", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", rr.Code)
	}

	var resp model.ListPipesResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if len(resp.Pipes) != 2 {
		t.Errorf("expected 2 pipes, got %d", len(resp.Pipes))
	}
}

func TestStartStopPipe(t *testing.T) {
	router := testRouter()

	// Create
	body, _ := json.Marshal(model.CreatePipeRequest{
		Source:  "arn:aws:sqs:us-east-1:000000000000:q",
		Target:  "arn:aws:sqs:us-east-1:000000000000:q2",
		RoleArn: "arn:aws:iam::000000000000:role/r",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/pipes/toggle", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Stop
	req = httptest.NewRequest(http.MethodPost, "/v1/pipes/toggle/stop", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("stop: expected 200, got %d", rr.Code)
	}
	var resp model.UpdatePipeResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.CurrentState != "STOPPED" {
		t.Errorf("expected STOPPED, got %s", resp.CurrentState)
	}

	// Start
	req = httptest.NewRequest(http.MethodPost, "/v1/pipes/toggle/start", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("start: expected 200, got %d", rr.Code)
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.CurrentState != "RUNNING" {
		t.Errorf("expected RUNNING, got %s", resp.CurrentState)
	}
}

func TestTagOperations(t *testing.T) {
	router := testRouter()

	// Create pipe
	body, _ := json.Marshal(model.CreatePipeRequest{
		Source:  "arn:aws:sqs:us-east-1:000000000000:q",
		Target:  "arn:aws:sqs:us-east-1:000000000000:q2",
		RoleArn: "arn:aws:iam::000000000000:role/r",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/pipes/tag-pipe", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	var createResp model.CreatePipeResponse
	json.NewDecoder(rr.Body).Decode(&createResp)
	arn := createResp.Arn

	// Tag
	tagBody, _ := json.Marshal(map[string]interface{}{
		"tags": map[string]string{"env": "prod"},
	})
	req = httptest.NewRequest(http.MethodPost, "/tags/"+arn, bytes.NewReader(tagBody))
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("tag: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// List tags
	req = httptest.NewRequest(http.MethodGet, "/tags/"+arn, nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("list tags: expected 200, got %d", rr.Code)
	}

	var tagResp struct {
		Tags map[string]string `json:"tags"`
	}
	json.NewDecoder(rr.Body).Decode(&tagResp)
	if tagResp.Tags["env"] != "prod" {
		t.Errorf("expected tag env=prod, got %v", tagResp.Tags)
	}

	// Untag
	req = httptest.NewRequest(http.MethodDelete, "/tags/"+arn+"?tagKeys=env", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("untag: expected 200, got %d", rr.Code)
	}
}

func TestCreatePipeWithSourceParameters(t *testing.T) {
	router := testRouter()

	createReq := model.CreatePipeRequest{
		Source:  "arn:aws:sqs:us-east-1:000000000000:q",
		Target:  "arn:aws:sqs:us-east-1:000000000000:q2",
		RoleArn: "arn:aws:iam::000000000000:role/r",
		SourceParameters: &model.SourceParameters{
			SqsQueueParameters: &model.SqsQueueParameters{
				BatchSize: 5,
			},
			FilterCriteria: &model.FilterCriteria{
				Filters: []model.Filter{
					{Pattern: `{"body": {"priority": ["high"]}}`},
				},
			},
		},
	}
	body, _ := json.Marshal(createReq)
	req := httptest.NewRequest(http.MethodPost, "/v1/pipes/param-pipe", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Describe to verify
	req = httptest.NewRequest(http.MethodGet, "/v1/pipes/param-pipe", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	var desc model.DescribePipeResponse
	json.NewDecoder(rr.Body).Decode(&desc)

	if desc.SourceParameters == nil || desc.SourceParameters.SqsQueueParameters == nil {
		t.Fatal("SourceParameters.SqsQueueParameters should not be nil")
	}
	if desc.SourceParameters.SqsQueueParameters.BatchSize != 5 {
		t.Errorf("expected batch size 5, got %d", desc.SourceParameters.SqsQueueParameters.BatchSize)
	}
	if desc.SourceParameters.FilterCriteria == nil || len(desc.SourceParameters.FilterCriteria.Filters) != 1 {
		t.Error("expected 1 filter in SourceParameters.FilterCriteria")
	}
}

func TestAdminSummary(t *testing.T) {
	router := testRouter()
	req := httptest.NewRequest(http.MethodGet, "/admin/api/summary", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var summary model.AdminSummaryResponse
	json.NewDecoder(rr.Body).Decode(&summary)
	if summary.Service != "pipes" {
		t.Errorf("expected service pipes, got %s", summary.Service)
	}
}

func TestInvalidPipeName(t *testing.T) {
	router := testRouter()

	body, _ := json.Marshal(model.CreatePipeRequest{
		Source:  "arn:aws:sqs:us-east-1:000000000000:q",
		Target:  "arn:aws:sqs:us-east-1:000000000000:q2",
		RoleArn: "arn:aws:iam::000000000000:role/r",
	})

	// Name with special characters
	req := httptest.NewRequest(http.MethodPost, "/v1/pipes/bad@name!", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid name, got %d: %s", rr.Code, rr.Body.String())
	}
}
