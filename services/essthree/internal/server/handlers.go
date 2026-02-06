// SPDX-License-Identifier: Apache-2.0

package server

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tony/ess-three/internal/storage"
)

// S3 XML response structures

type ListBucketResult struct {
	XMLName               xml.Name   `xml:"ListBucketResult"`
	Xmlns                 string     `xml:"xmlns,attr"`
	Name                  string     `xml:"Name"`
	Prefix                string     `xml:"Prefix"`
	Marker                string     `xml:"Marker,omitempty"`
	NextMarker            string     `xml:"NextMarker,omitempty"`
	MaxKeys               int        `xml:"MaxKeys"`
	IsTruncated           bool       `xml:"IsTruncated"`
	Contents              []Contents `xml:"Contents"`
	KeyCount              int        `xml:"KeyCount,omitempty"`
	ContinuationToken     string     `xml:"ContinuationToken,omitempty"`
	NextContinuationToken string     `xml:"NextContinuationToken,omitempty"`
}

type Contents struct {
	Key          string    `xml:"Key"`
	LastModified time.Time `xml:"LastModified"`
	ETag         string    `xml:"ETag"`
	Size         int64     `xml:"Size"`
	StorageClass string    `xml:"StorageClass"`
}

type Error struct {
	XMLName   xml.Name `xml:"Error"`
	Code      string   `xml:"Code"`
	Message   string   `xml:"Message"`
	Resource  string   `xml:"Resource"`
	RequestId string   `xml:"RequestId"`
}

type InitiateMultipartUploadResult struct {
	XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
	Xmlns    string   `xml:"xmlns,attr"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	UploadId string   `xml:"UploadId"`
}

type CompleteMultipartUploadRequest struct {
	XMLName xml.Name       `xml:"CompleteMultipartUpload"`
	Parts   []CompletePart `xml:"Part"`
}

type CompletePart struct {
	PartNumber int    `xml:"PartNumber"`
	ETag       string `xml:"ETag"`
}

type CompleteMultipartUploadResult struct {
	XMLName  xml.Name `xml:"CompleteMultipartUploadResult"`
	Xmlns    string   `xml:"xmlns,attr"`
	Location string   `xml:"Location"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	ETag     string   `xml:"ETag"`
}

type DeleteRequest struct {
	XMLName xml.Name       `xml:"Delete"`
	Objects []DeleteObject `xml:"Object"`
	Quiet   bool           `xml:"Quiet"`
}

type DeleteObject struct {
	Key string `xml:"Key"`
}

type DeleteResult struct {
	XMLName xml.Name        `xml:"DeleteResult"`
	Xmlns   string          `xml:"xmlns,attr"`
	Deleted []DeletedObject `xml:"Deleted"`
	Errors  []DeleteError   `xml:"Error,omitempty"`
}

type DeletedObject struct {
	Key string `xml:"Key"`
}

type DeleteError struct {
	Key     string `xml:"Key"`
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}

