// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ObjectMetadata holds metadata about stored objects
type ObjectMetadata struct {
	Key          string            `json:"key"`
	Size         int64             `json:"size"`
	LastModified time.Time         `json:"last_modified"`
	ETag         string            `json:"etag"`
	ContentType  string            `json:"content_type"`
	Metadata     map[string]string `json:"metadata"`
}

// MultipartUpload represents an ongoing multipart upload
type MultipartUpload struct {
	UploadID    string            `json:"upload_id"`
	Bucket      string            `json:"bucket"`
	Key         string            `json:"key"`
	Created     time.Time         `json:"created"`
	ContentType string            `json:"content_type"`
	Metadata    map[string]string `json:"metadata"`
}

// Part represents a single part in a multipart upload
type Part struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
	Size       int64  `json:"size"`
}

// ListResult holds paginated list results
type ListResult struct {
	Objects               []ObjectMetadata
	IsTruncated           bool
	NextMarker            string
	NextContinuationToken string
}

// Storage interface defines operations for object storage
type Storage interface {
	PutObject(bucket, key string, data io.Reader, metadata map[string]string, contentType string) (*ObjectMetadata, error)
	GetObject(bucket, key string) (io.ReadCloser, *ObjectMetadata, error)
	GetObjectRange(bucket, key string, rangeStart, rangeEnd int64) (io.ReadCloser, *ObjectMetadata, int64, int64, error)
	HeadObject(bucket, key string) (*ObjectMetadata, error)
	DeleteObject(bucket, key string) error
	DeleteObjects(bucket string, keys []string) ([]string, []error)
	ListObjects(bucket, prefix, marker string, maxKeys int) (*ListResult, error)
	ListObjectsV2(bucket, prefix, continuationToken string, maxKeys int) (*ListResult, error)

	// Multipart upload operations
	CreateMultipartUpload(bucket, key, contentType string, metadata map[string]string) (*MultipartUpload, error)
	UploadPart(bucket, key, uploadID string, partNumber int, data io.Reader) (*Part, error)
	CompleteMultipartUpload(bucket, key, uploadID string, parts []Part) (*ObjectMetadata, error)
	AbortMultipartUpload(bucket, key, uploadID string) error
	ListParts(bucket, key, uploadID string) ([]Part, error)
}

// FileSystemStorage implements Storage using the local filesystem
type FileSystemStorage struct {
	baseDir string
}

// NewFileSystemStorage creates a new filesystem-based storage backend
func NewFileSystemStorage(baseDir string) (*FileSystemStorage, error) {
	// Create base directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	return &FileSystemStorage{
		baseDir: baseDir,
	}, nil
}

// objectPath returns the filesystem path for an object
func (fs *FileSystemStorage) objectPath(bucket, key string) string {
	return filepath.Join(fs.baseDir, bucket, "objects", key)
}

// metadataPath returns the filesystem path for object metadata
func (fs *FileSystemStorage) metadataPath(bucket, key string) string {
	return filepath.Join(fs.baseDir, bucket, "metadata", key+".json")
}

// PutObject stores an object and its metadata
func (fs *FileSystemStorage) PutObject(bucket, key string, data io.Reader, metadata map[string]string, contentType string) (*ObjectMetadata, error) {
	objPath := fs.objectPath(bucket, key)
	metaPath := fs.metadataPath(bucket, key)

	// Create directories
	if err := os.MkdirAll(filepath.Dir(objPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create object directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(metaPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create metadata directory: %w", err)
	}

	// Write object data
	file, err := os.Create(objPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create object file: %w", err)
	}
	defer file.Close()

	size, err := io.Copy(file, data)
	if err != nil {
		return nil, fmt.Errorf("failed to write object data: %w", err)
	}

	// Generate ETag (simplified - just use size and timestamp)
	etag := fmt.Sprintf("\"%d-%d\"", size, time.Now().Unix())

	// Create metadata
	objMeta := &ObjectMetadata{
		Key:          key,
		Size:         size,
		LastModified: time.Now().UTC(),
		ETag:         etag,
		ContentType:  contentType,
		Metadata:     metadata,
	}

	// Write metadata
	metaFile, err := os.Create(metaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create metadata file: %w", err)
	}
	defer metaFile.Close()

	if err := json.NewEncoder(metaFile).Encode(objMeta); err != nil {
		return nil, fmt.Errorf("failed to write metadata: %w", err)
	}

	return objMeta, nil
}

// GetObject retrieves an object and its metadata
func (fs *FileSystemStorage) GetObject(bucket, key string) (io.ReadCloser, *ObjectMetadata, error) {
	objPath := fs.objectPath(bucket, key)
	metaPath := fs.metadataPath(bucket, key)

	// Check if object exists
	if _, err := os.Stat(objPath); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("object not found: %s/%s", bucket, key)
	}

	// Read metadata
	metaFile, err := os.Open(metaPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read metadata: %w", err)
	}
	defer metaFile.Close()

	var meta ObjectMetadata
	if err := json.NewDecoder(metaFile).Decode(&meta); err != nil {
		return nil, nil, fmt.Errorf("failed to decode metadata: %w", err)
	}

	// Open object file
	file, err := os.Open(objPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open object: %w", err)
	}

	return file, &meta, nil
}

// HeadObject retrieves only the metadata for an object
func (fs *FileSystemStorage) HeadObject(bucket, key string) (*ObjectMetadata, error) {
	metaPath := fs.metadataPath(bucket, key)

	metaFile, err := os.Open(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("object not found: %s/%s", bucket, key)
		}
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}
	defer metaFile.Close()

	var meta ObjectMetadata
	if err := json.NewDecoder(metaFile).Decode(&meta); err != nil {
		return nil, fmt.Errorf("failed to decode metadata: %w", err)
	}

	return &meta, nil
}

