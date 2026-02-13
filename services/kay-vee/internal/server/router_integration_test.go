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
