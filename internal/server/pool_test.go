package server

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/ekhodzitsky/mcp-guard/internal/config"
	"github.com/ekhodzitsky/mcp-guard/internal/events"
)

func TestPool(t *testing.T) {
	bus := events.NewBus()
	cfg := config.ServerConfig{Command: "cat"}

	tests := []struct {
		name string
		fn   func(t *testing.T, pool *Pool)
	}{
		{
			name: "happy path start stop",
			fn: func(t *testing.T, pool *Pool) {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()

				if err := pool.Start(ctx); err != nil {
					t.Fatalf("pool start: %v", err)
				}

				proc := pool.Get("echo")
				if proc == nil {
					t.Fatal("expected process")
				}

				stopCtx, cancelStop := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancelStop()
				if err := pool.Stop(stopCtx); err != nil {
					t.Fatalf("pool stop: %v", err)
				}
			},
		},
		{
			name: "get returns process",
			fn: func(t *testing.T, pool *Pool) {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()

				if err := pool.Start(ctx); err != nil {
					t.Fatalf("pool start: %v", err)
				}
				defer func() {
					stopCtx, cancelStop := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancelStop()
					_ = pool.Stop(stopCtx)
				}()

				proc := pool.Get("echo")
				if proc == nil {
					t.Fatal("expected process for echo")
				}
			},
		},
		{
			name: "names returns configured names",
			fn: func(t *testing.T, pool *Pool) {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()

				if err := pool.Start(ctx); err != nil {
					t.Fatalf("pool start: %v", err)
				}
				defer func() {
					stopCtx, cancelStop := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancelStop()
					_ = pool.Stop(stopCtx)
				}()

				names := pool.Names()
				if len(names) != 1 || names[0] != "echo" {
					t.Fatalf("expected names [echo], got %v", names)
				}
			},
		},
		{
			name: "restart works",
			fn: func(t *testing.T, pool *Pool) {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()

				if err := pool.Start(ctx); err != nil {
					t.Fatalf("pool start: %v", err)
				}
				defer func() {
					stopCtx, cancelStop := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancelStop()
					_ = pool.Stop(stopCtx)
				}()

				old := pool.Get("echo")
				if old == nil {
					t.Fatal("expected process")
				}

				if err := pool.Restart(ctx, "echo"); err != nil {
					t.Fatalf("restart: %v", err)
				}

				new := pool.Get("echo")
				if new == nil {
					t.Fatal("expected process after restart")
				}
				if old == new {
					t.Fatal("expected new process after restart")
				}
			},
		},
		{
			name: "get unknown returns nil",
			fn: func(t *testing.T, pool *Pool) {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()

				if err := pool.Start(ctx); err != nil {
					t.Fatalf("pool start: %v", err)
				}
				defer func() {
					stopCtx, cancelStop := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancelStop()
					_ = pool.Stop(stopCtx)
				}()

				if pool.Get("unknown") != nil {
					t.Fatal("expected nil for unknown")
				}
			},
		},
		{
			name: "double start returns error",
			fn: func(t *testing.T, pool *Pool) {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()

				if err := pool.Start(ctx); err != nil {
					t.Fatalf("pool start: %v", err)
				}
				defer func() {
					stopCtx, cancelStop := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancelStop()
					_ = pool.Stop(stopCtx)
				}()

				if err := pool.Start(ctx); err == nil {
					t.Fatal("expected error on double start")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := NewPool(map[string]config.ServerConfig{"echo": cfg}, bus, 5*time.Second)
			tt.fn(t, pool)
		})
	}
}

func TestPoolPartialStartFailure(t *testing.T) {
	bus := events.NewBus()
	pool := NewPool(map[string]config.ServerConfig{
		"ok":   {Command: "cat"},
		"fail": {Command: "nonexistent-command-12345"},
	}, bus, 5*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := pool.Start(ctx); err == nil {
		t.Fatal("expected start error")
	}

	if pool.Get("ok") != nil {
		t.Fatal("expected ok process to be cleaned up after partial start failure")
	}
	if len(pool.Names()) != 0 {
		t.Fatalf("expected no running names, got %v", pool.Names())
	}
}

func TestPoolDoubleStartNoLeak(t *testing.T) {
	pool := NewPool(map[string]config.ServerConfig{"echo": {Command: "cat"}}, nil, 5*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := pool.Start(ctx); err != nil {
		t.Fatalf("pool start: %v", err)
	}
	defer func() {
		stopCtx, cancelStop := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelStop()
		_ = pool.Stop(stopCtx)
	}()

	// Give goroutines time to settle
	runtime.Gosched()
	before := runtime.NumGoroutine()

	if err := pool.Start(ctx); err == nil {
		t.Fatal("expected error on double start")
	}

	runtime.Gosched()
	after := runtime.NumGoroutine()

	if after > before {
		t.Fatalf("goroutine leak: before=%d after=%d", before, after)
	}
}

func TestPoolStopClearsMaps(t *testing.T) {
	bus := events.NewBus()
	pool := NewPool(map[string]config.ServerConfig{"echo": {Command: "cat"}}, bus, 5*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := pool.Start(ctx); err != nil {
		t.Fatalf("pool start: %v", err)
	}

	stopCtx, cancelStop := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelStop()
	if err := pool.Stop(stopCtx); err != nil {
		t.Fatalf("pool stop: %v", err)
	}

	if pool.Get("echo") != nil {
		t.Fatal("expected Get to return nil after Stop")
	}
	if len(pool.Names()) != 0 {
		t.Fatalf("expected Names to be empty after Stop, got %v", pool.Names())
	}
}

func TestPoolRestartFailureClearsMaps(t *testing.T) {
	bus := events.NewBus()
	pool := NewPool(map[string]config.ServerConfig{"echo": {Command: "cat"}}, bus, 5*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := pool.Start(ctx); err != nil {
		t.Fatalf("pool start: %v", err)
	}
	defer func() {
		stopCtx, cancelStop := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelStop()
		_ = pool.Stop(stopCtx)
	}()

	// Corrupt the config so restart fails
	pool.config["echo"] = config.ServerConfig{Command: "nonexistent-command-12345"}

	if err := pool.Restart(ctx, "echo"); err == nil {
		t.Fatal("expected restart error")
	}

	if pool.Get("echo") != nil {
		t.Fatal("expected Get to return nil after failed restart")
	}
	if len(pool.Names()) != 0 {
		t.Fatalf("expected Names to be empty after failed restart, got %v", pool.Names())
	}
}

func TestPoolConcurrentRestart(t *testing.T) {
	bus := events.NewBus()
	pool := NewPool(map[string]config.ServerConfig{"echo": {Command: "cat"}}, bus, 5*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Start(ctx); err != nil {
		t.Fatalf("pool start: %v", err)
	}
	defer func() {
		stopCtx, cancelStop := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelStop()
		_ = pool.Stop(stopCtx)
	}()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = pool.Restart(ctx, "echo")
		}()
	}
	wg.Wait()

	if pool.Get("echo") == nil {
		t.Fatal("expected process after concurrent restarts")
	}
}
