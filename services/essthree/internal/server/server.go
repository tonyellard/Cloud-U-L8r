// SPDX-License-Identifier: Apache-2.0

package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/tony/ess-three/internal/storage"
)

// Server represents the S3 API server
type Server struct {
	storage storage.Storage
}

// NewServer creates a new S3 API server
func NewServer(storage storage.Storage) *Server {
	return &Server{
		storage: storage,
	}
}

// Router creates and configures the HTTP router
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	// Health check
	r.Get("/health", s.handleHealth)

	// S3 API routes
	// Bucket operations
	r.Route("/{bucket}", func(r chi.Router) {
		// List objects (supports both V1 and V2)
		r.Get("/", s.handleListObjects)

		// Batch delete
		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			if _, ok := r.URL.Query()["delete"]; ok {
				s.handleBatchDelete(w, r)
			} else {
				http.Error(w, "Not Found", http.StatusNotFound)
			}
		})

		// Object operations
		r.Route("/{key:.*}", func(r chi.Router) {
			r.Head("/", s.handleHeadObject)

			r.Get("/", s.handleGetObject)

			r.Put("/", func(w http.ResponseWriter, req *http.Request) {
				// Check if this is a multipart operation
				_, hasPartNumber := req.URL.Query()["partNumber"]
				_, hasUploadId := req.URL.Query()["uploadId"]
				if hasPartNumber && hasUploadId {
					s.handleUploadPart(w, req)
				} else {
					s.handlePutObject(w, req)
				}
			})

			r.Post("/", func(w http.ResponseWriter, req *http.Request) {
				// Check what type of POST this is
				_, hasUploads := req.URL.Query()["uploads"]
				_, hasUploadId := req.URL.Query()["uploadId"]

				if hasUploads {
					s.handleCreateMultipartUpload(w, req)
				} else if hasUploadId {
					s.handleCompleteMultipartUpload(w, req)
				} else {
					http.Error(w, "Not Found", http.StatusNotFound)
				}
			})

			r.Delete("/", func(w http.ResponseWriter, req *http.Request) {
				if _, ok := req.URL.Query()["uploadId"]; ok {
					s.handleAbortMultipartUpload(w, req)
				} else {
					s.handleDeleteObject(w, req)
				}
			})
		})
	})

	return r
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
