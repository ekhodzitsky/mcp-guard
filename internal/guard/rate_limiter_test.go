package guard

import (
	"testing"
	"time"
)

func TestRateLimiterRPM(t *testing.T) {
	// 2 requests per minute.
	lim := NewRateLimiter(2, 0)

	if !lim.Allow("read_file") {
		t.Fatal("expected 1st request allowed")
	}
	if !lim.Allow("read_file") {
		t.Fatal("expected 2nd request allowed")
	}
	if lim.Allow("read_file") {
		t.Fatal("expected 3rd request denied")
	}
	if !lim.Allow("write_file") {
		t.Fatal("expected different tool allowed")
	}
}

func TestRateLimiterWindow(t *testing.T) {
	lim := NewRateLimiter(1, 0)
	if !lim.Allow("tool") {
		t.Fatal("expected 1st request allowed")
	}
	if lim.Allow("tool") {
		t.Fatal("expected 2nd request denied")
	}
	// Advance window manually by resetting.
	lim.Reset("tool")
	if !lim.Allow("tool") {
		t.Fatal("expected request after reset allowed")
	}
}

func TestRateLimiterSlidingWindow(t *testing.T) {
	base := time.Now().UTC()
	lim := NewRateLimiter(1, 0)
	lim.now = func() time.Time { return base }

	if !lim.Allow("tool") {
		t.Fatal("expected 1st request allowed")
	}
	if lim.Allow("tool") {
		t.Fatal("expected 2nd request denied")
	}

	// Move time forward so the first request falls out of the sliding minute window.
	lim.now = func() time.Time { return base.Add(time.Minute + time.Second) }
	if !lim.Allow("tool") {
		t.Fatal("expected 3rd request allowed after sliding window")
	}
}

func TestRateLimiterBurstExploitPrevented(t *testing.T) {
	// With a fixed window a request at 00:59:59 and another at 01:00:00 would
	// both be allowed. The sliding window must reject the second one.
	base := time.Now().UTC().Truncate(time.Minute)
	lim := NewRateLimiter(1, 0)
	lim.now = func() time.Time { return base.Add(59 * time.Second) }

	if !lim.Allow("tool") {
		t.Fatal("expected 1st request allowed")
	}

	// Cross the fixed-window boundary; the sliding window must still deny.
	lim.now = func() time.Time { return base.Add(time.Minute) }
	if lim.Allow("tool") {
		t.Fatal("expected 2nd request denied by sliding window")
	}
}

func TestRateLimiterStaleEntryCleanup(t *testing.T) {
	base := time.Now().UTC()
	lim := NewRateLimiter(1, 0)
	lim.now = func() time.Time { return base }

	if !lim.Allow("tool") {
		t.Fatal("expected request allowed")
	}

	// After the request ages out of the day window, the stale entry should be
	// deleted instead of having its counter reset.
	lim.now = func() time.Time { return base.Add(24*time.Hour + time.Second) }
	if !lim.Allow("tool") {
		t.Fatal("expected request allowed after stale entry cleaned up")
	}

	// Verify the entry was deleted and recreated, not accumulated.
	lim.mu.Lock()
	w, ok := lim.perTool["tool"]
	lim.mu.Unlock()
	if !ok {
		t.Fatal("expected entry to exist after new request")
	}
	if len(w.requests) != 1 {
		t.Fatalf("expected 1 request in window, got %d", len(w.requests))
	}
}
