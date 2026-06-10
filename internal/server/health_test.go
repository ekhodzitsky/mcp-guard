package server

import (
	"context"
	"testing"
	"time"

	"github.com/ekhodzitsky/mcp-guard/internal/config"
	"github.com/ekhodzitsky/mcp-guard/internal/events"
)

func TestHealthChecker_FailuresZeroAfterSuccessfulPing(t *testing.T) {
	bus := events.NewBus()
	cfg := config.ServerConfig{Command: "cat"}
	p := NewProcess("test", cfg, bus)
	if err := p.Start(context.Background()); err != nil {
		t.Fatalf("start process: %v", err)
	}
	defer func() { _ = p.Stop(context.Background()) }()

	checker := NewHealthChecker(p, bus, 50*time.Millisecond, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	checker.Start(ctx)
	<-ctx.Done()
	checker.Stop()

	if checker.Failures() != 0 {
		t.Fatalf("expected 0 failures, got %d", checker.Failures())
	}
}

func TestHealthChecker_FailuresIncrementWhenNotRunning(t *testing.T) {
	bus := events.NewBus()
	cfg := config.ServerConfig{Command: "cat"}
	p := NewProcess("test", cfg, bus)
	// intentionally not starting the process

	checker := NewHealthChecker(p, bus, 50*time.Millisecond, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	checker.Start(ctx)
	<-ctx.Done()
	checker.Stop()

	if checker.Failures() == 0 {
		t.Fatal("expected failures > 0, got 0")
	}
}

func TestHealthChecker_StopIsIdempotent(t *testing.T) {
	bus := events.NewBus()
	cfg := config.ServerConfig{Command: "cat"}
	p := NewProcess("test", cfg, bus)

	checker := NewHealthChecker(p, bus, 50*time.Millisecond, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	checker.Start(ctx)
	<-ctx.Done()
	checker.Stop()
	checker.Stop() // must not panic
}

func TestHealthChecker_StartAfterStopIsSafe(t *testing.T) {
	bus := events.NewBus()
	cfg := config.ServerConfig{Command: "cat"}
	p := NewProcess("test", cfg, bus)

	checker := NewHealthChecker(p, bus, 50*time.Millisecond, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	checker.Start(ctx)
	<-ctx.Done()
	checker.Stop()

	// must not panic even though Start should be called once per instance
	checker.Start(context.Background())
}

func TestHealthChecker_FailuresIncrementOnTimeout(t *testing.T) {
	bus := events.NewBus()
	// sleep does not read from stdin nor write to stdout,
	// so the ping will time out.
	cfg := config.ServerConfig{Command: "sleep", Args: []string{"100"}}
	p := NewProcess("test", cfg, bus)
	if err := p.Start(context.Background()); err != nil {
		t.Fatalf("start process: %v", err)
	}
	defer func() { _ = p.Stop(context.Background()) }()

	checker := NewHealthChecker(p, bus, 50*time.Millisecond, 2)
	checker.checkTimeout = 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	checker.Start(ctx)
	<-ctx.Done()
	checker.Stop()

	if checker.Failures() == 0 {
		t.Fatal("expected failures > 0, got 0")
	}
}

func TestHealthChecker_OKNotPublishedOnEverySuccess(t *testing.T) {
	bus := events.NewBus()
	sub := bus.Subscribe("test")
	defer bus.Unsubscribe("test", sub)

	cfg := config.ServerConfig{Command: "cat"}
	p := NewProcess("test", cfg, bus)
	if err := p.Start(context.Background()); err != nil {
		t.Fatalf("start process: %v", err)
	}
	defer func() { _ = p.Stop(context.Background()) }()

	checker := NewHealthChecker(p, bus, 50*time.Millisecond, 2)
	checker.checkTimeout = 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	checker.Start(ctx)
	<-ctx.Done()
	checker.Stop()

	var okCount int
drain:
	for {
		select {
		case evt := <-sub:
			if evt.Type == EventHealthOK {
				okCount++
			}
		case <-time.After(100 * time.Millisecond):
			break drain
		}
	}

	if okCount > 0 {
		t.Fatalf("expected no health.ok events on successful checks without prior failure, got %d", okCount)
	}
}

func TestHealthChecker_OKPublishedOnRecovery(t *testing.T) {
	bus := events.NewBus()
	sub := bus.Subscribe("test")
	defer bus.Unsubscribe("test", sub)

	// Start with a non-responsive process to accumulate failures.
	cfg := config.ServerConfig{Command: "sleep", Args: []string{"100"}}
	p := NewProcess("test", cfg, bus)
	if err := p.Start(context.Background()); err != nil {
		t.Fatalf("start process: %v", err)
	}

	checker := NewHealthChecker(p, bus, 50*time.Millisecond, 2)
	checker.checkTimeout = 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	checker.Start(ctx)
	<-ctx.Done()
	checker.Stop()

	if checker.Failures() == 0 {
		t.Fatal("expected failures > 0 before recovery")
	}

	// Stop the hung process and start a responsive one.
	_ = p.Stop(context.Background())

	cfg2 := config.ServerConfig{Command: "cat"}
	p2 := NewProcess("test", cfg2, bus)
	if err := p2.Start(context.Background()); err != nil {
		t.Fatalf("start process: %v", err)
	}
	defer func() { _ = p2.Stop(context.Background()) }()

	// Drain all stale events (process.stopped, process.started, health.failed).
drain:
	for {
		select {
		case <-sub:
		case <-time.After(200 * time.Millisecond):
			break drain
		}
	}

	checker2 := NewHealthChecker(p2, bus, 50*time.Millisecond, 2)
	checker2.checkTimeout = 100 * time.Millisecond

	// Simulate prior failures so that a successful check triggers recovery.
	checker2.mu.Lock()
	checker2.failures = 1
	checker2.mu.Unlock()

	checker2.check(context.Background())

	select {
	case evt := <-sub:
		if evt.Type != EventHealthOK {
			t.Fatalf("expected health.ok event on recovery, got %s", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected health.ok event on recovery, got none")
	}

	// A subsequent successful check must not publish health.ok again.
	checker2.check(context.Background())
	select {
	case evt := <-sub:
		if evt.Type == EventHealthOK {
			t.Fatal("health.ok should not be published on subsequent successful checks")
		}
	case <-time.After(100 * time.Millisecond):
		// expected
	}
}
