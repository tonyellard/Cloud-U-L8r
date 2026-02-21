// SPDX-License-Identifier: Apache-2.0

package server

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/tonyellard/cloud-u-l8r/pkg/activity"
	"github.com/tonyellard/cloud-u-l8r/pkg/awserrors"
	"github.com/tonyellard/cloud-u-l8r/pkg/health"
	"github.com/tonyellard/essthree/internal/storage"
)

// Server represents the S3 API server
type Server struct {
	storage  storage.Storage
	activity *activity.Logger
}

// NewServer creates a new S3 API server
func NewServer(storage storage.Storage) *Server {
	return &Server{
		storage: storage,
		activity: activity.NewLogger(activity.WithExcludeFunc(func(e activity.Entry) bool {
			return strings.HasPrefix(e.Path, "/admin/") || e.Path == "/health"
		})),
	}
}

// Router creates and configures the HTTP router
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(s.activityMiddleware)

	// Health check
	r.Get("/health", health.Handler("essthree"))

	// Admin API endpoints
	r.Get("/admin/api/buckets", s.handleAdminBuckets)
	r.Get("/admin/api/buckets/{bucket}/objects", s.handleAdminListObjects)
	r.Post("/admin/api/buckets", s.handleAdminCreateBucket)
	r.Delete("/admin/api/buckets/{bucket}", s.handleAdminDeleteBucket)
	r.Delete("/admin/api/buckets/{bucket}/objects", s.handleAdminDeleteObject)
	r.Post("/admin/api/buckets/{bucket}/purge", s.handleAdminPurgeBucket)
	r.Get("/admin/api/activity", s.handleAdminActivity)

	// S3 API routes
	// Bucket operations
	r.Route("/{bucket}", func(r chi.Router) {
		// Bucket-level GET: dispatch on query params for bucket properties
		r.Get("/", s.handleBucketGet)

		// CreateBucket
		r.Put("/", s.handleCreateBucket)

		// HeadBucket
		r.Head("/", s.handleHeadBucket)

		// DeleteBucket
		r.Delete("/", s.handleDeleteBucket)

		// Batch delete
		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			if _, ok := r.URL.Query()["delete"]; ok {
				s.handleBatchDelete(w, r)
			} else {
				http.Error(w, "Not Found", http.StatusNotFound)
			}
		})

		// Object operations
		r.Head("/*", s.handleHeadObject)

		r.Get("/*", s.handleGetObjectOrListPrefix)

		r.Put("/*", func(w http.ResponseWriter, req *http.Request) {
			// Check if this is a multipart operation
			_, hasPartNumber := req.URL.Query()["partNumber"]
			_, hasUploadId := req.URL.Query()["uploadId"]
			if hasPartNumber && hasUploadId {
				s.handleUploadPart(w, req)
			} else {
				s.handlePutObject(w, req)
			}
		})

		r.Post("/*", func(w http.ResponseWriter, req *http.Request) {
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

		r.Delete("/*", func(w http.ResponseWriter, req *http.Request) {
			if _, ok := req.URL.Query()["uploadId"]; ok {
				s.handleAbortMultipartUpload(w, req)
			} else {
				s.handleDeleteObject(w, req)
			}
		})
	})

	return r
}

// activityMiddleware records each S3 API request in the activity log.
func (s *Server) activityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		action := describeS3Action(r)
		entry := activity.Entry{
			Method:     r.Method,
			Path:       r.URL.Path,
			Action:     action,
			StatusCode: rec.status,
		}

		if rec.status >= 400 {
			if errCode, errMsg := parseXMLError(rec.errBody.Bytes()); errCode != "" {
				entry.ErrorType = errCode
				entry.Detail = errMsg
			}
		}

		s.activity.Record(entry)
	})
}

// statusRecorder captures the HTTP status code and, for error responses,
// the response body so the middleware can extract error details.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	errBody     bytes.Buffer
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.wroteHeader = true
	}
	if r.status >= 400 {
		r.errBody.Write(b)
	}
	return r.ResponseWriter.Write(b)
}

// parseXMLError extracts the Code and Message from an S3-style XML error body.
func parseXMLError(body []byte) (string, string) {
	if len(body) == 0 {
		return "", ""
	}
	var xmlErr awserrors.XMLError
	if err := xml.Unmarshal(body, &xmlErr); err != nil {
		return "", ""
	}
	return xmlErr.Code, xmlErr.Message
}

// describeS3Action returns a human-readable label for S3 API operations.
func describeS3Action(r *http.Request) string {
	path := r.URL.Path
	parts := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 2)
	bucket := ""
	key := ""
	if len(parts) >= 1 {
		bucket = parts[0]
	}
	if len(parts) >= 2 {
		key = parts[1]
	}

	switch r.Method {
	case http.MethodPut:
		if key != "" {
			if _, ok := r.URL.Query()["partNumber"]; ok {
				return "UploadPart"
			}
			return "PutObject"
		}
		return "CreateBucket"
	case http.MethodGet:
		if key != "" {
			return "GetObject"
		}
		if bucket != "" {
			return "ListObjects"
		}
		return "ListBuckets"
	case http.MethodHead:
		if key != "" {
			return "HeadObject"
		}
		return "HeadBucket"
	case http.MethodDelete:
		if key != "" {
			if _, ok := r.URL.Query()["uploadId"]; ok {
				return "AbortMultipartUpload"
			}
			return "DeleteObject"
		}
		return "DeleteBucket"
	case http.MethodPost:
		if _, ok := r.URL.Query()["delete"]; ok {
			return "DeleteObjects"
		}
		if _, ok := r.URL.Query()["uploads"]; ok {
			return "CreateMultipartUpload"
		}
		if _, ok := r.URL.Query()["uploadId"]; ok {
			return "CompleteMultipartUpload"
		}
	}
	return ""
}

