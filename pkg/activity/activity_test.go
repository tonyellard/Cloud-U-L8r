// SPDX-License-Identifier: Apache-2.0

package activity

import (
	"testing"
	"time"
)

func TestRecordAndList(t *testing.T) {
	l := NewLogger(WithMaxSize(5))

	for i := 0; i < 7; i++ {
		l.Record(Entry{
			Method:     "GET",
			Path:       "/test",
			Action:     "TestAction",
			StatusCode: 200,
		})
	}

	entries, _, _ := l.List(0, "")
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries (maxSize), got %d", len(entries))
	}
}

func TestListReverseOrder(t *testing.T) {
	l := NewLogger()

	l.Record(Entry{Method: "GET", Path: "/first", StatusCode: 200, Timestamp: time.Now().Add(-2 * time.Second)})
	l.Record(Entry{Method: "POST", Path: "/second", StatusCode: 201, Timestamp: time.Now().Add(-1 * time.Second)})
	l.Record(Entry{Method: "DELETE", Path: "/third", StatusCode: 204, Timestamp: time.Now()})

	entries, _, _ := l.List(0, "")
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Path != "/third" {
		t.Errorf("expected newest first, got %s", entries[0].Path)
	}
	if entries[2].Path != "/first" {
		t.Errorf("expected oldest last, got %s", entries[2].Path)
	}
}

func TestListPagination(t *testing.T) {
	l := NewLogger()

	for i := 0; i < 10; i++ {
		l.Record(Entry{Method: "GET", Path: "/test", StatusCode: 200})
	}

	page1, token1, err := l.List(3, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page1) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(page1))
	}
	if token1 == "" {
		t.Fatal("expected non-empty next token")
	}

	page2, token2, err := l.List(3, token1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page2) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(page2))
	}
	if token2 == "" {
		t.Fatal("expected non-empty next token")
	}
}

func TestExcludeFunc(t *testing.T) {
	l := NewLogger(WithExcludeFunc(func(e Entry) bool {
		return e.Path == "/health"
	}))

	l.Record(Entry{Method: "GET", Path: "/health", StatusCode: 200})
	l.Record(Entry{Method: "GET", Path: "/data", StatusCode: 200})

	entries, _, _ := l.List(0, "")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after exclusion, got %d", len(entries))
	}
	if entries[0].Path != "/data" {
		t.Errorf("expected /data, got %s", entries[0].Path)
	}
}

func TestSubscribe(t *testing.T) {
	l := NewLogger()
	ch := l.Subscribe()

	go func() {
		l.Record(Entry{Method: "PUT", Path: "/bucket", StatusCode: 200})
	}()

	select {
	case entry := <-ch:
		if entry.Path != "/bucket" {
			t.Errorf("expected /bucket, got %s", entry.Path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for subscriber notification")
	}

	l.Unsubscribe(ch)
}

func TestClear(t *testing.T) {
	l := NewLogger()
	l.Record(Entry{Method: "GET", Path: "/test", StatusCode: 200})
	l.Clear()

	entries, _, _ := l.List(0, "")
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries after clear, got %d", len(entries))
	}
}
