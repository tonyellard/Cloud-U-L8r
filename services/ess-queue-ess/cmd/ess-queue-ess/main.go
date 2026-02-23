// SPDX-License-Identifier: Apache-2.0

package main

import (
	"log"
	"net/http"
	"os"

	"github.com/tonyellard/ess-queue-ess/internal/server"
)

func main() {
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
