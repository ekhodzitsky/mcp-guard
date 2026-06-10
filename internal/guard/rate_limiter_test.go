package guard

import (
	"testing"
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
