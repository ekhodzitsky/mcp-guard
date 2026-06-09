package server

import (
	"context"
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
			pool := NewPool(map[string]config.ServerConfig{"echo": cfg}, bus)
			tt.fn(t, pool)
		})
	}
}
