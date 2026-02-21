// SPDX-License-Identifier: Apache-2.0

// Package health provides a standardized health check handler
// for Cloud-U-L8r service emulators.
package health

import (
	"encoding/json"
	"net/http"
)

// Response is the standard health check response format.
type Response struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

// Handler returns an http.HandlerFunc that responds with a JSON health check.
// The service parameter identifies the service in the response.
func Handler(service string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Response{
			Status:  "ok",
			Service: service,
		})
	}
}
