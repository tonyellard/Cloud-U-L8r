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

func invokeAWSJSON(t *testing.T, client *http.Client, baseURL, target string, body string) (int, map[string]any) {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, baseURL+"/", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", target)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	payloadBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	payload := map[string]any{}
	if len(payloadBytes) > 0 {
		if err := json.Unmarshal(payloadBytes, &payload); err != nil {
			t.Fatalf("failed to parse response body: %v body=%s", err, string(payloadBytes))
		}
	}

	return resp.StatusCode, payload
}

func TestIntegrationPaginationLoopForDescribeParameters(t *testing.T) {
	router := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(router)
	defer ts.Close()

	client := ts.Client()

	for _, name := range []string{"/sdk/page/a", "/sdk/page/b", "/sdk/page/c"} {
		status, _ := invokeAWSJSON(t, client, ts.URL, "AmazonSSM.PutParameter", `{"Name":"`+name+`","Type":"String","Value":"v","Overwrite":true}`)
		if status != http.StatusOK {
			t.Fatalf("expected put status 200, got %d", status)
		}
	}

	nextToken := ""
	seen := map[string]struct{}{}
	for {
		body := `{"MaxResults":1`
		if nextToken != "" {
			body += `,"NextToken":"` + nextToken + `"`
		}
		body += `}`

		status, payload := invokeAWSJSON(t, client, ts.URL, "AmazonSSM.DescribeParameters", body)
		if status != http.StatusOK {
			t.Fatalf("expected describe status 200, got %d payload=%v", status, payload)
		}

		params, ok := payload["Parameters"].([]any)
		if !ok {
			t.Fatalf("expected Parameters list, got %v", payload)
		}
		for _, item := range params {
			name := item.(map[string]any)["Name"].(string)
			seen[name] = struct{}{}
		}

		nextTokenValue, hasToken := payload["NextToken"]
		if !hasToken {
			break
		}
		nextToken, ok = nextTokenValue.(string)
		if !ok || nextToken == "" {
			break
		}
	}

	if len(seen) != 3 {
		t.Fatalf("expected to collect 3 unique parameters across pages, got %d", len(seen))
	}
}

func TestIntegrationPaginationLoopForListSecrets(t *testing.T) {
	router := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(router)
	defer ts.Close()

	client := ts.Client()

	for _, name := range []string{"sdk/secret/a", "sdk/secret/b", "sdk/secret/c"} {
		status, _ := invokeAWSJSON(t, client, ts.URL, "secretsmanager.CreateSecret", `{"Name":"`+name+`","SecretString":"value"}`)
		if status != http.StatusOK {
			t.Fatalf("expected create status 200, got %d", status)
		}
	}

	nextToken := ""
	seen := map[string]struct{}{}
	for {
		body := `{"MaxResults":1`
		if nextToken != "" {
			body += `,"NextToken":"` + nextToken + `"`
		}
		body += `}`

		status, payload := invokeAWSJSON(t, client, ts.URL, "secretsmanager.ListSecrets", body)
		if status != http.StatusOK {
			t.Fatalf("expected list status 200, got %d payload=%v", status, payload)
		}

		secrets, ok := payload["SecretList"].([]any)
		if !ok {
			t.Fatalf("expected SecretList, got %v", payload)
		}
		for _, item := range secrets {
			name := item.(map[string]any)["Name"].(string)
			seen[name] = struct{}{}
		}

		nextTokenValue, hasToken := payload["NextToken"]
		if !hasToken {
			break
		}
		nextToken, ok = nextTokenValue.(string)
		if !ok || nextToken == "" {
			break
		}
	}

	if len(seen) != 3 {
		t.Fatalf("expected to collect 3 unique secrets across pages, got %d", len(seen))
	}
}

func TestIntegrationInvalidNextTokenReturnsValidationException(t *testing.T) {
	router := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(router)
	defer ts.Close()

	client := ts.Client()

	status, payload := invokeAWSJSON(t, client, ts.URL, "AmazonSSM.DescribeParameters", `{"MaxResults":1,"NextToken":"not-an-int"}`)
	if status != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d payload=%v", status, payload)
	}
	if payload["__type"] != "ValidationException" {
		t.Fatalf("expected ValidationException type, got %v", payload["__type"])
	}
}

