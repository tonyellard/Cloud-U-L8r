package server

// SPDX-License-Identifier: Apache-2.0

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDefaultRootObjectAppliedToNestedDirectoryPath(t *testing.T) {
	t.Parallel()

	requestedPath := make(chan string, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath <- r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	defer origin.Close()

	config := &Config{
		Server: ServerConfig{},
		Origins: []Origin{
			{
				Name:              "s3",
				URL:               origin.URL,
				PathPatterns:      []string{"/s3/*"},
				RequireSignature:  boolPtr(false),
				DefaultRootObject: strPtr("index.html"),
			},
		},
		Signing: SigningConfig{Enabled: false},
	}

	router := SetupRouter(config, nil)
	req := httptest.NewRequest(http.MethodGet, "/s3/tester-dir/", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got=%d body=%s", resp.Code, resp.Body.String())
	}

	select {
	case got := <-requestedPath:
		want := "/s3/tester-dir/index.html"
		if got != want {
			t.Fatalf("unexpected proxied path: got=%s want=%s", got, want)
		}
	default:
		t.Fatal("origin did not receive a request")
	}
}

func TestDefaultRootObjectAppliedToNestedDirectoryPathWithoutTrailingSlash(t *testing.T) {
	t.Parallel()

	requestedPath := make(chan string, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath <- r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	defer origin.Close()

	config := &Config{
		Server: ServerConfig{},
		Origins: []Origin{
			{
				Name:              "s3",
				URL:               origin.URL,
				PathPatterns:      []string{"/s3/*"},
				RequireSignature:  boolPtr(false),
				DefaultRootObject: strPtr("index.html"),
			},
		},
		Signing: SigningConfig{Enabled: false},
	}

	router := SetupRouter(config, nil)
	req := httptest.NewRequest(http.MethodGet, "/s3/tester-dir", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got=%d body=%s", resp.Code, resp.Body.String())
	}

	select {
	case got := <-requestedPath:
		want := "/s3/tester-dir/index.html"
		if got != want {
			t.Fatalf("unexpected proxied path: got=%s want=%s", got, want)
		}
	default:
		t.Fatal("origin did not receive a request")
	}
}

func TestDefaultRootObjectAppliedToRootPath(t *testing.T) {
	t.Parallel()

	requestedPath := make(chan string, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath <- r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	defer origin.Close()

	config := &Config{
		Server: ServerConfig{},
		Origins: []Origin{
			{
				Name:              "s3",
				URL:               origin.URL,
				PathPatterns:      []string{"/s3/*"},
				RequireSignature:  boolPtr(false),
				DefaultRootObject: strPtr("index.html"),
			},
		},
		Signing: SigningConfig{Enabled: false},
	}

	router := SetupRouter(config, nil)
	req := httptest.NewRequest(http.MethodGet, "/s3/", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got=%d body=%s", resp.Code, resp.Body.String())
	}

	select {
	case got := <-requestedPath:
		want := "/s3/index.html"
		if got != want {
			t.Fatalf("unexpected proxied path: got=%s want=%s", got, want)
		}
	default:
		t.Fatal("origin did not receive a request")
	}
}

func TestContentTypeInferredFromExtensionWhenOriginReturnsOctetStream(t *testing.T) {
	t.Parallel()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "<html><body>ok</body></html>")
	}))
	defer origin.Close()

	config := &Config{
		Server: ServerConfig{},
		Origins: []Origin{
			{
				Name:             "s3",
				URL:              origin.URL,
				PathPatterns:     []string{"/s3/*"},
				RequireSignature: boolPtr(false),
			},
		},
		Signing: SigningConfig{Enabled: false},
	}

	router := SetupRouter(config, nil)
	req := httptest.NewRequest(http.MethodGet, "/s3/index.html", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got=%d body=%s", resp.Code, resp.Body.String())
	}

	if got := resp.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("unexpected content-type: got=%s want-prefix=text/html", got)
	}
}

func TestContentTypePreservedWhenOriginProvidesSpecificType(t *testing.T) {
	t.Parallel()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "plain")
	}))
	defer origin.Close()

	config := &Config{
		Server: ServerConfig{},
		Origins: []Origin{
			{
				Name:             "s3",
				URL:              origin.URL,
				PathPatterns:     []string{"/s3/*"},
				RequireSignature: boolPtr(false),
			},
		},
		Signing: SigningConfig{Enabled: false},
	}

	router := SetupRouter(config, nil)
	req := httptest.NewRequest(http.MethodGet, "/s3/index.html", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got=%d body=%s", resp.Code, resp.Body.String())
	}

	if got := resp.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("unexpected content-type: got=%s want=text/plain; charset=utf-8", got)
	}
}

func TestAdminSigningConfigUpdate(t *testing.T) {
	t.Parallel()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	defer origin.Close()

	config := &Config{
		Server: ServerConfig{},
		Origins: []Origin{
			{
				Name:             "s3",
				URL:              origin.URL,
				PathPatterns:     []string{"/*"},
				RequireSignature: boolPtr(false),
			},
		},
		Signing: SigningConfig{Enabled: false},
	}

	router := SetupRouter(config, nil)
	payload := map[string]any{
		"key_pair_id":     "KTEST123",
		"public_key_path": "keys/public_key.pem",
		"token_options": map[string]any{
			"clock_skew_seconds":         45,
			"default_url_ttl_seconds":    900,
			"default_cookie_ttl_seconds": 1200,
			"allow_wildcard_patterns":    true,
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/admin/api/signing/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got=%d body=%s", resp.Code, resp.Body.String())
	}

	if config.Signing.KeyPairID != "KTEST123" {
		t.Fatalf("unexpected key pair id: got=%s", config.Signing.KeyPairID)
	}
	if config.Signing.PublicKeyPath != "keys/public_key.pem" {
		t.Fatalf("unexpected public key path: got=%s", config.Signing.PublicKeyPath)
	}
	if config.Signing.TokenOptions.ClockSkewSeconds != 45 {
		t.Fatalf("unexpected clock skew: got=%d", config.Signing.TokenOptions.ClockSkewSeconds)
	}
	if config.Signing.TokenOptions.DefaultURLTTLSeconds != 900 {
		t.Fatalf("unexpected url ttl: got=%d", config.Signing.TokenOptions.DefaultURLTTLSeconds)
	}
	if config.Signing.TokenOptions.DefaultCookieTTLSeconds != 1200 {
		t.Fatalf("unexpected cookie ttl: got=%d", config.Signing.TokenOptions.DefaultCookieTTLSeconds)
	}
	if !config.Signing.TokenOptions.AllowWildcardPatterns {
		t.Fatal("expected allow_wildcard_patterns to be true")
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func strPtr(value string) *string {
	return &value
}
