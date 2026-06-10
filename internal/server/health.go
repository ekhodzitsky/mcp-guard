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

const (
	// EventHealthOK is published when a health check succeeds.
	EventHealthOK = "health.ok"
	// EventHealthFailed is published when a health check fails.
	EventHealthFailed = "health.failed"
)

// HealthChecker pings a process periodically and reports health.
type HealthChecker struct {
	process      *Process
	bus          *events.Bus
	interval     time.Duration
	checkTimeout time.Duration
	maxFailures  int
	mu           sync.Mutex
	failures     int
	stopCh       chan struct{}
	stopOnce     sync.Once
	wg           sync.WaitGroup
	started      bool
}

// NewHealthChecker creates a health checker.
func NewHealthChecker(p *Process, bus *events.Bus, interval time.Duration, maxFailures int) *HealthChecker {
	if p == nil {
		panic("NewHealthChecker: process is nil")
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if maxFailures <= 0 {
		maxFailures = 3
	}
	return &HealthChecker{
		process:      p,
		bus:          bus,
		interval:     interval,
		checkTimeout: 2 * time.Second,
		maxFailures:  maxFailures,
		stopCh:       make(chan struct{}),
	}
}

// Start begins health checking in a background goroutine.
// Start should be called once per instance; subsequent calls are no-ops.
func (h *HealthChecker) Start(ctx context.Context) {
	h.mu.Lock()
	if h.started {
		h.mu.Unlock()
		return
	}
	h.started = true
	h.wg.Add(1)
	h.mu.Unlock()
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
	h.stopOnce.Do(func() {
		close(h.stopCh)
	})
	h.mu.Lock()
	started := h.started
	h.mu.Unlock()
	if started {
		h.wg.Wait()
	}
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

	respCh := h.process.Responses()
	if respCh == nil {
		h.recordFailure(ctx, "responses channel unavailable")
		return
	}

	timer := time.NewTimer(h.checkTimeout)
	defer timer.Stop()

	select {
	case resp, ok := <-respCh:
		if !ok {
			h.recordFailure(ctx, "responses channel closed")
			return
		}
		if !isValidJSONRPC(resp) {
			h.recordFailure(ctx, "invalid jsonrpc response")
			return
		}
		h.recordSuccess(ctx)
	case <-timer.C:
		h.recordFailure(ctx, "ping response timeout")
	}
}

func isValidJSONRPC(data []byte) bool {
	var msg struct {
		JSONRPC string `json:"jsonrpc"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return false
	}
	return msg.JSONRPC == mcp.JSONRPCVersion
}

func (h *HealthChecker) recordSuccess(ctx context.Context) {
	h.mu.Lock()
	wasFailed := h.failures > 0
	h.failures = 0
	h.mu.Unlock()

	if wasFailed && h.bus != nil {
		h.bus.Publish(ctx, events.Event{
			Type:   EventHealthOK,
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
			Type:    EventHealthFailed,
			Server:  h.process.Name(),
			Payload: map[string]any{"reason": reason, "consecutive": failures},
		})
	}
}