// DeleteObject removes an object and its metadata
func (fs *FileSystemStorage) DeleteObject(bucket, key string) error {
	objPath := fs.objectPath(bucket, key)
	metaPath := fs.metadataPath(bucket, key)

	// Remove object file
	if err := os.Remove(objPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	// Remove metadata file
	if err := os.Remove(metaPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete metadata: %w", err)
	}

	return nil
}

// DeleteObjects removes multiple objects
func (fs *FileSystemStorage) DeleteObjects(bucket string, keys []string) ([]string, []error) {
	var deleted []string
	var errors []error

	for _, key := range keys {
		err := fs.DeleteObject(bucket, key)
		if err != nil {
			errors = append(errors, err)
		} else {
			deleted = append(deleted, key)
		}
	}

	return deleted, errors
}

// GetObjectRange retrieves a byte range from an object
func (fs *FileSystemStorage) GetObjectRange(bucket, key string, rangeStart, rangeEnd int64) (io.ReadCloser, *ObjectMetadata, int64, int64, error) {
	objPath := fs.objectPath(bucket, key)
	metaPath := fs.metadataPath(bucket, key)

	// Check if object exists
	fileInfo, err := os.Stat(objPath)
	if os.IsNotExist(err) {
		return nil, nil, 0, 0, fmt.Errorf("object not found: %s/%s", bucket, key)
	}

	fileSize := fileInfo.Size()

	// Adjust range if end is negative or beyond file size
	if rangeEnd < 0 || rangeEnd >= fileSize {
		rangeEnd = fileSize - 1
	}
	if rangeStart < 0 {
		rangeStart = 0
	}
	if rangeStart > rangeEnd {
		return nil, nil, 0, 0, fmt.Errorf("invalid range")
	}

	// Read metadata
	metaFile, err := os.Open(metaPath)
	if err != nil {
		return nil, nil, 0, 0, fmt.Errorf("failed to read metadata: %w", err)
	}
	defer metaFile.Close()

	var meta ObjectMetadata
	if err := json.NewDecoder(metaFile).Decode(&meta); err != nil {
		return nil, nil, 0, 0, fmt.Errorf("failed to decode metadata: %w", err)
	}

	// Open file and seek to range start
	file, err := os.Open(objPath)
	if err != nil {
		return nil, nil, 0, 0, fmt.Errorf("failed to open object: %w", err)
	}

	if _, err := file.Seek(rangeStart, 0); err != nil {
		file.Close()
		return nil, nil, 0, 0, fmt.Errorf("failed to seek: %w", err)
	}

	// Create limited reader for the range
	limitedReader := &limitedReadCloser{
		Reader: io.LimitReader(file, rangeEnd-rangeStart+1),
		Closer: file,
	}

	return limitedReader, &meta, rangeStart, rangeEnd, nil
}

type limitedReadCloser struct {
	io.Reader
	io.Closer
}

// listAllObjects is a helper to get all objects sorted
func (fs *FileSystemStorage) listAllObjects(bucket, prefix string) ([]ObjectMetadata, error) {
	metadataDir := filepath.Join(fs.baseDir, bucket, "metadata")

	var objects []ObjectMetadata

	err := filepath.Walk(metadataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if filepath.Ext(path) != ".json" {
			return nil
		}

		// Read metadata
		metaFile, err := os.Open(path)
		if err != nil {
			return nil // Skip files we can't read
		}
		defer metaFile.Close()

		var meta ObjectMetadata
		if err := json.NewDecoder(metaFile).Decode(&meta); err != nil {
			return nil // Skip invalid metadata
		}

		// Filter by prefix
		if prefix == "" || strings.HasPrefix(meta.Key, prefix) {
			objects = append(objects, meta)
		}

		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	// Sort by key for consistent ordering
	sort.Slice(objects, func(i, j int) bool {
		return objects[i].Key < objects[j].Key
	})

	return objects, nil
}

// ListObjects lists objects (V1 API) with marker-based pagination
func (fs *FileSystemStorage) ListObjects(bucket, prefix, marker string, maxKeys int) (*ListResult, error) {
	if maxKeys <= 0 {
		maxKeys = 1000
	}

	allObjects, err := fs.listAllObjects(bucket, prefix)
	if err != nil {
		return nil, err
	}

	// Start from marker if provided
	startIdx := 0
	if marker != "" {
		for i, obj := range allObjects {
			if obj.Key > marker {
				startIdx = i
				break
			}
		}
	}

	// Get slice for this page
	endIdx := startIdx + maxKeys
	isTruncated := endIdx < len(allObjects)
	if endIdx > len(allObjects) {
		endIdx = len(allObjects)
	}

	pageObjects := allObjects[startIdx:endIdx]

	nextMarker := ""
	if isTruncated && len(pageObjects) > 0 {
		nextMarker = pageObjects[len(pageObjects)-1].Key
	}

	return &ListResult{
		Objects:     pageObjects,
		IsTruncated: isTruncated,
		NextMarker:  nextMarker,
	}, nil
}

// ListObjectsV2 lists objects (V2 API) with continuation token pagination
func (fs *FileSystemStorage) ListObjectsV2(bucket, prefix, continuationToken string, maxKeys int) (*ListResult, error) {
	if maxKeys <= 0 {
		maxKeys = 1000
	}

	allObjects, err := fs.listAllObjects(bucket, prefix)
	if err != nil {
		return nil, err
	}

	// Decode continuation token (it's just the last key)
	startIdx := 0
	if continuationToken != "" {
		for i, obj := range allObjects {
			if obj.Key > continuationToken {
				startIdx = i
				break
			}
		}
	}

	// Get slice for this page
	endIdx := startIdx + maxKeys
	isTruncated := endIdx < len(allObjects)
	if endIdx > len(allObjects) {
		endIdx = len(allObjects)
	}

	pageObjects := allObjects[startIdx:endIdx]

	nextToken := ""
	if isTruncated && len(pageObjects) > 0 {
		nextToken = pageObjects[len(pageObjects)-1].Key
	}

	return &ListResult{
		Objects:               pageObjects,
		IsTruncated:           isTruncated,
		NextContinuationToken: nextToken,
	}, nil
}

// multipartPath returns the directory for multipart upload data
func (fs *FileSystemStorage) multipartPath(bucket, key, uploadID string) string {
	return filepath.Join(fs.baseDir, bucket, "multipart", uploadID)
}

// CreateMultipartUpload initiates a multipart upload
func (fs *FileSystemStorage) CreateMultipartUpload(bucket, key, contentType string, metadata map[string]string) (*MultipartUpload, error) {
	// Generate upload ID (timestamp + random component)
	uploadID := fmt.Sprintf("%d-%s", time.Now().UnixNano(), generateRandomID())

	upload := &MultipartUpload{
		UploadID:    uploadID,
		Bucket:      bucket,
		Key:         key,
		Created:     time.Now().UTC(),
		ContentType: contentType,
		Metadata:    metadata,
	}

	// Create multipart directory
	mpPath := fs.multipartPath(bucket, key, uploadID)
	if err := os.MkdirAll(mpPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create multipart directory: %w", err)
	}

	// Save upload metadata
	metaPath := filepath.Join(mpPath, "upload.json")
	metaFile, err := os.Create(metaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create upload metadata: %w", err)
	}
	defer metaFile.Close()

	if err := json.NewEncoder(metaFile).Encode(upload); err != nil {
		return nil, fmt.Errorf("failed to write upload metadata: %w", err)
	}

	return upload, nil
}

// UploadPart uploads a part of a multipart upload
func (fs *FileSystemStorage) UploadPart(bucket, key, uploadID string, partNumber int, data io.Reader) (*Part, error) {
	mpPath := fs.multipartPath(bucket, key, uploadID)

	// Verify upload exists
	if _, err := os.Stat(filepath.Join(mpPath, "upload.json")); os.IsNotExist(err) {
		return nil, fmt.Errorf("upload not found: %s", uploadID)
	}

	// Write part data
	partPath := filepath.Join(mpPath, fmt.Sprintf("part-%05d", partNumber))
	partFile, err := os.Create(partPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create part file: %w", err)
	}
	defer partFile.Close()

	// Calculate MD5 hash while writing
	hasher := md5.New()
	multiWriter := io.MultiWriter(partFile, hasher)

	size, err := io.Copy(multiWriter, data)
	if err != nil {
		return nil, fmt.Errorf("failed to write part data: %w", err)
	}

	etag := hex.EncodeToString(hasher.Sum(nil))

	part := &Part{
		PartNumber: partNumber,
		ETag:       etag,
		Size:       size,
	}

	// Save part metadata
	partMetaPath := filepath.Join(mpPath, fmt.Sprintf("part-%05d.json", partNumber))
	partMetaFile, err := os.Create(partMetaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create part metadata: %w", err)
	}
	defer partMetaFile.Close()

	if err := json.NewEncoder(partMetaFile).Encode(part); err != nil {
		return nil, fmt.Errorf("failed to write part metadata: %w", err)
	}

	return part, nil
}

// CompleteMultipartUpload combines all parts into final object
func (fs *FileSystemStorage) CompleteMultipartUpload(bucket, key, uploadID string, parts []Part) (*ObjectMetadata, error) {
	mpPath := fs.multipartPath(bucket, key, uploadID)

	// Load upload metadata
	metaPath := filepath.Join(mpPath, "upload.json")
	metaFile, err := os.Open(metaPath)
	if err != nil {
		return nil, fmt.Errorf("upload not found: %w", err)
	}
	var upload MultipartUpload
	json.NewDecoder(metaFile).Decode(&upload)
	metaFile.Close()

	// Create final object file
	objPath := fs.objectPath(bucket, key)
	if err := os.MkdirAll(filepath.Dir(objPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create object directory: %w", err)
	}

	finalFile, err := os.Create(objPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create final object: %w", err)
	}
	defer finalFile.Close()

	// Sort parts by part number
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].PartNumber < parts[j].PartNumber
	})

	// Concatenate all parts
	var totalSize int64
	for _, part := range parts {
		partPath := filepath.Join(mpPath, fmt.Sprintf("part-%05d", part.PartNumber))
		partFile, err := os.Open(partPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open part %d: %w", part.PartNumber, err)
		}

		n, err := io.Copy(finalFile, partFile)
		partFile.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to copy part %d: %w", part.PartNumber, err)
		}
		totalSize += n
	}

	// Generate ETag (for multipart, it's different format)
	etag := fmt.Sprintf("\"%s-%d\"", generateRandomID(), len(parts))

	// Create object metadata
	objMeta := &ObjectMetadata{
		Key:          key,
		Size:         totalSize,
		LastModified: time.Now().UTC(),
		ETag:         etag,
		ContentType:  upload.ContentType,
		Metadata:     upload.Metadata,
	}

	// Save metadata
	metaObjPath := fs.metadataPath(bucket, key)
	if err := os.MkdirAll(filepath.Dir(metaObjPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create metadata directory: %w", err)
	}

	objMetaFile, err := os.Create(metaObjPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create object metadata: %w", err)
	}
	defer objMetaFile.Close()

	if err := json.NewEncoder(objMetaFile).Encode(objMeta); err != nil {
		return nil, fmt.Errorf("failed to write object metadata: %w", err)
	}

	// Clean up multipart directory
	os.RemoveAll(mpPath)

	return objMeta, nil
}