func TestIntegrationAdminExportImportRoundTrip(t *testing.T) {
	router := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(router)
	defer ts.Close()

	client := ts.Client()

	status, _ := invokeAWSJSON(t, client, ts.URL, "AmazonSSM.PutParameter", `{"Name":"/admin/rt","Type":"String","Value":"v1","Overwrite":true}`)
	if status != http.StatusOK {
		t.Fatalf("expected put status 200, got %d", status)
	}

	status, _ = invokeAWSJSON(t, client, ts.URL, "secretsmanager.CreateSecret", `{"Name":"admin/rt-secret","SecretString":"sv"}`)
	if status != http.StatusOK {
		t.Fatalf("expected create secret status 200, got %d", status)
	}

	exportReq, err := http.NewRequest(http.MethodGet, ts.URL+"/admin/api/export", nil)
	if err != nil {
		t.Fatalf("failed to create export request: %v", err)
	}
	exportResp, err := client.Do(exportReq)
	if err != nil {
		t.Fatalf("failed to call export: %v", err)
	}
	defer exportResp.Body.Close()
	if exportResp.StatusCode != http.StatusOK {
		t.Fatalf("expected export status 200, got %d", exportResp.StatusCode)
	}
	exportBytes, err := io.ReadAll(exportResp.Body)
	if err != nil {
		t.Fatalf("failed to read export body: %v", err)
	}

	clearPayload := `{"Names":["/admin/rt"]}`
	status, _ = invokeAWSJSON(t, client, ts.URL, "AmazonSSM.DeleteParameters", clearPayload)
	if status != http.StatusOK {
		t.Fatalf("expected delete parameters status 200, got %d", status)
	}

	status, _ = invokeAWSJSON(t, client, ts.URL, "secretsmanager.DeleteSecret", `{"SecretId":"admin/rt-secret"}`)
	if status != http.StatusOK {
		t.Fatalf("expected delete secret status 200, got %d", status)
	}

	importReq, err := http.NewRequest(http.MethodPost, ts.URL+"/admin/api/import", bytes.NewBuffer(exportBytes))
	if err != nil {
		t.Fatalf("failed to create import request: %v", err)
	}
	importReq.Header.Set("Content-Type", "application/json")
	importResp, err := client.Do(importReq)
	if err != nil {
		t.Fatalf("failed to call import: %v", err)
	}
	defer importResp.Body.Close()
	if importResp.StatusCode != http.StatusOK {
		t.Fatalf("expected import status 200, got %d", importResp.StatusCode)
	}

	status, payload := invokeAWSJSON(t, client, ts.URL, "AmazonSSM.GetParameter", `{"Name":"/admin/rt"}`)
	if status != http.StatusOK {
		t.Fatalf("expected get parameter after import status 200, got %d payload=%v", status, payload)
	}

	status, payload = invokeAWSJSON(t, client, ts.URL, "secretsmanager.GetSecretValue", `{"SecretId":"admin/rt-secret"}`)
	if status != http.StatusOK {
		t.Fatalf("expected get secret after import status 200, got %d payload=%v", status, payload)
	}
}

func TestIntegrationAdminActivityEndpoint(t *testing.T) {
	router := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(router)
	defer ts.Close()

	client := ts.Client()

	status, _ := invokeAWSJSON(t, client, ts.URL, "AmazonSSM.PutParameter", `{"Name":"/activity/key","Type":"String","Value":"v","Overwrite":true}`)
	if status != http.StatusOK {
		t.Fatalf("expected put status 200, got %d", status)
	}

	status, _ = invokeAWSJSON(t, client, ts.URL, "AmazonSSM.GetParameter", `{"Name":"/activity/missing"}`)
	if status != http.StatusNotFound {
		t.Fatalf("expected get missing status 404, got %d", status)
	}

	activityReq, err := http.NewRequest(http.MethodGet, ts.URL+"/admin/api/activity?maxResults=1", nil)
	if err != nil {
		t.Fatalf("failed to create activity request: %v", err)
	}
	activityResp, err := client.Do(activityReq)
	if err != nil {
		t.Fatalf("failed to call activity endpoint: %v", err)
	}
	defer activityResp.Body.Close()
	if activityResp.StatusCode != http.StatusOK {
		t.Fatalf("expected activity status 200, got %d", activityResp.StatusCode)
	}

	firstPageBytes, err := io.ReadAll(activityResp.Body)
	if err != nil {
		t.Fatalf("failed to read activity response: %v", err)
	}
	firstPage := map[string]any{}
	if err := json.Unmarshal(firstPageBytes, &firstPage); err != nil {
		t.Fatalf("failed to parse activity response: %v body=%s", err, string(firstPageBytes))
	}

	entries, ok := firstPage["activity"].([]any)
	if !ok || len(entries) != 1 {
		t.Fatalf("expected one activity entry on first page, got %v", firstPage["activity"])
	}
	entry := entries[0].(map[string]any)
	if entry["target"] != "AmazonSSM.GetParameter" {
		t.Fatalf("expected most recent target AmazonSSM.GetParameter, got %v", entry["target"])
	}
	if entry["errorType"] != "ParameterNotFound" {
		t.Fatalf("expected ParameterNotFound error type, got %v", entry["errorType"])
	}

	nextToken, ok := firstPage["nextToken"].(string)
	if !ok || nextToken == "" {
		t.Fatalf("expected nextToken in first activity page, got %v", firstPage["nextToken"])
	}

	secondReq, err := http.NewRequest(http.MethodGet, ts.URL+"/admin/api/activity?maxResults=1&nextToken="+nextToken, nil)
	if err != nil {
		t.Fatalf("failed to create second activity request: %v", err)
	}
	secondResp, err := client.Do(secondReq)
	if err != nil {
		t.Fatalf("failed to call activity endpoint page 2: %v", err)
	}
	defer secondResp.Body.Close()
	if secondResp.StatusCode != http.StatusOK {
		t.Fatalf("expected second activity status 200, got %d", secondResp.StatusCode)
	}

	secondPageBytes, err := io.ReadAll(secondResp.Body)
	if err != nil {
		t.Fatalf("failed to read second activity response: %v", err)
	}
	secondPage := map[string]any{}
	if err := json.Unmarshal(secondPageBytes, &secondPage); err != nil {
		t.Fatalf("failed to parse second activity response: %v body=%s", err, string(secondPageBytes))
	}

	secondEntries, ok := secondPage["activity"].([]any)
	if !ok || len(secondEntries) != 1 {
		t.Fatalf("expected one activity entry on second page, got %v", secondPage["activity"])
	}
	secondEntry := secondEntries[0].(map[string]any)
	if secondEntry["target"] != "AmazonSSM.PutParameter" {
		t.Fatalf("expected older target AmazonSSM.PutParameter, got %v", secondEntry["target"])
	}
}

