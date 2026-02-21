// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/tonyellard/ess-queue-ess/internal/server"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "", "Path to configuration file")
	flag.Parse()

	// Load configuration if provided
	if *configPath != "" {
		config, err := server.LoadConfig(*configPath)
		if err != nil {
			log.Printf("Warning: Failed to load config: %v", err)
		} else {
			log.Printf("Loaded configuration from %s", *configPath)
			if err := server.BootstrapQueues(config); err != nil {
				log.Fatalf("Failed to bootstrap queues: %v", err)
			}
			log.Printf("Bootstrapped %d queues from configuration", len(config.Queues))

			// Use port from config if not overridden by environment
			if os.Getenv("PORT") == "" && config.Server.Port > 0 {
				os.Setenv("PORT", strconv.Itoa(config.Server.Port))
			}
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "9320" // Default SQS port for local development
	}

	// Setup router
	router := server.SetupRouter()

	log.Printf("Starting Ess-Queue-Ess on port %s", port)
	log.Printf("SQS endpoint: http://localhost:%s/", port)
	log.Printf("Admin UI: http://localhost:%s/admin", port)

	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
