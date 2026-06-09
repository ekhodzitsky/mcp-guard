package server

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ekhodzitsky/mcp-guard/internal/events"
	"github.com/ekhodzitsky/mcp-guard/pkg/mcp"
)

// HealthChecker pings a process periodically and reports health.
type HealthChecker struct {
	process     *Process
	bus         *events.Bus
	interval    time.Duration
	maxFailures int
	mu          sync.Mutex
	failures    int
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// NewHealthChecker creates a health checker.
func NewHealthChecker(p *Process, bus *events.Bus, interval time.Duration, maxFailures int) *HealthChecker {
	if maxFailures <= 0 {
		maxFailures = 3
	}
	return &HealthChecker{
		process:     p,
		bus:         bus,
		interval:    interval,
		maxFailures: maxFailures,
		stopCh:      make(chan struct{}),
	}
}

// Start begins health checking in a background goroutine.
func (h *HealthChecker) Start(ctx context.Context) {
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		ticker := time.NewTicker(h.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				h.check(ctx)
			case <-h.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stop stops the health checker.
func (h *HealthChecker) Stop() {
	close(h.stopCh)
	h.wg.Wait()
}

// Failures returns the current consecutive failure count.
func (h *HealthChecker) Failures() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.failures
}

func (h *HealthChecker) check(ctx context.Context) {
	if !h.process.Running() {
		h.recordFailure(ctx, "process not running")
		return
	}

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "health-check",
		Method:  mcp.MethodPing,
	}
	b, err := json.Marshal(req)
	if err != nil {
		h.recordFailure(ctx, fmt.Sprintf("marshal ping: %v", err))
		return
	}

	stdin := h.process.Stdin()
	if stdin == nil {
		h.recordFailure(ctx, "stdin unavailable")
		return
	}

	if _, err := fmt.Fprintf(stdin, "%s\n", b); err != nil {
		h.recordFailure(ctx, fmt.Sprintf("write ping: %v", err))
		return
	}

	h.mu.Lock()
	h.failures = 0
	h.mu.Unlock()

	if h.bus != nil {
		h.bus.Publish(ctx, events.Event{
			Type:   "health.ok",
			Server: h.process.Name(),
		})
	}
}

func (h *HealthChecker) recordFailure(ctx context.Context, reason string) {
	h.mu.Lock()
	h.failures++
	failures := h.failures
	h.mu.Unlock()

	if h.bus != nil {
		h.bus.Publish(ctx, events.Event{
			Type:    "health.failed",
			Server:  h.process.Name(),
			Payload: map[string]any{"reason": reason, "consecutive": failures},
		})
	}
}