func TestIntegrationAdminResourcesDoesNotAffectActivity(t *testing.T) {
	router := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(router)
	defer ts.Close()

	client := ts.Client()

	status, _ := invokeAWSJSON(t, client, ts.URL, "AmazonSSM.PutParameter", `{"Name":"/admin/resources/key","Type":"String","Value":"v","Overwrite":true}`)
	if status != http.StatusOK {
		t.Fatalf("expected put status 200, got %d", status)
	}

	status, _ = invokeAWSJSON(t, client, ts.URL, "secretsmanager.CreateSecret", `{"Name":"admin/resources/secret","SecretString":"sv"}`)
	if status != http.StatusOK {
		t.Fatalf("expected create secret status 200, got %d", status)
	}

	activityBeforeReq, err := http.NewRequest(http.MethodGet, ts.URL+"/admin/api/activity?maxResults=100", nil)
	if err != nil {
		t.Fatalf("failed to create activity request: %v", err)
	}
	activityBeforeResp, err := client.Do(activityBeforeReq)
	if err != nil {
		t.Fatalf("failed to call activity endpoint: %v", err)
	}
	defer activityBeforeResp.Body.Close()
	if activityBeforeResp.StatusCode != http.StatusOK {
		t.Fatalf("expected activity status 200, got %d", activityBeforeResp.StatusCode)
	}

	beforeBytes, err := io.ReadAll(activityBeforeResp.Body)
	if err != nil {
		t.Fatalf("failed to read activity response: %v", err)
	}
	beforePayload := map[string]any{}
	if err := json.Unmarshal(beforeBytes, &beforePayload); err != nil {
		t.Fatalf("failed to parse activity response: %v body=%s", err, string(beforeBytes))
	}
	beforeEntries, _ := beforePayload["activity"].([]any)

	resourcesReq, err := http.NewRequest(http.MethodGet, ts.URL+"/admin/api/resources?parameterPath=%2F&recursive=true&parameterMaxResults=10&secretMaxResults=10", nil)
	if err != nil {
		t.Fatalf("failed to create resources request: %v", err)
	}
	resourcesResp, err := client.Do(resourcesReq)
	if err != nil {
		t.Fatalf("failed to call resources endpoint: %v", err)
	}
	defer resourcesResp.Body.Close()
	if resourcesResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resourcesResp.Body)
		t.Fatalf("expected resources status 200, got %d body=%s", resourcesResp.StatusCode, string(body))
	}

	activityAfterReq, err := http.NewRequest(http.MethodGet, ts.URL+"/admin/api/activity?maxResults=100", nil)
	if err != nil {
		t.Fatalf("failed to create second activity request: %v", err)
	}
	activityAfterResp, err := client.Do(activityAfterReq)
	if err != nil {
		t.Fatalf("failed to call activity endpoint: %v", err)
	}
	defer activityAfterResp.Body.Close()
	if activityAfterResp.StatusCode != http.StatusOK {
		t.Fatalf("expected activity status 200, got %d", activityAfterResp.StatusCode)
	}

	afterBytes, err := io.ReadAll(activityAfterResp.Body)
	if err != nil {
		t.Fatalf("failed to read second activity response: %v", err)
	}
	afterPayload := map[string]any{}
	if err := json.Unmarshal(afterBytes, &afterPayload); err != nil {
		t.Fatalf("failed to parse second activity response: %v body=%s", err, string(afterBytes))
	}
	afterEntries, _ := afterPayload["activity"].([]any)

	if len(beforeEntries) != len(afterEntries) {
		t.Fatalf("expected activity count unchanged after /admin/api/resources call, before=%d after=%d", len(beforeEntries), len(afterEntries))
	}
}
