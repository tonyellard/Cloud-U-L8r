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

func TestLabelParameterVersionViaTarget(t *testing.T) {
	router := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))

	firstPut := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"Name":"/label/p","Type":"String","Value":"v1","Overwrite":true}`))
	firstPut.Header.Set("X-Amz-Target", "AmazonSSM.PutParameter")
	firstRR := httptest.NewRecorder()
	router.ServeHTTP(firstRR, firstPut)
	if firstRR.Code != http.StatusOK {
		t.Fatalf("expected first put status 200, got %d", firstRR.Code)
	}

	secondPut := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"Name":"/label/p","Type":"String","Value":"v2","Overwrite":true}`))
	secondPut.Header.Set("X-Amz-Target", "AmazonSSM.PutParameter")
	secondRR := httptest.NewRecorder()
	router.ServeHTTP(secondRR, secondPut)
	if secondRR.Code != http.StatusOK {
		t.Fatalf("expected second put status 200, got %d", secondRR.Code)
	}

	labelReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"Name":"/label/p","Labels":["stable"],"ParameterVersion":1}`))
	labelReq.Header.Set("X-Amz-Target", "AmazonSSM.LabelParameterVersion")
	labelRR := httptest.NewRecorder()
	router.ServeHTTP(labelRR, labelReq)
	if labelRR.Code != http.StatusOK {
		t.Fatalf("expected label status 200, got %d body=%s", labelRR.Code, labelRR.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"Name":"/label/p:stable"}`))
	getReq.Header.Set("X-Amz-Target", "AmazonSSM.GetParameter")
	getRR := httptest.NewRecorder()
	router.ServeHTTP(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("expected get-by-label status 200, got %d body=%s", getRR.Code, getRR.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(getRR.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse get-by-label payload: %v", err)
	}
	param := payload["Parameter"].(map[string]any)
	if param["Value"] != "v1" {
		t.Fatalf("expected labeled value v1, got %v", param["Value"])
	}
}

func TestUpdateSecretVersionStageViaTarget(t *testing.T) {
	router := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))

	createReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"Name":"svc/stage-route","SecretString":"one"}`))
	createReq.Header.Set("X-Amz-Target", "secretsmanager.CreateSecret")
	createRR := httptest.NewRecorder()
	router.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusOK {
		t.Fatalf("expected create status 200, got %d body=%s", createRR.Code, createRR.Body.String())
	}

	putReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"SecretId":"svc/stage-route","SecretString":"two"}`))
	putReq.Header.Set("X-Amz-Target", "secretsmanager.PutSecretValue")
	putRR := httptest.NewRecorder()
	router.ServeHTTP(putRR, putReq)
	if putRR.Code != http.StatusOK {
		t.Fatalf("expected put secret status 200, got %d body=%s", putRR.Code, putRR.Body.String())
	}

	var createPayload map[string]any
	if err := json.Unmarshal(createRR.Body.Bytes(), &createPayload); err != nil {
		t.Fatalf("failed to parse create payload: %v", err)
	}
	var putPayload map[string]any
	if err := json.Unmarshal(putRR.Body.Bytes(), &putPayload); err != nil {
		t.Fatalf("failed to parse put payload: %v", err)
	}
	createdVersion := createPayload["VersionId"].(string)
	putVersion := putPayload["VersionId"].(string)

	updateStageReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"SecretId":"svc/stage-route","VersionStage":"AWSCURRENT","MoveToVersionId":"`+createdVersion+`","RemoveFromVersionId":"`+putVersion+`"}`))
	updateStageReq.Header.Set("X-Amz-Target", "secretsmanager.UpdateSecretVersionStage")
	updateStageRR := httptest.NewRecorder()
	router.ServeHTTP(updateStageRR, updateStageReq)
	if updateStageRR.Code != http.StatusOK {
		t.Fatalf("expected update stage status 200, got %d body=%s", updateStageRR.Code, updateStageRR.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"SecretId":"svc/stage-route"}`))
	getReq.Header.Set("X-Amz-Target", "secretsmanager.GetSecretValue")
	getRR := httptest.NewRecorder()
	router.ServeHTTP(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("expected get secret status 200, got %d body=%s", getRR.Code, getRR.Body.String())
	}

	var getPayload map[string]any
	if err := json.Unmarshal(getRR.Body.Bytes(), &getPayload); err != nil {
		t.Fatalf("failed to parse get payload: %v", err)
	}
	if getPayload["SecretString"] != "one" {
		t.Fatalf("expected AWSCURRENT to move back to one, got %v", getPayload["SecretString"])
	}
}

