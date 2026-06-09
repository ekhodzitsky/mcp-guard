package proxy

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/ekhodzitsky/mcp-guard/internal/audit"
	"github.com/ekhodzitsky/mcp-guard/internal/config"
	"github.com/ekhodzitsky/mcp-guard/internal/events"
	"github.com/ekhodzitsky/mcp-guard/internal/server"
	"github.com/ekhodzitsky/mcp-guard/pkg/mcp"
)

func TestProxyForward(t *testing.T) {
	bus := events.NewBus()
	cfg := config.ServerConfig{Command: "cat"}
	pool := server.NewPool(map[string]config.ServerConfig{"echo": cfg}, bus)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := pool.Start(ctx); err != nil {
		t.Fatalf("pool start: %v", err)
	}
	defer pool.Stop(ctx)

	logger, _ := audit.NewJSONLinesLogger("/dev/null")
	p := NewProxy(pool, logger, nil)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: mcp.MethodPing}
	_, err := p.Forward(ctx, "echo", req)
	if err != nil {
		t.Fatalf("forward: %v", err)
	}
}

func TestProxyForwardUnknownServer(t *testing.T) {
	bus := events.NewBus()
	pool := server.NewPool(map[string]config.ServerConfig{}, bus)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logger, _ := audit.NewJSONLinesLogger("/dev/null")
	p := NewProxy(pool, logger, nil)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: mcp.MethodPing}
	_, err := p.Forward(ctx, "unknown", req)
	if err == nil {
		t.Fatal("expected error for unknown server")
	}
	if !strings.Contains(err.Error(), "unknown server") {
		t.Fatalf("expected 'unknown server' error, got: %v", err)
	}
}

func TestProxyForwardNotRunning(t *testing.T) {
	bus := events.NewBus()
	cfg := config.ServerConfig{Command: "cat"}
	pool := server.NewPool(map[string]config.ServerConfig{"echo": cfg}, bus)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := pool.Start(ctx); err != nil {
		t.Fatalf("pool start: %v", err)
	}
	defer pool.Stop(ctx)

	// Stop the process directly without telling the pool.
	proc := pool.Get("echo")
	if proc == nil {
		t.Fatal("expected process to exist")
	}
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	if err := proc.Stop(stopCtx); err != nil {
		t.Fatalf("stop process: %v", err)
	}

	logger, _ := audit.NewJSONLinesLogger("/dev/null")
	p := NewProxy(pool, logger, nil)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: mcp.MethodPing}
	_, err := p.Forward(ctx, "echo", req)
	if err == nil {
		t.Fatal("expected error for not running process")
	}
	if !strings.Contains(err.Error(), "not running") {
		t.Fatalf("expected 'not running' error, got: %v", err)
	}
}

func TestProxyForwardTimeout(t *testing.T) {
	bus := events.NewBus()
	cfg := config.ServerConfig{Command: "sleep", Args: []string{"100"}}
	pool := server.NewPool(map[string]config.ServerConfig{"slow": cfg}, bus)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Start(ctx); err != nil {
		t.Fatalf("pool start: %v", err)
	}
	defer pool.Stop(ctx)

	logger, _ := audit.NewJSONLinesLogger("/dev/null")
	p := NewProxy(pool, logger, nil)

	// Use a short context so the test doesn't take 30s.
	shortCtx, shortCancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer shortCancel()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: mcp.MethodPing}
	_, err := p.Forward(shortCtx, "slow", req)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got: %v", err)
	}
}

func TestProxyRunBasic(t *testing.T) {
	bus := events.NewBus()
	cfg := config.ServerConfig{Command: "cat"}
	pool := server.NewPool(map[string]config.ServerConfig{"echo": cfg}, bus)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := pool.Start(ctx); err != nil {
		t.Fatalf("pool start: %v", err)
	}
	defer pool.Stop(ctx)

	logger, _ := audit.NewJSONLinesLogger("/dev/null")
	p := NewProxy(pool, logger, nil)

	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n")
	var stdout bytes.Buffer

	err := p.Run(ctx, stdin, &stdout, "echo")
	if err != nil && err != io.EOF {
		t.Fatalf("run: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, `"jsonrpc":"2.0"`) {
		t.Fatalf("expected JSON-RPC response, got: %s", output)
	}
}
