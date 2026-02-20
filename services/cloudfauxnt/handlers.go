// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tonyellard/cloud-u-l8r/pkg/awserrors"
	"github.com/tonyellard/cloud-u-l8r/pkg/health"
)

// ProxyHandler handles incoming requests and proxies them to origins
type ProxyHandler struct {
	config    *Config
	validator *SignatureValidator
}

// NewProxyHandler creates a new proxy handler
func NewProxyHandler(config *Config, validator *SignatureValidator) *ProxyHandler {
	return &ProxyHandler{
		config:    config,
		validator: validator,
	}
}

// ServeHTTP handles the proxy request
func (ph *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Find matching origin first to determine signature requirement and default root object
	origin, err := ph.config.FindOrigin(r.URL.Path)
	if err != nil {
		awserrors.WriteCloudFrontXML(w, "NoSuchKey", "The specified path does not match any configured origin", generateCloudFrontID(), http.StatusNotFound)
		return
	}

	// Determine if signature is required for this origin
	requireSignature := ph.config.Signing.Enabled // Default to global setting
	if origin.RequireSignature != nil {
		// Per-origin setting overrides global setting
		requireSignature = *origin.RequireSignature
	}

	// Validate signature if required
	if requireSignature {
		if err := ph.validator.ValidateRequest(r); err != nil {
			awserrors.WriteCloudFrontXML(w, "AccessDenied", err.Error(), generateCloudFrontID(), http.StatusForbidden)
			return
		}
	}

	// Proxy to origin
	if err := ph.proxyToOrigin(w, r, origin); err != nil {
		awserrors.WriteCloudFrontXML(w, "ServiceUnavailable", err.Error(), generateCloudFrontID(), http.StatusServiceUnavailable)
		return
	}
}

// proxyToOrigin forwards the request to the origin server
func (ph *ProxyHandler) proxyToOrigin(w http.ResponseWriter, r *http.Request, origin *Origin) error {
	// Parse origin URL
	originURL, err := url.Parse(origin.URL)
	if err != nil {
		return fmt.Errorf("invalid origin URL: %w", err)
	}

	// Create reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(originURL)

	// Customize the director to modify the request
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// Remove CloudFront signature parameters
		req.URL = RemoveSignatureParams(req.URL)

		// Apply path rewriting if configured
		if origin.StripPrefix != "" {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, origin.StripPrefix)
		}

		// Apply default root object before adding target prefix.
		// For GET/HEAD, normalize both directory-style paths ending with "/"
		// and directory-style paths without a trailing slash.
		if req.Method == http.MethodGet || req.Method == http.MethodHead {
			if defaultRootObject := ph.defaultRootObjectForOrigin(origin); defaultRootObject != "" {
				req.URL.Path = applyDefaultRootObject(req.URL.Path, defaultRootObject)
			}
		}

		if origin.TargetPrefix != "" {
			req.URL.Path = origin.TargetPrefix + req.URL.Path
		}

		// Set proper Host header
		req.Host = originURL.Host
		req.Header.Set("Host", originURL.Host)

		// Add CloudFront headers
		req.Header.Set("X-Amz-Cf-Id", generateCloudFrontID())
		req.Header.Set("Via", "1.1 cloudfauxnt")

		// Preserve original headers
		if userAgent := r.Header.Get("User-Agent"); userAgent != "" {
			req.Header.Set("User-Agent", userAgent)
		}
	}

	// Customize response modifier to add CloudFront headers
	proxy.ModifyResponse = func(resp *http.Response) error {
		normalizeResponseContentType(resp)
		resp.Header.Set("X-Cache", "Miss from cloudfauxnt")
		resp.Header.Set("X-Amz-Cf-Id", generateCloudFrontID())
		resp.Header.Set("Via", "1.1 cloudfauxnt")
		resp.Header.Set("Server", "CloudFauxnt")
		resp.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
		return nil
	}

	// Handle errors
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		awserrors.WriteCloudFrontXML(w, "BadGateway", fmt.Sprintf("Failed to reach origin: %v", err), generateCloudFrontID(), http.StatusBadGateway)
	}

	// Serve the proxy request
	proxy.ServeHTTP(w, r)
	return nil
}

func (ph *ProxyHandler) defaultRootObjectForOrigin(origin *Origin) string {
	if origin.DefaultRootObject != nil && *origin.DefaultRootObject != "" {
		return *origin.DefaultRootObject
	}

	return ph.config.Server.DefaultRootObject
}

