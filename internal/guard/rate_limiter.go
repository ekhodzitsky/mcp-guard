package guard

import (
	"sync"
	"time"
)

// RateLimiter enforces per-tool RPM and RPD limits using a sliding window.
type RateLimiter struct {
	mu      sync.Mutex
	rpm     int // requests per minute; 0 = unlimited
	rpd     int // requests per day; 0 = unlimited
	perTool map[string]*toolWindow
	now     func() time.Time // overridden in tests
}

// toolWindow tracks individual request timestamps for exact sliding-window accounting.
type toolWindow struct {
	requests []time.Time
}

// NewRateLimiter creates a rate limiter.
func NewRateLimiter(rpm, rpd int) *RateLimiter {
	return &RateLimiter{
		rpm:     rpm,
		rpd:     rpd,
		perTool: make(map[string]*toolWindow),
	}
}

// Allow reports whether a call to the named tool is permitted.
func (r *RateLimiter) Allow(tool string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	if r.now != nil {
		now = r.now()
	}
	w, ok := r.perTool[tool]
	if !ok {
		w = &toolWindow{}
	}

	// Evict requests older than the day window.
	cutoff := now.Add(-24 * time.Hour)
	idx := 0
	for idx < len(w.requests) && w.requests[idx].Before(cutoff) {
		idx++
	}
	w.requests = w.requests[idx:]

	// Delete stale entries instead of resetting counters to prevent unbounded growth.
	if len(w.requests) == 0 {
		delete(r.perTool, tool)
	}

	// Count requests in the sliding minute window.
	minuteCutoff := now.Add(-time.Minute)
	minuteCount := 0
	for _, t := range w.requests {
		if !t.Before(minuteCutoff) {
			minuteCount++
		}
	}
	dayCount := len(w.requests)

	if r.rpm > 0 && minuteCount >= r.rpm {
		return false
	}
	if r.rpd > 0 && dayCount >= r.rpd {
		return false
	}

	w.requests = append(w.requests, now)
	r.perTool[tool] = w
	return true
}

// Reset clears limits for a tool (useful in tests).
func (r *RateLimiter) Reset(tool string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.perTool, tool)
}
