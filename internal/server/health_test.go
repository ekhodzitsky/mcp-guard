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
	defer p.Stop(context.Background())

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
