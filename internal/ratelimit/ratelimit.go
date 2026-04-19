// internal/ratelimit/ratelimit.go
package ratelimit

import (
	"sync"
	"time"
)

type Limiter interface {
	// Allow returns true if the request is permitted, false if rate limited.
	// On deny, retryAfter is the duration until the window resets.
	Allow(key string, limit int, window time.Duration) (allowed bool, retryAfter time.Duration)
}

type entry struct {
	count     int
	expiresAt time.Time
}

type MemoryLimiter struct {
	mu      sync.Mutex
	entries map[string]*entry
}

func NewMemoryLimiter() *MemoryLimiter {
	l := &MemoryLimiter{
		entries: make(map[string]*entry),
	}
	go l.cleanupLoop()
	return l
}

func (l *MemoryLimiter) Allow(key string, limit int, window time.Duration) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	e, ok := l.entries[key]

	if !ok || now.After(e.expiresAt) {
		// First request or window has expired — start fresh
		l.entries[key] = &entry{
			count:     1,
			expiresAt: now.Add(window),
		}
		return true, 0
	}

	if e.count >= limit {
		return false, time.Until(e.expiresAt)
	}

	e.count++
	return true, 0
}

// RecordFailure is a semantic helper for failure-based limits (e.g., failed logins).
// It's identical to Allow but makes intent clearer at the call site.
func (l *MemoryLimiter) RecordFailure(key string, limit int, window time.Duration) (bool, time.Duration) {
	return l.Allow(key, limit, window)
}

// Reset clears the counter for a key. Call on successful login, etc.
func (l *MemoryLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

func (l *MemoryLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		for k, e := range l.entries {
			if now.After(e.expiresAt) {
				delete(l.entries, k)
			}
		}
		l.mu.Unlock()
	}
}

// Peek returns the same decision as Allow but without incrementing the counter.
// Use for "check before action" patterns like rate-limiting failed logins,
// where you want to increment only on failure.
func (l *MemoryLimiter) Peek(key string, limit int, window time.Duration) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	e, ok := l.entries[key]
	now := time.Now()

	if !ok || now.After(e.expiresAt) {
		return true, 0
	}

	if e.count >= limit {
		return false, time.Until(e.expiresAt)
	}

	return true, 0
}