// AbortMultipartUpload cancels a multipart upload
func (fs *FileSystemStorage) AbortMultipartUpload(bucket, key, uploadID string) error {
	mpPath := fs.multipartPath(bucket, key, uploadID)
	return os.RemoveAll(mpPath)
}

// ListParts lists the uploaded parts
func (fs *FileSystemStorage) ListParts(bucket, key, uploadID string) ([]Part, error) {
	mpPath := fs.multipartPath(bucket, key, uploadID)

	// Verify upload exists
	if _, err := os.Stat(filepath.Join(mpPath, "upload.json")); os.IsNotExist(err) {
		return nil, fmt.Errorf("upload not found: %s", uploadID)
	}

	var parts []Part

	files, err := os.ReadDir(mpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read multipart directory: %w", err)
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") || file.Name() == "upload.json" {
			continue
		}

		partMetaPath := filepath.Join(mpPath, file.Name())
		partMetaFile, err := os.Open(partMetaPath)
		if err != nil {
			continue
		}

		var part Part
		if err := json.NewDecoder(partMetaFile).Decode(&part); err == nil {
			parts = append(parts, part)
		}
		partMetaFile.Close()
	}

	// Sort by part number
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].PartNumber < parts[j].PartNumber
	})

	return parts, nil
}

// generateRandomID generates a random ID for uploads
func generateRandomID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}