// handleListObjects handles GET /{bucket} - ListObjects V1 and V2
func (s *Server) handleListObjects(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	prefix := r.URL.Query().Get("prefix")
	maxKeysStr := r.URL.Query().Get("max-keys")

	// Check if this is V2 or V1
	listType := r.URL.Query().Get("list-type")

	maxKeys := 1000
	if maxKeysStr != "" {
		if mk, err := strconv.Atoi(maxKeysStr); err == nil {
			maxKeys = mk
		}
	}

	var result *storage.ListResult
	var err error

	if listType == "2" {
		// ListObjectsV2
		continuationToken := r.URL.Query().Get("continuation-token")
		result, err = s.storage.ListObjectsV2(bucket, prefix, continuationToken, maxKeys)
	} else {
		// ListObjectsV1
		marker := r.URL.Query().Get("marker")
		result, err = s.storage.ListObjects(bucket, prefix, marker, maxKeys)
	}

	if err != nil {
		s.sendError(w, r, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	contents := make([]Contents, len(result.Objects))
	for i, obj := range result.Objects {
		contents[i] = Contents{
			Key:          obj.Key,
			LastModified: obj.LastModified,
			ETag:         obj.ETag,
			Size:         obj.Size,
			StorageClass: "STANDARD",
		}
	}

	response := ListBucketResult{
		Xmlns:       "http://s3.amazonaws.com/doc/2006-03-01/",
		Name:        bucket,
		Prefix:      prefix,
		MaxKeys:     maxKeys,
		IsTruncated: result.IsTruncated,
		Contents:    contents,
	}

	if listType == "2" {
		// V2 specific fields
		response.KeyCount = len(contents)
		if result.IsTruncated {
			response.NextContinuationToken = result.NextContinuationToken
		}
		if r.URL.Query().Get("continuation-token") != "" {
			response.ContinuationToken = r.URL.Query().Get("continuation-token")
		}
	} else {
		// V1 specific fields
		if result.IsTruncated {
			response.NextMarker = result.NextMarker
		}
		if r.URL.Query().Get("marker") != "" {
			response.Marker = r.URL.Query().Get("marker")
		}
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	xml.NewEncoder(w).Encode(response)
}

// handleGetObject handles GET /{bucket}/{key} - GetObject with range support
func (s *Server) handleGetObject(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "key")

	// Check for Range header
	rangeHeader := r.Header.Get("Range")

	if rangeHeader != "" {
		// Parse range header
		rangeStart, rangeEnd, err := parseRangeHeader(rangeHeader)
		if err != nil {
			s.sendError(w, r, "InvalidRange", err.Error(), http.StatusRequestedRangeNotSatisfiable)
			return
		}

		reader, metadata, start, end, err := s.storage.GetObjectRange(bucket, key, rangeStart, rangeEnd)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				s.sendError(w, r, "NoSuchKey", "The specified key does not exist", http.StatusNotFound)
			} else {
				s.sendError(w, r, "InternalError", err.Error(), http.StatusInternalServerError)
			}
			return
		}
		defer reader.Close()

		// Set headers for partial content
		w.Header().Set("Content-Type", metadata.ContentType)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, metadata.Size))
		w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
		w.Header().Set("ETag", metadata.ETag)
		w.Header().Set("Last-Modified", metadata.LastModified.Format(http.TimeFormat))
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Server", "ess-three")
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))

		// Set custom metadata headers
		for k, v := range metadata.Metadata {
			w.Header().Set("x-amz-meta-"+k, v)
		}

		w.WriteHeader(http.StatusPartialContent)
		// Use io.CopyN to respect Content-Length and prevent chunked encoding
		io.CopyN(w, reader, end-start+1)
	} else {
		// Normal GET (full object)
		reader, metadata, err := s.storage.GetObject(bucket, key)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				s.sendError(w, r, "NoSuchKey", "The specified key does not exist", http.StatusNotFound)
			} else {
				s.sendError(w, r, "InternalError", err.Error(), http.StatusInternalServerError)
			}
			return
		}
		defer reader.Close()

		// Set headers
		w.Header().Set("Content-Type", metadata.ContentType)
		w.Header().Set("Content-Length", strconv.FormatInt(metadata.Size, 10))
		w.Header().Set("ETag", metadata.ETag)
		w.Header().Set("Last-Modified", metadata.LastModified.Format(http.TimeFormat))
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Server", "ess-three")
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		w.Header().Set("Connection", "keep-alive")

		// Set custom metadata headers
		for k, v := range metadata.Metadata {
			w.Header().Set("x-amz-meta-"+k, v)
		}

		w.WriteHeader(http.StatusOK)
		// Use io.CopyN to respect Content-Length and prevent chunked encoding
		io.CopyN(w, reader, metadata.Size)
	}
}

// parseRangeHeader parses HTTP Range header
func parseRangeHeader(rangeHeader string) (int64, int64, error) {
	// Format: "bytes=start-end"
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return 0, 0, fmt.Errorf("invalid range header format")
	}

	rangeSpec := strings.TrimPrefix(rangeHeader, "bytes=")
	parts := strings.Split(rangeSpec, "-")

	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid range format")
	}

	var start, end int64
	var err error

	if parts[0] != "" {
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return 0, 0, err
		}
	}

	if parts[1] != "" {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return 0, 0, err
		}
	} else {
		end = -1 // To end of file
	}

	return start, end, nil
}

// handlePutObject handles PUT /{bucket}/{key} - PutObject
func (s *Server) handlePutObject(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "key")

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Extract custom metadata from headers
	metadata := make(map[string]string)
	for headerKey, values := range r.Header {
		if strings.HasPrefix(strings.ToLower(headerKey), "x-amz-meta-") {
			metaKey := strings.TrimPrefix(strings.ToLower(headerKey), "x-amz-meta-")
			if len(values) > 0 {
				metadata[metaKey] = values[0]
			}
		}
	}

	objMetadata, err := s.storage.PutObject(bucket, key, r.Body, metadata, contentType)
	if err != nil {
		s.sendError(w, r, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	// Set response headers for S3 compatibility
	w.Header().Set("ETag", objMetadata.ETag)
	w.Header().Set("x-amz-version-id", "null")
	w.Header().Set("Server", "ess-three")
	w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
	w.Header().Set("Content-Length", "0")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
}

// handleHeadObject handles HEAD /{bucket}/{key} - HeadObject
func (s *Server) handleHeadObject(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "key")

	metadata, err := s.storage.HeadObject(bucket, key)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	// Set headers
	w.Header().Set("Content-Type", metadata.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(metadata.Size, 10))
	w.Header().Set("ETag", metadata.ETag)
	w.Header().Set("Last-Modified", metadata.LastModified.Format(http.TimeFormat))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Server", "ess-three")
	w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
	w.Header().Set("x-amz-version-id", "null")

	// Set custom metadata headers
	for k, v := range metadata.Metadata {
		w.Header().Set("x-amz-meta-"+k, v)
	}

	w.WriteHeader(http.StatusOK)
}

