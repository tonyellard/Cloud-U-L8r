// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/tonyellard/cloudfauxnt/internal/server"
)

func main() {
	var config *server.Config
	var err error

	// Use env vars for configuration
	if configPath := os.Getenv("CONFIG_PATH"); configPath != "" {
		log.Printf("Loading configuration from %s", configPath)
		config, err = server.LoadConfig(configPath)
	} else {
		log.Println("Building configuration from environment variables")
		config, err = server.BuildConfigFromEnv()
	}
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("CloudFauxnt starting with %d origin(s)", len(config.Origins))
	for _, origin := range config.Origins {
		log.Printf("  - %s: %s (patterns: %v)", origin.Name, origin.URL, origin.PathPatterns)
	}

	// Initialize signature validator if signing is enabled
	var validator *server.SignatureValidator
	if config.Signing.Enabled {
		clockSkew := config.Signing.TokenOptions.ClockSkewSeconds
		if clockSkew == 0 {
			clockSkew = 30 // Default 30 seconds clock skew
		}
		validator = server.NewSignatureValidator(config.Signing.PublicKey, config.Signing.KeyPairID, clockSkew)
		log.Printf("CloudFront signature validation enabled (Key Pair ID: %s, Clock Skew: %d seconds)",
			config.Signing.KeyPairID, clockSkew)
	} else {
		log.Println("CloudFront signature validation disabled")
	}

	// Setup router
	router := server.SetupRouter(config, validator)

	// Configure HTTP server
	addr := fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  time.Duration(config.Server.TimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(config.Server.TimeoutSeconds) * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server
	log.Printf("CloudFauxnt listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
