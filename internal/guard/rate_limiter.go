package guard

import (
	"sync"
	"time"
)

// RateLimiter enforces per-tool RPM and RPD limits.
type RateLimiter struct {
	mu      sync.Mutex
	rpm     int // requests per minute; 0 = unlimited
	rpd     int // requests per day; 0 = unlimited
	perTool map[string]*toolWindow
}

type toolWindow struct {
	minuteCount int
	minuteStart time.Time
	dayCount    int
	dayStart    time.Time
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
	w, ok := r.perTool[tool]
	if !ok {
		w = &toolWindow{minuteStart: now, dayStart: now}
		r.perTool[tool] = w
	}

	// Reset windows if expired.
	if now.Sub(w.minuteStart) >= time.Minute {
		w.minuteCount = 0
		w.minuteStart = now
	}
	if now.Sub(w.dayStart) >= 24*time.Hour {
		w.dayCount = 0
		w.dayStart = now
	}

	if r.rpm > 0 && w.minuteCount >= r.rpm {
		return false
	}
	if r.rpd > 0 && w.dayCount >= r.rpd {
		return false
	}

	w.minuteCount++
	w.dayCount++
	return true
}

// Reset clears limits for a tool (useful in tests).
func (r *RateLimiter) Reset(tool string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.perTool, tool)
}
