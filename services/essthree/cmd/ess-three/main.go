// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/tony/ess-three/internal/server"
	"github.com/tony/ess-three/internal/storage"
)

func main() {
	port := flag.String("port", "9300", "Port to run the server on")
	dataDir := flag.String("data-dir", "/data", "Directory to store bucket data")
	flag.Parse()

	// Create storage backend
	store, err := storage.NewFileSystemStorage(*dataDir)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create and configure server
	srv := server.NewServer(store)

	addr := fmt.Sprintf(":%s", *port)
	log.Printf("Starting ess-three S3 emulator on %s", addr)
	log.Printf("Data directory: %s", *dataDir)

	if err := http.ListenAndServe(addr, srv.Router()); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