func applyDefaultRootObject(path, defaultRootObject string) string {
	if defaultRootObject == "" {
		return path
	}

	if path == "" {
		return "/" + defaultRootObject
	}

	if path == "/" {
		return "/" + defaultRootObject
	}

	if strings.HasSuffix(path, "/") {
		return path + defaultRootObject
	}

	lastSlash := strings.LastIndex(path, "/")
	lastSegment := path
	if lastSlash >= 0 {
		lastSegment = path[lastSlash+1:]
	}

	if lastSegment != "" && !strings.Contains(lastSegment, ".") {
		return path + "/" + defaultRootObject
	}

	return path
}

func normalizeResponseContentType(resp *http.Response) {
	if resp == nil || resp.Request == nil || resp.Request.URL == nil {
		return
	}

	current := strings.TrimSpace(resp.Header.Get("Content-Type"))
	mediaType := strings.ToLower(strings.Split(current, ";")[0])
	if mediaType != "" && mediaType != "application/octet-stream" {
		return
	}

	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Disposition"))), "attachment") {
		return
	}

	ext := strings.ToLower(path.Ext(resp.Request.URL.Path))
	if ext == "" {
		return
	}

	if guessed := mime.TypeByExtension(ext); guessed != "" {
		resp.Header.Set("Content-Type", guessed)
	}
}

// generateCloudFrontID generates a unique CloudFront request ID
func generateCloudFrontID() string {
	id := uuid.New().String()
	return strings.ToUpper(strings.ReplaceAll(id, "-", ""))
}

// SetupRouter configures the Chi router with all routes
func SetupRouter(config *Config, validator *SignatureValidator) chi.Router {
	r := chi.NewRouter()

	// Add CORS middleware if enabled
	if config.CORS.Enabled {
		corsMiddleware := NewCORSMiddleware(config.CORS)
		r.Use(corsMiddleware.Handler)
	}

	// Health check endpoint
	r.Get("/health", health.Handler("cloudfauxnt"))
	r.Get("/admin/api/overview", func(w http.ResponseWriter, r *http.Request) {
		type originOverview struct {
			Name              string   `json:"name"`
			URL               string   `json:"url"`
			PathPatterns      []string `json:"path_patterns"`
			StripPrefix       string   `json:"strip_prefix,omitempty"`
			TargetPrefix      string   `json:"target_prefix,omitempty"`
			RequireSignature  bool     `json:"require_signature"`
			DefaultRootObject string   `json:"default_root_object,omitempty"`
		}

		type response struct {
			Service string `json:"service"`
			Server  struct {
				Host              string `json:"host"`
				Port              int    `json:"port"`
				DefaultRootObject string `json:"default_root_object"`
			} `json:"server"`
			Signing struct {
				Enabled   bool   `json:"enabled"`
				KeyPairID string `json:"key_pair_id,omitempty"`
			} `json:"signing"`
			Stats struct {
				Origins   int `json:"origins"`
				Behaviors int `json:"behaviors"`
			} `json:"stats"`
			Origins []originOverview `json:"origins"`
		}

		result := response{Service: "cloudfauxnt"}
		result.Server.Host = config.Server.Host
		result.Server.Port = config.Server.Port
		result.Server.DefaultRootObject = config.Server.DefaultRootObject
		result.Signing.Enabled = config.Signing.Enabled
		result.Signing.KeyPairID = config.Signing.KeyPairID
		result.Stats.Origins = len(config.Origins)

		origins := make([]originOverview, 0, len(config.Origins))
		for _, origin := range config.Origins {
			requireSignature := config.Signing.Enabled
			if origin.RequireSignature != nil {
				requireSignature = *origin.RequireSignature
			}
			defaultRootObject := config.Server.DefaultRootObject
			if origin.DefaultRootObject != nil {
				defaultRootObject = *origin.DefaultRootObject
			}

			origins = append(origins, originOverview{
				Name:              origin.Name,
				URL:               origin.URL,
				PathPatterns:      origin.PathPatterns,
				StripPrefix:       origin.StripPrefix,
				TargetPrefix:      origin.TargetPrefix,
				RequireSignature:  requireSignature,
				DefaultRootObject: defaultRootObject,
			})
			result.Stats.Behaviors += len(origin.PathPatterns)
		}
		result.Origins = origins

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})

	// Main proxy handler (catch-all)
	proxyHandler := NewProxyHandler(config, validator)
	r.NotFound(proxyHandler.ServeHTTP)

	return r
}