func (s *Server) handleAdminBuckets(w http.ResponseWriter, r *http.Request) {
	buckets, err := s.storage.ListBuckets()
	if err != nil {
		http.Error(w, "failed to list buckets", http.StatusInternalServerError)
		return
	}

	type response struct {
		Buckets []storage.BucketSummary `json:"buckets"`
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response{Buckets: buckets})
}

func (s *Server) handleAdminListObjects(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	prefix := r.URL.Query().Get("prefix")
	maxKeysStr := r.URL.Query().Get("maxKeys")
	continuationToken := r.URL.Query().Get("continuationToken")

	maxKeys := 100
	if maxKeysStr != "" {
		if v, err := strconv.Atoi(maxKeysStr); err == nil && v > 0 {
			maxKeys = v
		}
	}

	if !s.storage.BucketExists(bucket) {
		writeAdminError(w, http.StatusNotFound, fmt.Sprintf("bucket %q does not exist", bucket))
		return
	}

	result, err := s.storage.ListObjectsV2(bucket, prefix, continuationToken, maxKeys)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type objectEntry struct {
		Key          string `json:"key"`
		Size         int64  `json:"size"`
		LastModified string `json:"lastModified"`
		ETag         string `json:"etag"`
		ContentType  string `json:"contentType"`
	}

	objects := make([]objectEntry, 0, len(result.Objects))
	for _, obj := range result.Objects {
		objects = append(objects, objectEntry{
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified.Format("2006-01-02T15:04:05Z"),
			ETag:         obj.ETag,
			ContentType:  obj.ContentType,
		})
	}

	writeAdminJSON(w, http.StatusOK, map[string]any{
		"bucket":                bucket,
		"prefix":                prefix,
		"objects":               objects,
		"isTruncated":           result.IsTruncated,
		"nextContinuationToken": result.NextContinuationToken,
	})
}

func (s *Server) handleAdminCreateBucket(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeAdminError(w, http.StatusBadRequest, "name is required")
		return
	}

	if err := s.storage.CreateBucket(req.Name); err != nil {
		writeAdminError(w, http.StatusConflict, err.Error())
		return
	}

	writeAdminJSON(w, http.StatusCreated, map[string]string{"name": req.Name})
}

func (s *Server) handleAdminDeleteBucket(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	if err := s.storage.DeleteBucket(bucket); err != nil {
		if strings.Contains(err.Error(), "not empty") {
			writeAdminError(w, http.StatusConflict, err.Error())
		} else if strings.Contains(err.Error(), "does not exist") {
			writeAdminError(w, http.StatusNotFound, err.Error())
		} else {
			writeAdminError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminDeleteObject(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	key := r.URL.Query().Get("key")
	if key == "" {
		writeAdminError(w, http.StatusBadRequest, "key query parameter is required")
		return
	}

	if !s.storage.BucketExists(bucket) {
		writeAdminError(w, http.StatusNotFound, fmt.Sprintf("bucket %q does not exist", bucket))
		return
	}

	if err := s.storage.DeleteObject(bucket, key); err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminPurgeBucket(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")

	if !s.storage.BucketExists(bucket) {
		writeAdminError(w, http.StatusNotFound, fmt.Sprintf("bucket %q does not exist", bucket))
		return
	}

	// List all objects (unpaginated) and delete them.
	result, err := s.storage.ListObjectsV2(bucket, "", "", 0)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}

	keys := make([]string, 0, len(result.Objects))
	for _, obj := range result.Objects {
		keys = append(keys, obj.Key)
	}

	if len(keys) > 0 {
		s.storage.DeleteObjects(bucket, keys)
	}

	writeAdminJSON(w, http.StatusOK, map[string]any{
		"bucket":  bucket,
		"deleted": len(keys),
	})
}

func (s *Server) handleAdminActivity(w http.ResponseWriter, r *http.Request) {
	maxResultsStr := r.URL.Query().Get("maxResults")
	nextToken := r.URL.Query().Get("nextToken")

	maxResults := 50
	if maxResultsStr != "" {
		if v, err := strconv.Atoi(maxResultsStr); err == nil && v > 0 {
			maxResults = v
		}
	}

	entries, token, err := s.activity.List(maxResults, nextToken)
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeAdminJSON(w, http.StatusOK, map[string]any{
		"activity":  entries,
		"nextToken": token,
	})
}

func writeAdminJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeAdminError(w http.ResponseWriter, status int, message string) {
	writeAdminJSON(w, status, map[string]string{"error": message})
}