func TestDescribeParametersViaTarget(t *testing.T) {
	router := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))

	putReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"Name":"/describe/p","Type":"String","Value":"v","Overwrite":true}`))
	putReq.Header.Set("X-Amz-Target", "AmazonSSM.PutParameter")
	putRR := httptest.NewRecorder()
	router.ServeHTTP(putRR, putReq)
	if putRR.Code != http.StatusOK {
		t.Fatalf("expected put status 200, got %d", putRR.Code)
	}

	describeReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{}`))
	describeReq.Header.Set("X-Amz-Target", "AmazonSSM.DescribeParameters")
	describeRR := httptest.NewRecorder()
	router.ServeHTTP(describeRR, describeReq)
	if describeRR.Code != http.StatusOK {
		t.Fatalf("expected describe status 200, got %d body=%s", describeRR.Code, describeRR.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(describeRR.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse describe payload: %v", err)
	}
	params, ok := payload["Parameters"].([]any)
	if !ok || len(params) == 0 {
		t.Fatalf("expected parameters in describe payload: %v", payload)
	}
}

func TestGetParameterHistoryViaTarget(t *testing.T) {
	router := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))

	firstPut := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"Name":"/history/p","Type":"String","Value":"v1","Overwrite":true}`))
	firstPut.Header.Set("X-Amz-Target", "AmazonSSM.PutParameter")
	firstRR := httptest.NewRecorder()
	router.ServeHTTP(firstRR, firstPut)
	if firstRR.Code != http.StatusOK {
		t.Fatalf("expected first put status 200, got %d", firstRR.Code)
	}

	secondPut := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"Name":"/history/p","Type":"String","Value":"v2","Overwrite":true}`))
	secondPut.Header.Set("X-Amz-Target", "AmazonSSM.PutParameter")
	secondRR := httptest.NewRecorder()
	router.ServeHTTP(secondRR, secondPut)
	if secondRR.Code != http.StatusOK {
		t.Fatalf("expected second put status 200, got %d", secondRR.Code)
	}

	historyReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"Name":"/history/p"}`))
	historyReq.Header.Set("X-Amz-Target", "AmazonSSM.GetParameterHistory")
	historyRR := httptest.NewRecorder()
	router.ServeHTTP(historyRR, historyReq)
	if historyRR.Code != http.StatusOK {
		t.Fatalf("expected history status 200, got %d body=%s", historyRR.Code, historyRR.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(historyRR.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse history payload: %v", err)
	}
	history, ok := payload["Parameters"].([]any)
	if !ok || len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %v", payload["Parameters"])
	}
	first := history[0].(map[string]any)
	if first["Value"] != "v1" {
		t.Fatalf("expected first history value v1, got %v", first["Value"])
	}
}

func TestPaginationViaTargets(t *testing.T) {
	router := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))

	putA := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"Name":"/page/a","Type":"String","Value":"a","Overwrite":true}`))
	putA.Header.Set("X-Amz-Target", "AmazonSSM.PutParameter")
	putARR := httptest.NewRecorder()
	router.ServeHTTP(putARR, putA)

	putB := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"Name":"/page/b","Type":"String","Value":"b","Overwrite":true}`))
	putB.Header.Set("X-Amz-Target", "AmazonSSM.PutParameter")
	putBRR := httptest.NewRecorder()
	router.ServeHTTP(putBRR, putB)

	describeReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"MaxResults":1}`))
	describeReq.Header.Set("X-Amz-Target", "AmazonSSM.DescribeParameters")
	describeRR := httptest.NewRecorder()
	router.ServeHTTP(describeRR, describeReq)
	if describeRR.Code != http.StatusOK {
		t.Fatalf("expected describe status 200, got %d body=%s", describeRR.Code, describeRR.Body.String())
	}

	var describePayload map[string]any
	if err := json.Unmarshal(describeRR.Body.Bytes(), &describePayload); err != nil {
		t.Fatalf("failed to parse describe payload: %v", err)
	}
	if describePayload["NextToken"] == nil {
		t.Fatalf("expected NextToken in describe response")
	}

	listCreateOne := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"Name":"svc/page1","SecretString":"one"}`))
	listCreateOne.Header.Set("X-Amz-Target", "secretsmanager.CreateSecret")
	listCreateOneRR := httptest.NewRecorder()
	router.ServeHTTP(listCreateOneRR, listCreateOne)

	listCreateTwo := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"Name":"svc/page2","SecretString":"two"}`))
	listCreateTwo.Header.Set("X-Amz-Target", "secretsmanager.CreateSecret")
	listCreateTwoRR := httptest.NewRecorder()
	router.ServeHTTP(listCreateTwoRR, listCreateTwo)

	listReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"MaxResults":1}`))
	listReq.Header.Set("X-Amz-Target", "secretsmanager.ListSecrets")
	listRR := httptest.NewRecorder()
	router.ServeHTTP(listRR, listReq)
	if listRR.Code != http.StatusOK {
		t.Fatalf("expected list secrets status 200, got %d body=%s", listRR.Code, listRR.Body.String())
	}

	var listPayload map[string]any
	if err := json.Unmarshal(listRR.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("failed to parse list payload: %v", err)
	}
	if listPayload["NextToken"] == nil {
		t.Fatalf("expected NextToken in list response")
	}
}