// handleDeleteObject handles DELETE /{bucket}/{key} - DeleteObject
func (s *Server) handleDeleteObject(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "key")

	err := s.storage.DeleteObject(bucket, key)
	if err != nil {
		s.sendError(w, r, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleBatchDelete handles POST /{bucket}?delete - DeleteObjects
func (s *Server) handleBatchDelete(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")

	// Parse delete request
	var deleteReq DeleteRequest
	if err := xml.NewDecoder(r.Body).Decode(&deleteReq); err != nil {
		s.sendError(w, r, "MalformedXML", "Invalid XML", http.StatusBadRequest)
		return
	}

	// Extract keys to delete
	keys := make([]string, len(deleteReq.Objects))
	for i, obj := range deleteReq.Objects {
		keys[i] = obj.Key
	}

	// Delete objects
	deleted, errors := s.storage.DeleteObjects(bucket, keys)

	// Build response
	result := DeleteResult{
		Xmlns: "http://s3.amazonaws.com/doc/2006-03-01/",
	}

	for _, key := range deleted {
		result.Deleted = append(result.Deleted, DeletedObject{Key: key})
	}

	for i, err := range errors {
		if err != nil {
			result.Errors = append(result.Errors, DeleteError{
				Key:     keys[i],
				Code:    "InternalError",
				Message: err.Error(),
			})
		}
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	xml.NewEncoder(w).Encode(result)
}

// handleCreateMultipartUpload handles POST /{bucket}/{key}?uploads
func (s *Server) handleCreateMultipartUpload(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "key")

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Extract metadata
	metadata := make(map[string]string)
	for headerKey, values := range r.Header {
		if strings.HasPrefix(strings.ToLower(headerKey), "x-amz-meta-") {
			metaKey := strings.TrimPrefix(strings.ToLower(headerKey), "x-amz-meta-")
			if len(values) > 0 {
				metadata[metaKey] = values[0]
			}
		}
	}

	upload, err := s.storage.CreateMultipartUpload(bucket, key, contentType, metadata)
	if err != nil {
		s.sendError(w, r, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	result := InitiateMultipartUploadResult{
		Xmlns:    "http://s3.amazonaws.com/doc/2006-03-01/",
		Bucket:   bucket,
		Key:      key,
		UploadId: upload.UploadID,
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	xml.NewEncoder(w).Encode(result)
}

// handleUploadPart handles PUT /{bucket}/{key}?partNumber=X&uploadId=Y
func (s *Server) handleUploadPart(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "key")
	uploadID := r.URL.Query().Get("uploadId")
	partNumberStr := r.URL.Query().Get("partNumber")

	partNumber, err := strconv.Atoi(partNumberStr)
	if err != nil {
		s.sendError(w, r, "InvalidArgument", "Invalid part number", http.StatusBadRequest)
		return
	}

	part, err := s.storage.UploadPart(bucket, key, uploadID, partNumber, r.Body)
	if err != nil {
		s.sendError(w, r, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("ETag", fmt.Sprintf("\"%s\"", part.ETag))
	w.WriteHeader(http.StatusOK)
}

// handleCompleteMultipartUpload handles POST /{bucket}/{key}?uploadId=X
func (s *Server) handleCompleteMultipartUpload(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "key")
	uploadID := r.URL.Query().Get("uploadId")

	// Parse complete request
	var completeReq CompleteMultipartUploadRequest
	if err := xml.NewDecoder(r.Body).Decode(&completeReq); err != nil {
		s.sendError(w, r, "MalformedXML", "Invalid XML", http.StatusBadRequest)
		return
	}

	// Convert to storage parts
	parts := make([]storage.Part, len(completeReq.Parts))
	for i, p := range completeReq.Parts {
		parts[i] = storage.Part{
			PartNumber: p.PartNumber,
			ETag:       strings.Trim(p.ETag, "\""),
		}
	}

	// Complete the upload
	objMeta, err := s.storage.CompleteMultipartUpload(bucket, key, uploadID, parts)
	if err != nil {
		s.sendError(w, r, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	result := CompleteMultipartUploadResult{
		Xmlns:    "http://s3.amazonaws.com/doc/2006-03-01/",
		Location: fmt.Sprintf("/%s/%s", bucket, key),
		Bucket:   bucket,
		Key:      key,
		ETag:     objMeta.ETag,
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	xml.NewEncoder(w).Encode(result)
}

// handleAbortMultipartUpload handles DELETE /{bucket}/{key}?uploadId=X
func (s *Server) handleAbortMultipartUpload(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "key")
	uploadID := r.URL.Query().Get("uploadId")

	err := s.storage.AbortMultipartUpload(bucket, key, uploadID)
	if err != nil {
		s.sendError(w, r, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// sendError sends an S3-formatted error response
func (s *Server) sendError(w http.ResponseWriter, r *http.Request, code, message string, statusCode int) {
	errorResp := Error{
		Code:      code,
		Message:   message,
		Resource:  r.URL.Path,
		RequestId: r.Header.Get("X-Request-ID"),
	}

	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Server", "ess-three")
	w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
	w.WriteHeader(statusCode)
	xml.NewEncoder(w).Encode(errorResp)
}
