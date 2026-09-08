package bunker

import (
	"sync"
	"time"
)

type rateLimitEntry struct {
	count int
	reset time.Time
}

// rateLimiter counts failures per key within a fixed window
type rateLimiter struct {
	limit  int
	window time.Duration

	mu      sync.Mutex
	entries map[string]*rateLimitEntry
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		limit:   limit,
		window:  window,
		entries: map[string]*rateLimitEntry{},
	}
}

// Allowed reports whether key may attempt again, and how long it must wait
// when it may not
func (l *rateLimiter) Allowed(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	l.pruneLocked(now)

	entry := l.entries[key]
	if entry == nil || now.After(entry.reset) {
		return true, 0
	}

	if entry.count >= l.limit {
		return false, time.Until(entry.reset)
	}

	return true, 0
}

// Fail records a failed attempt for key
func (l *rateLimiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()

	entry := l.entries[key]
	if entry == nil || now.After(entry.reset) {
		entry = &rateLimitEntry{reset: now.Add(l.window)}
		l.entries[key] = entry
	}

	entry.count++
}

// Reset forgets all failures for key
func (l *rateLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.entries, key)
}

func (l *rateLimiter) pruneLocked(now time.Time) {
	if len(l.entries) < 4096 {
		return
	}

	for key, entry := range l.entries {
		if now.After(entry.reset) {
			delete(l.entries, key)
		}
	}
}
