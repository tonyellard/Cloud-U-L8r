// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"bytes"
	"os"
	"testing"
)

func TestFileSystemStorage(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "ess-three-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storage, err := NewFileSystemStorage(tempDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	bucket := "test-bucket"
	key := "test-file.txt"
	content := []byte("Hello, World!")
	contentType := "text/plain"
	metadata := map[string]string{
		"author":  "test",
		"version": "1.0",
	}

	t.Run("PutObject", func(t *testing.T) {
		reader := bytes.NewReader(content)
		meta, err := storage.PutObject(bucket, key, reader, metadata, contentType)
		if err != nil {
			t.Fatalf("PutObject failed: %v", err)
		}

		if meta.Key != key {
			t.Errorf("Expected key %s, got %s", key, meta.Key)
		}
		if meta.Size != int64(len(content)) {
			t.Errorf("Expected size %d, got %d", len(content), meta.Size)
		}
		if meta.ContentType != contentType {
			t.Errorf("Expected content type %s, got %s", contentType, meta.ContentType)
		}
		if meta.Metadata["author"] != "test" {
			t.Errorf("Metadata mismatch")
		}
	})

	t.Run("GetObject", func(t *testing.T) {
		reader, meta, err := storage.GetObject(bucket, key)
		if err != nil {
			t.Fatalf("GetObject failed: %v", err)
		}
		defer reader.Close()

		buf := new(bytes.Buffer)
		buf.ReadFrom(reader)
		if !bytes.Equal(buf.Bytes(), content) {
			t.Errorf("Content mismatch")
		}

		if meta.ContentType != contentType {
			t.Errorf("Expected content type %s, got %s", contentType, meta.ContentType)
		}
	})

	t.Run("HeadObject", func(t *testing.T) {
		meta, err := storage.HeadObject(bucket, key)
		if err != nil {
			t.Fatalf("HeadObject failed: %v", err)
		}

		if meta.Size != int64(len(content)) {
			t.Errorf("Expected size %d, got %d", len(content), meta.Size)
		}
		if meta.Metadata["version"] != "1.0" {
			t.Errorf("Metadata mismatch")
		}
	})

	t.Run("ListObjects", func(t *testing.T) {
		// Add more objects
		storage.PutObject(bucket, "file1.txt", bytes.NewReader([]byte("test1")), nil, "text/plain")
		storage.PutObject(bucket, "file2.txt", bytes.NewReader([]byte("test2")), nil, "text/plain")
		storage.PutObject(bucket, "dir/file3.txt", bytes.NewReader([]byte("test3")), nil, "text/plain")

		objects, err := storage.ListObjects(bucket, "", 10)
		if err != nil {
			t.Fatalf("ListObjects failed: %v", err)
		}

		if len(objects) < 4 {
			t.Errorf("Expected at least 4 objects, got %d", len(objects))
		}

		// Test with prefix
		objects, err = storage.ListObjects(bucket, "file", 10)
		if err != nil {
			t.Fatalf("ListObjects with prefix failed: %v", err)
		}

		if len(objects) < 2 {
			t.Errorf("Expected at least 2 objects with prefix 'file', got %d", len(objects))
		}
	})

	t.Run("DeleteObject", func(t *testing.T) {
		err := storage.DeleteObject(bucket, key)
		if err != nil {
			t.Fatalf("DeleteObject failed: %v", err)
		}

		// Verify object is gone
		_, err = storage.HeadObject(bucket, key)
		if err == nil {
			t.Error("Expected error for deleted object, got nil")
		}
	})

	t.Run("NonExistentObject", func(t *testing.T) {
		_, err := storage.HeadObject(bucket, "nonexistent.txt")
		if err == nil {
			t.Error("Expected error for nonexistent object, got nil")
		}
	})
}
