package server

// SPDX-License-Identifier: Apache-2.0

import (
	"bytes"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/tony/ess-three/internal/storage"
)

type listResponseForTest struct {
	Contents []struct {
		Key string `xml:"Key"`
	} `xml:"Contents"`
	CommonPrefixes []struct {
		Prefix string `xml:"Prefix"`
	} `xml:"CommonPrefixes"`
}

func TestNestedObjectPathsDoNot404(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store, err := storage.NewFileSystemStorage(baseDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	srv := NewServer(store)
	router := srv.Router()

	bucket := "sync-bucket"
	keys := []string{
		"root.txt",
		"nested/level-one.txt",
		"nested/level-one/level-two.txt",
	}

	for _, key := range keys {
		putReq := httptest.NewRequest(http.MethodPut, "/"+bucket+"/"+key, bytes.NewBufferString("content:"+key))
		putReq.Header.Set("Content-Type", "text/plain")
		putResp := httptest.NewRecorder()

		router.ServeHTTP(putResp, putReq)

		if putResp.Code != http.StatusOK {
			t.Fatalf("PUT %q failed: status=%d body=%s", key, putResp.Code, putResp.Body.String())
		}

		getReq := httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key, nil)
		getResp := httptest.NewRecorder()

		router.ServeHTTP(getResp, getReq)

		if getResp.Code != http.StatusOK {
			t.Fatalf("GET %q failed: status=%d body=%s", key, getResp.Code, getResp.Body.String())
		}
	}
}

