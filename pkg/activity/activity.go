// SPDX-License-Identifier: Apache-2.0

// Package activity provides a shared, thread-safe activity log for
// Cloud-U-L8r service emulators. It records recent API operations in
// a bounded ring buffer and exposes paginated retrieval along with a
// real-time subscription channel for Server-Sent Events.
package activity

import (
	"fmt"
	"sync"
	"time"
)

// Entry represents a single activity log entry.
type Entry struct {
	Timestamp  time.Time `json:"timestamp"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Action     string    `json:"action,omitempty"`
	StatusCode int       `json:"statusCode"`
	ErrorType  string    `json:"errorType,omitempty"`
	Detail     string    `json:"detail,omitempty"`
}

// ExcludeFunc is called for each entry before recording. If it returns
// true the entry is silently dropped. This lets individual services
// filter out noise such as admin polling endpoints.
type ExcludeFunc func(Entry) bool

// Logger is a bounded, thread-safe activity log. It keeps the most
// recent maxSize entries in insertion order and supports real-time
// subscriber notification.
type Logger struct {
	mu          sync.RWMutex
	entries     []Entry
	maxSize     int
	exclude     ExcludeFunc
	subscribers []chan Entry
}

// Option configures a Logger at construction time.
type Option func(*Logger)

// WithMaxSize overrides the default ring buffer capacity (1000).
func WithMaxSize(n int) Option {
	return func(l *Logger) {
		if n > 0 {
			l.maxSize = n
		}
	}
}

// WithExcludeFunc installs a filter that silently drops matching entries.
func WithExcludeFunc(fn ExcludeFunc) Option {
	return func(l *Logger) {
		l.exclude = fn
	}
}

// NewLogger creates a new activity logger with the given options.
func NewLogger(opts ...Option) *Logger {
	l := &Logger{
		maxSize:     1000,
		entries:     make([]Entry, 0, 64),
		subscribers: make([]chan Entry, 0),
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Record adds a new entry to the log and notifies subscribers.
// If an ExcludeFunc was provided and returns true for this entry,
// the call is a no-op.
func (l *Logger) Record(entry Entry) {
	if l.exclude != nil && l.exclude(entry) {
		return
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	l.mu.Lock()
	l.entries = append(l.entries, entry)
	if len(l.entries) > l.maxSize {
		excess := len(l.entries) - l.maxSize
		l.entries = append([]Entry(nil), l.entries[excess:]...)
	}
	// snapshot subscribers while holding the lock
	subs := make([]chan Entry, len(l.subscribers))
	copy(subs, l.subscribers)
	l.mu.Unlock()

	// notify outside the lock
	for _, ch := range subs {
		select {
		case ch <- entry:
		default:
		}
	}
}

// List returns entries in reverse-chronological order (newest first).
// maxResults <= 0 means return all. nextToken is an opaque pagination
// cursor (an integer offset encoded as a string).
func (l *Logger) List(maxResults int, nextToken string) ([]Entry, string, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Build reversed slice.
	all := make([]Entry, len(l.entries))
	for i, e := range l.entries {
		all[len(l.entries)-1-i] = e
	}

	start, end, token, err := paginateBounds(len(all), maxResults, nextToken)
	if err != nil {
		return nil, "", err
	}
	return all[start:end], token, nil
}

// Entries returns all entries in chronological order (oldest first).
func (l *Logger) Entries() []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	out := make([]Entry, len(l.entries))
	copy(out, l.entries)
	return out
}

// Subscribe returns a channel that receives new entries as they are
// recorded. The caller must eventually call Unsubscribe to avoid leaks.
func (l *Logger) Subscribe() chan Entry {
	l.mu.Lock()
	defer l.mu.Unlock()

	ch := make(chan Entry, 16)
	l.subscribers = append(l.subscribers, ch)
	return ch
}

// Unsubscribe removes a previously subscribed channel.
func (l *Logger) Unsubscribe(ch chan Entry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for i, sub := range l.subscribers {
		if sub == ch {
			l.subscribers = append(l.subscribers[:i], l.subscribers[i+1:]...)
			close(ch)
			return
		}
	}
}

// Clear removes all entries. Useful for testing.
func (l *Logger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = make([]Entry, 0, 64)
}

// paginateBounds is a shared pagination helper identical to the one
// used throughout Cloud-U-L8r services.
func paginateBounds(total, maxResults int, nextToken string) (int, int, string, error) {
	start := 0
	if nextToken != "" {
		parsed := 0
		if _, err := fmt.Sscanf(nextToken, "%d", &parsed); err != nil || parsed < 0 || parsed > total {
			return 0, 0, "", fmt.Errorf("invalid nextToken")
		}
		start = parsed
	}

	end := total
	newToken := ""
	if maxResults > 0 {
		candidate := start + maxResults
		if candidate < total {
			end = candidate
			newToken = fmt.Sprintf("%d", candidate)
		}
	}

	if start > end {
		start = end
	}
	return start, end, newToken, nil
}
