// SPDX-License-Identifier: Apache-2.0

package server

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tonyellard/essthree/internal/storage"
	"github.com/tonyellard/cloud-u-l8r/pkg/awserrors"
)

// S3 XML response structures

type ListBucketResult struct {
	XMLName               xml.Name       `xml:"ListBucketResult"`
	Xmlns                 string         `xml:"xmlns,attr"`
	Name                  string         `xml:"Name"`
	Prefix                string         `xml:"Prefix"`
	Delimiter             string         `xml:"Delimiter,omitempty"`
	Marker                string         `xml:"Marker,omitempty"`
	NextMarker            string         `xml:"NextMarker,omitempty"`
	MaxKeys               int            `xml:"MaxKeys"`
	IsTruncated           bool           `xml:"IsTruncated"`
	Contents              []Contents     `xml:"Contents"`
	CommonPrefixes        []CommonPrefix `xml:"CommonPrefixes,omitempty"`
	KeyCount              int            `xml:"KeyCount,omitempty"`
	ContinuationToken     string         `xml:"ContinuationToken,omitempty"`
	NextContinuationToken string         `xml:"NextContinuationToken,omitempty"`
}

type CommonPrefix struct {
	Prefix string `xml:"Prefix"`
}

type Contents struct {
	Key          string    `xml:"Key"`
	LastModified time.Time `xml:"LastModified"`
	ETag         string    `xml:"ETag"`
	Size         int64     `xml:"Size"`
	StorageClass string    `xml:"StorageClass"`
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

// handleCreateBucket handles PUT /{bucket} - CreateBucket
func (s *Server) handleCreateBucket(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")

	if err := s.storage.CreateBucket(bucket); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			// S3 returns 200 OK for CreateBucket on existing bucket owned by you
			w.Header().Set("Location", "/"+bucket)
			w.WriteHeader(http.StatusOK)
			return
		}
		awserrors.WriteXML(w, r, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Location", "/"+bucket)
	w.WriteHeader(http.StatusOK)
}

// handleHeadBucket handles HEAD /{bucket} - HeadBucket
func (s *Server) handleHeadBucket(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")

	if !s.storage.BucketExists(bucket) {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("x-amz-bucket-region", "us-east-1")
	w.Header().Set("Server", "ess-three")
	w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
	w.WriteHeader(http.StatusOK)
}

// handleDeleteBucket handles DELETE /{bucket} - DeleteBucket
func (s *Server) handleDeleteBucket(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")

	if err := s.storage.DeleteBucket(bucket); err != nil {
		if strings.Contains(err.Error(), "not found") {
			awserrors.WriteXML(w, r, "NoSuchBucket", "The specified bucket does not exist", http.StatusNotFound)
		} else if strings.Contains(err.Error(), "not empty") {
			awserrors.WriteXML(w, r, "BucketNotEmpty", "The bucket you tried to delete is not empty", http.StatusConflict)
		} else {
			awserrors.WriteXML(w, r, "InternalError", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleBucketGet dispatches GET /{bucket}/ based on query params
func (s *Server) handleBucketGet(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	q := r.URL.Query()

	// ListObjectVersions
	if _, ok := q["versions"]; ok {
		s.handleListObjectVersions(w, r)
		return
	}

	// Bucket property queries — return appropriate "not configured" responses
	if _, ok := q["versioning"]; ok {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><VersioningConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/"/>`)
		return
	}
	if _, ok := q["accelerate"]; ok {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><AccelerateConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/"/>`)
		return
	}
	if _, ok := q["requestPayment"]; ok {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><RequestPaymentConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Payer>BucketOwner</Payer></RequestPaymentConfiguration>`)
		return
	}
	if _, ok := q["location"]; ok {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><LocationConstraint xmlns="http://s3.amazonaws.com/doc/2006-03-01/"/>`)
		return
	}
	if _, ok := q["tagging"]; ok {
		awserrors.WriteXML(w, r, "NoSuchTagSet", "The TagSet does not exist", http.StatusNotFound)
		return
	}
	if _, ok := q["cors"]; ok {
		awserrors.WriteXML(w, r, "NoSuchCORSConfiguration", "The CORS configuration does not exist", http.StatusNotFound)
		return
	}
	if _, ok := q["encryption"]; ok {
		awserrors.WriteXML(w, r, "ServerSideEncryptionConfigurationNotFoundError", "The server side encryption configuration was not found", http.StatusNotFound)
		return
	}
	if _, ok := q["lifecycle"]; ok {
		awserrors.WriteXML(w, r, "NoSuchLifecycleConfiguration", "The lifecycle configuration does not exist", http.StatusNotFound)
		return
	}
	if _, ok := q["logging"]; ok {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><BucketLoggingStatus xmlns="http://s3.amazonaws.com/doc/2006-03-01/"/>`)
		return
	}
	if _, ok := q["policy"]; ok {
		awserrors.WriteXML(w, r, "NoSuchBucketPolicy", "The bucket policy does not exist", http.StatusNotFound)
		return
	}
	if _, ok := q["replication"]; ok {
		awserrors.WriteXML(w, r, "ReplicationConfigurationNotFoundError", "The replication configuration was not found", http.StatusNotFound)
		return
	}
	if _, ok := q["acl"]; ok {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><AccessControlPolicy xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Owner><ID>%s</ID><DisplayName>%s</DisplayName></Owner><AccessControlList><Grant><Grantee xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="CanonicalUser"><ID>%s</ID><DisplayName>%s</DisplayName></Grantee><Permission>FULL_CONTROL</Permission></Grant></AccessControlList></AccessControlPolicy>`,
			"000000000000", bucket, "000000000000", bucket)
		return
	}
	if _, ok := q["object-lock"]; ok {
		awserrors.WriteXML(w, r, "ObjectLockConfigurationNotFoundError", "Object Lock configuration does not exist for this bucket", http.StatusNotFound)
		return
	}
	if _, ok := q["policyStatus"]; ok {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><PolicyStatus xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><IsPublic>FALSE</IsPublic></PolicyStatus>`)
		return
	}
	if _, ok := q["ownershipControls"]; ok {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><OwnershipControls xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Rule><ObjectOwnership>BucketOwnerEnforced</ObjectOwnership></Rule></OwnershipControls>`)
		return
	}
	if _, ok := q["website"]; ok {
		awserrors.WriteXML(w, r, "NoSuchWebsiteConfiguration", "The specified bucket does not have a website configuration", http.StatusNotFound)
		return
	}
	if _, ok := q["publicAccessBlock"]; ok {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><PublicAccessBlockConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><BlockPublicAcls>false</BlockPublicAcls><IgnorePublicAcls>false</IgnorePublicAcls><BlockPublicPolicy>false</BlockPublicPolicy><RestrictPublicBuckets>false</RestrictPublicBuckets></PublicAccessBlockConfiguration>`)
		return
	}

	// Default: list objects
	s.handleListObjects(w, r)
}

// handleListObjectVersions handles GET /{bucket}?versions - ListObjectVersions
func (s *Server) handleListObjectVersions(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	prefix := r.URL.Query().Get("prefix")

	result, err := s.storage.ListObjectsV2(bucket, prefix, "", 1000)
	if err != nil {
		awserrors.WriteXML(w, r, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	// Build ListVersionsResult XML (versioning disabled = each object has a "null" version)
	w.Header().Set("Content-Type", "application/xml")
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><ListVersionsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Name>%s</Name><Prefix>%s</Prefix><MaxKeys>1000</MaxKeys><IsTruncated>false</IsTruncated>`, bucket, prefix)
	for _, obj := range result.Objects {
		fmt.Fprintf(w, `<Version><Key>%s</Key><VersionId>null</VersionId><IsLatest>true</IsLatest><LastModified>%s</LastModified><ETag>%s</ETag><Size>%d</Size><StorageClass>STANDARD</StorageClass></Version>`,
			obj.Key, obj.LastModified.Format(time.RFC3339), obj.ETag, obj.Size)
	}
	fmt.Fprint(w, `</ListVersionsResult>`)
}

// handleListObjects handles GET /{bucket} - ListObjects V1 and V2
func (s *Server) handleListObjects(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	prefix := r.URL.Query().Get("prefix")
	delimiter := r.URL.Query().Get("delimiter")
	prefix = s.normalizeDirectoryListPrefix(bucket, prefix, delimiter)
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
		awserrors.WriteXML(w, r, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	filteredObjects := result.Objects
	commonPrefixes := make([]string, 0)
	if delimiter != "" {
		filteredObjects, commonPrefixes = collapseByDelimiter(result.Objects, prefix, delimiter)
	}

	contents := make([]Contents, len(filteredObjects))
	for i, obj := range filteredObjects {
		contents[i] = Contents{
			Key:          obj.Key,
			LastModified: obj.LastModified,
			ETag:         obj.ETag,
			Size:         obj.Size,
			StorageClass: "STANDARD",
		}
	}

	xmlCommonPrefixes := make([]CommonPrefix, len(commonPrefixes))
	for i, p := range commonPrefixes {
		xmlCommonPrefixes[i] = CommonPrefix{Prefix: p}
	}

	response := ListBucketResult{
		Xmlns:          "http://s3.amazonaws.com/doc/2006-03-01/",
		Name:           bucket,
		Prefix:         prefix,
		Delimiter:      delimiter,
		MaxKeys:        maxKeys,
		IsTruncated:    result.IsTruncated,
		Contents:       contents,
		CommonPrefixes: xmlCommonPrefixes,
	}

	if listType == "2" {
		// V2 specific fields
		response.KeyCount = len(contents) + len(xmlCommonPrefixes)
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

func (s *Server) normalizeDirectoryListPrefix(bucket, prefix, delimiter string) string {
	if delimiter != "/" || prefix == "" || strings.HasSuffix(prefix, "/") {
		return prefix
	}

	result, err := s.storage.ListObjectsV2(bucket, prefix+"/", "", 1)
	if err != nil {
		return prefix
	}

	if len(result.Objects) > 0 {
		return prefix + "/"
	}

	return prefix
}

func collapseByDelimiter(objects []storage.ObjectMetadata, prefix, delimiter string) ([]storage.ObjectMetadata, []string) {
	filtered := make([]storage.ObjectMetadata, 0, len(objects))
	seenPrefixes := make(map[string]struct{})
	commonPrefixes := make([]string, 0)

	for _, obj := range objects {
		remainder := obj.Key
		if prefix != "" {
			remainder = strings.TrimPrefix(obj.Key, prefix)
		}

		if idx := strings.Index(remainder, delimiter); idx >= 0 {
			prefixValue := prefix + remainder[:idx+len(delimiter)]
			if _, exists := seenPrefixes[prefixValue]; !exists {
				seenPrefixes[prefixValue] = struct{}{}
				commonPrefixes = append(commonPrefixes, prefixValue)
			}
			continue
		}

		filtered = append(filtered, obj)
	}

	return filtered, commonPrefixes
}

func (s *Server) handleGetObjectOrListPrefix(w http.ResponseWriter, r *http.Request) {
	// Handle object-level query parameter APIs
	if _, ok := r.URL.Query()["tagging"]; ok {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><Tagging xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><TagSet/></Tagging>`)
		return
	}

	bucket := chi.URLParam(r, "bucket")
	key := s.objectKey(r)

	if strings.HasSuffix(key, "/") {
		s.handlePathPrefixList(w, r, key)
		return
	}

	if r.Header.Get("Range") == "" && s.keyLooksLikePrefix(bucket, key) {
		s.handlePathPrefixList(w, r, key+"/")
		return
	}

	s.handleGetObject(w, r)
}

func (s *Server) handlePathPrefixList(w http.ResponseWriter, r *http.Request, prefix string) {
	query := url.Values{}
	for k, values := range r.URL.Query() {
		for _, v := range values {
			query.Add(k, v)
		}
	}

	if query.Get("prefix") == "" {
		query.Set("prefix", prefix)
	}
	if query.Get("delimiter") == "" {
		query.Set("delimiter", "/")
	}

	clone := r.Clone(r.Context())
	clone.URL.RawQuery = query.Encode()
	s.handleListObjects(w, clone)
}

func (s *Server) keyLooksLikePrefix(bucket, key string) bool {
	if key == "" {
		return false
	}

	if _, err := s.storage.HeadObject(bucket, key); err == nil {
		return false
	}

	result, err := s.storage.ListObjectsV2(bucket, key+"/", "", 1)
	if err != nil {
		return false
	}

	return len(result.Objects) > 0
}

// handleGetObject handles GET /{bucket}/{key} - GetObject with range support
func (s *Server) handleGetObject(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	key := s.objectKey(r)

	// Check for Range header
	rangeHeader := r.Header.Get("Range")

	if rangeHeader != "" {
		// Parse range header
		rangeStart, rangeEnd, err := parseRangeHeader(rangeHeader)
		if err != nil {
			awserrors.WriteXML(w, r, "InvalidRange", err.Error(), http.StatusRequestedRangeNotSatisfiable)
			return
		}

		reader, metadata, start, end, err := s.storage.GetObjectRange(bucket, key, rangeStart, rangeEnd)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				awserrors.WriteXML(w, r, "NoSuchKey", "The specified key does not exist", http.StatusNotFound)
			} else {
				awserrors.WriteXML(w, r, "InternalError", err.Error(), http.StatusInternalServerError)
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
				awserrors.WriteXML(w, r, "NoSuchKey", "The specified key does not exist", http.StatusNotFound)
			} else {
				awserrors.WriteXML(w, r, "InternalError", err.Error(), http.StatusInternalServerError)
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
	key := s.objectKey(r)

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
		awserrors.WriteXML(w, r, "InternalError", err.Error(), http.StatusInternalServerError)
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
	key := s.objectKey(r)

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
	key := s.objectKey(r)

	err := s.storage.DeleteObject(bucket, key)
	if err != nil {
		awserrors.WriteXML(w, r, "InternalError", err.Error(), http.StatusInternalServerError)
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
		awserrors.WriteXML(w, r, "MalformedXML", "Invalid XML", http.StatusBadRequest)
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
	key := s.objectKey(r)

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
		awserrors.WriteXML(w, r, "InternalError", err.Error(), http.StatusInternalServerError)
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
	key := s.objectKey(r)
	uploadID := r.URL.Query().Get("uploadId")
	partNumberStr := r.URL.Query().Get("partNumber")

	partNumber, err := strconv.Atoi(partNumberStr)
	if err != nil {
		awserrors.WriteXML(w, r, "InvalidArgument", "Invalid part number", http.StatusBadRequest)
		return
	}

	part, err := s.storage.UploadPart(bucket, key, uploadID, partNumber, r.Body)
	if err != nil {
		awserrors.WriteXML(w, r, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("ETag", fmt.Sprintf("\"%s\"", part.ETag))
	w.WriteHeader(http.StatusOK)
}

// handleCompleteMultipartUpload handles POST /{bucket}/{key}?uploadId=X
func (s *Server) handleCompleteMultipartUpload(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	key := s.objectKey(r)
	uploadID := r.URL.Query().Get("uploadId")

	// Parse complete request
	var completeReq CompleteMultipartUploadRequest
	if err := xml.NewDecoder(r.Body).Decode(&completeReq); err != nil {
		awserrors.WriteXML(w, r, "MalformedXML", "Invalid XML", http.StatusBadRequest)
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
		awserrors.WriteXML(w, r, "InternalError", err.Error(), http.StatusInternalServerError)
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
	key := s.objectKey(r)
	uploadID := r.URL.Query().Get("uploadId")

	err := s.storage.AbortMultipartUpload(bucket, key, uploadID)
	if err != nil {
		awserrors.WriteXML(w, r, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) objectKey(r *http.Request) string {
	if key := chi.URLParam(r, "key"); key != "" {
		return strings.TrimPrefix(key, "/")
	}

	return strings.TrimPrefix(chi.URLParam(r, "*"), "/")
}