func TestListObjectsWithDelimiterBehavesLikeDirectoryListing(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store, err := storage.NewFileSystemStorage(baseDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	srv := NewServer(store)
	router := srv.Router()

	bucket := "test-bucket"
	keys := []string{
		"root.txt",
		"tester-dir/file-a.txt",
		"tester-dir/file-b.txt",
		"tester-dir/nested/file-c.txt",
		"other-dir/file-d.txt",
	}

	for _, key := range keys {
		putReq := httptest.NewRequest(http.MethodPut, "/"+bucket+"/"+key, bytes.NewBufferString("content:"+key))
		putReq.Header.Set("Content-Type", "text/plain")
		putResp := httptest.NewRecorder()
		router.ServeHTTP(putResp, putReq)
		if putResp.Code != http.StatusOK {
			t.Fatalf("PUT %q failed: status=%d body=%s", key, putResp.Code, putResp.Body.String())
		}
	}

	rootListReq := httptest.NewRequest(http.MethodGet, "/"+bucket+"/?list-type=2&delimiter=/", nil)
	rootListResp := httptest.NewRecorder()
	router.ServeHTTP(rootListResp, rootListReq)
	if rootListResp.Code != http.StatusOK {
		t.Fatalf("root list failed: status=%d body=%s", rootListResp.Code, rootListResp.Body.String())
	}

	var rootList listResponseForTest
	if err := xml.Unmarshal(rootListResp.Body.Bytes(), &rootList); err != nil {
		t.Fatalf("failed to parse root list response: %v", err)
	}

	rootKeys := make([]string, 0, len(rootList.Contents))
	for _, content := range rootList.Contents {
		rootKeys = append(rootKeys, content.Key)
	}
	sort.Strings(rootKeys)

	if len(rootKeys) != 1 || rootKeys[0] != "root.txt" {
		t.Fatalf("unexpected root keys: %v", rootKeys)
	}

	rootPrefixes := make([]string, 0, len(rootList.CommonPrefixes))
	for _, prefix := range rootList.CommonPrefixes {
		rootPrefixes = append(rootPrefixes, prefix.Prefix)
	}
	sort.Strings(rootPrefixes)

	expectedRootPrefixes := []string{"other-dir/", "tester-dir/"}
	if len(rootPrefixes) != len(expectedRootPrefixes) || rootPrefixes[0] != expectedRootPrefixes[0] || rootPrefixes[1] != expectedRootPrefixes[1] {
		t.Fatalf("unexpected root common prefixes: got=%v want=%v", rootPrefixes, expectedRootPrefixes)
	}

	subdirListReq := httptest.NewRequest(http.MethodGet, "/"+bucket+"/?list-type=2&prefix=tester-dir/&delimiter=/", nil)
	subdirListResp := httptest.NewRecorder()
	router.ServeHTTP(subdirListResp, subdirListReq)
	if subdirListResp.Code != http.StatusOK {
		t.Fatalf("subdir list failed: status=%d body=%s", subdirListResp.Code, subdirListResp.Body.String())
	}

	var subdirList listResponseForTest
	if err := xml.Unmarshal(subdirListResp.Body.Bytes(), &subdirList); err != nil {
		t.Fatalf("failed to parse subdir list response: %v", err)
	}

	subdirKeys := make([]string, 0, len(subdirList.Contents))
	for _, content := range subdirList.Contents {
		subdirKeys = append(subdirKeys, content.Key)
	}
	sort.Strings(subdirKeys)

	expectedSubdirKeys := []string{"tester-dir/file-a.txt", "tester-dir/file-b.txt"}
	if len(subdirKeys) != len(expectedSubdirKeys) || subdirKeys[0] != expectedSubdirKeys[0] || subdirKeys[1] != expectedSubdirKeys[1] {
		t.Fatalf("unexpected subdir keys: got=%v want=%v", subdirKeys, expectedSubdirKeys)
	}

	subdirPrefixes := make([]string, 0, len(subdirList.CommonPrefixes))
	for _, prefix := range subdirList.CommonPrefixes {
		subdirPrefixes = append(subdirPrefixes, prefix.Prefix)
	}

	if len(subdirPrefixes) != 1 || subdirPrefixes[0] != "tester-dir/nested/" {
		t.Fatalf("unexpected subdir common prefixes: %v", subdirPrefixes)
	}
}

func TestSubdirectoryPathListingReturnsImmediateChildren(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store, err := storage.NewFileSystemStorage(baseDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	srv := NewServer(store)
	router := srv.Router()

	bucket := "test-bucket"
	keys := []string{
		"tester-dir/file-a.txt",
		"tester-dir/file-b.txt",
		"tester-dir/nested/file-c.txt",
	}

	for _, key := range keys {
		putReq := httptest.NewRequest(http.MethodPut, "/"+bucket+"/"+key, bytes.NewBufferString("content:"+key))
		putReq.Header.Set("Content-Type", "text/plain")
		putResp := httptest.NewRecorder()
		router.ServeHTTP(putResp, putReq)
		if putResp.Code != http.StatusOK {
			t.Fatalf("PUT %q failed: status=%d body=%s", key, putResp.Code, putResp.Body.String())
		}
	}

	listReq := httptest.NewRequest(http.MethodGet, "/"+bucket+"/tester-dir/", nil)
	listResp := httptest.NewRecorder()
	router.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("subdirectory path list failed: status=%d body=%s", listResp.Code, listResp.Body.String())
	}

	var list listResponseForTest
	if err := xml.Unmarshal(listResp.Body.Bytes(), &list); err != nil {
		t.Fatalf("failed to parse path list response: %v", err)
	}

	keysOut := make([]string, 0, len(list.Contents))
	for _, content := range list.Contents {
		keysOut = append(keysOut, content.Key)
	}
	sort.Strings(keysOut)

	expectedKeys := []string{"tester-dir/file-a.txt", "tester-dir/file-b.txt"}
	if len(keysOut) != len(expectedKeys) || keysOut[0] != expectedKeys[0] || keysOut[1] != expectedKeys[1] {
		t.Fatalf("unexpected keys for subdirectory path list: got=%v want=%v", keysOut, expectedKeys)
	}

	prefixesOut := make([]string, 0, len(list.CommonPrefixes))
	for _, prefix := range list.CommonPrefixes {
		prefixesOut = append(prefixesOut, prefix.Prefix)
	}

	if len(prefixesOut) != 1 || prefixesOut[0] != "tester-dir/nested/" {
		t.Fatalf("unexpected prefixes for subdirectory path list: %v", prefixesOut)
	}
}

func TestSubdirectoryPathListingWithoutTrailingSlashReturnsImmediateChildren(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store, err := storage.NewFileSystemStorage(baseDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	srv := NewServer(store)
	router := srv.Router()

	bucket := "test-bucket"
	keys := []string{
		"tester-dir/file-a.txt",
		"tester-dir/file-b.txt",
		"tester-dir/nested/file-c.txt",
	}

	for _, key := range keys {
		putReq := httptest.NewRequest(http.MethodPut, "/"+bucket+"/"+key, bytes.NewBufferString("content:"+key))
		putReq.Header.Set("Content-Type", "text/plain")
		putResp := httptest.NewRecorder()
		router.ServeHTTP(putResp, putReq)
		if putResp.Code != http.StatusOK {
			t.Fatalf("PUT %q failed: status=%d body=%s", key, putResp.Code, putResp.Body.String())
		}
	}

	listReq := httptest.NewRequest(http.MethodGet, "/"+bucket+"/tester-dir", nil)
	listResp := httptest.NewRecorder()
	router.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("subdirectory path list (no trailing slash) failed: status=%d body=%s", listResp.Code, listResp.Body.String())
	}

	var list listResponseForTest
	if err := xml.Unmarshal(listResp.Body.Bytes(), &list); err != nil {
		t.Fatalf("failed to parse path list response: %v", err)
	}

	keysOut := make([]string, 0, len(list.Contents))
	for _, content := range list.Contents {
		keysOut = append(keysOut, content.Key)
	}
	sort.Strings(keysOut)

	expectedKeys := []string{"tester-dir/file-a.txt", "tester-dir/file-b.txt"}
	if len(keysOut) != len(expectedKeys) || keysOut[0] != expectedKeys[0] || keysOut[1] != expectedKeys[1] {
		t.Fatalf("unexpected keys for subdirectory path list without trailing slash: got=%v want=%v", keysOut, expectedKeys)
	}

	prefixesOut := make([]string, 0, len(list.CommonPrefixes))
	for _, prefix := range list.CommonPrefixes {
		prefixesOut = append(prefixesOut, prefix.Prefix)
	}

	if len(prefixesOut) != 1 || prefixesOut[0] != "tester-dir/nested/" {
		t.Fatalf("unexpected prefixes for subdirectory path list without trailing slash: %v", prefixesOut)
	}
}

func TestListObjectsWithoutTrailingSlashPrefixMatchesDirectoryListing(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store, err := storage.NewFileSystemStorage(baseDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	srv := NewServer(store)
	router := srv.Router()

	bucket := "test-bucket"
	keys := []string{
		"tester-dir/hello-world",
		"tester-dir/dir1/file-a.txt",
		"tester-dir/dir2/file-b.txt",
	}

	for _, key := range keys {
		putReq := httptest.NewRequest(http.MethodPut, "/"+bucket+"/"+key, bytes.NewBufferString("content:"+key))
		putReq.Header.Set("Content-Type", "text/plain")
		putResp := httptest.NewRecorder()
		router.ServeHTTP(putResp, putReq)
		if putResp.Code != http.StatusOK {
			t.Fatalf("PUT %q failed: status=%d body=%s", key, putResp.Code, putResp.Body.String())
		}
	}

	withoutSlashReq := httptest.NewRequest(http.MethodGet, "/"+bucket+"/?list-type=2&prefix=tester-dir&delimiter=/", nil)
	withoutSlashResp := httptest.NewRecorder()
	router.ServeHTTP(withoutSlashResp, withoutSlashReq)
	if withoutSlashResp.Code != http.StatusOK {
		t.Fatalf("list without slash failed: status=%d body=%s", withoutSlashResp.Code, withoutSlashResp.Body.String())
	}

	withSlashReq := httptest.NewRequest(http.MethodGet, "/"+bucket+"/?list-type=2&prefix=tester-dir/&delimiter=/", nil)
	withSlashResp := httptest.NewRecorder()
	router.ServeHTTP(withSlashResp, withSlashReq)
	if withSlashResp.Code != http.StatusOK {
		t.Fatalf("list with slash failed: status=%d body=%s", withSlashResp.Code, withSlashResp.Body.String())
	}

	var withoutSlash listResponseForTest
	if err := xml.Unmarshal(withoutSlashResp.Body.Bytes(), &withoutSlash); err != nil {
		t.Fatalf("failed to parse list without slash: %v", err)
	}

	var withSlash listResponseForTest
	if err := xml.Unmarshal(withSlashResp.Body.Bytes(), &withSlash); err != nil {
		t.Fatalf("failed to parse list with slash: %v", err)
	}

	withoutSlashKeys := make([]string, 0, len(withoutSlash.Contents))
	for _, content := range withoutSlash.Contents {
		withoutSlashKeys = append(withoutSlashKeys, content.Key)
	}
	withSlashKeys := make([]string, 0, len(withSlash.Contents))
	for _, content := range withSlash.Contents {
		withSlashKeys = append(withSlashKeys, content.Key)
	}
	sort.Strings(withoutSlashKeys)
	sort.Strings(withSlashKeys)

	if len(withoutSlashKeys) != len(withSlashKeys) {
		t.Fatalf("content length mismatch: without=%v with=%v", withoutSlashKeys, withSlashKeys)
	}
	for i := range withSlashKeys {
		if withoutSlashKeys[i] != withSlashKeys[i] {
			t.Fatalf("content mismatch: without=%v with=%v", withoutSlashKeys, withSlashKeys)
		}
	}

	withoutSlashPrefixes := make([]string, 0, len(withoutSlash.CommonPrefixes))
	for _, prefix := range withoutSlash.CommonPrefixes {
		withoutSlashPrefixes = append(withoutSlashPrefixes, prefix.Prefix)
	}
	withSlashPrefixes := make([]string, 0, len(withSlash.CommonPrefixes))
	for _, prefix := range withSlash.CommonPrefixes {
		withSlashPrefixes = append(withSlashPrefixes, prefix.Prefix)
	}
	sort.Strings(withoutSlashPrefixes)
	sort.Strings(withSlashPrefixes)

	if len(withoutSlashPrefixes) != len(withSlashPrefixes) {
		t.Fatalf("prefix length mismatch: without=%v with=%v", withoutSlashPrefixes, withSlashPrefixes)
	}
	for i := range withSlashPrefixes {
		if withoutSlashPrefixes[i] != withSlashPrefixes[i] {
			t.Fatalf("prefix mismatch: without=%v with=%v", withoutSlashPrefixes, withSlashPrefixes)
		}
	}
}
