package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
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
	cfg := config.ServerConfig{
		Command: "cat",
		Timeout: config.TimeoutConfig{ToolsCall: 30 * time.Second, ToolsList: 10 * time.Second},
	}
	pool := server.NewPool(map[string]config.ServerConfig{"echo": cfg}, bus, 5*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := pool.Start(ctx); err != nil {
		t.Fatalf("pool start: %v", err)
	}
	defer func() { _ = pool.Stop(ctx) }()

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
	pool := server.NewPool(map[string]config.ServerConfig{}, bus, 5*time.Second)

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
	cfg := config.ServerConfig{
		Command: "cat",
		Timeout: config.TimeoutConfig{ToolsCall: 30 * time.Second, ToolsList: 10 * time.Second},
	}
	pool := server.NewPool(map[string]config.ServerConfig{"echo": cfg}, bus, 5*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := pool.Start(ctx); err != nil {
		t.Fatalf("pool start: %v", err)
	}
	defer func() { _ = pool.Stop(ctx) }()

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
	cfg := config.ServerConfig{
		Command: "sleep",
		Args:    []string{"100"},
		Timeout: config.TimeoutConfig{ToolsCall: 30 * time.Second, ToolsList: 10 * time.Second},
	}
	pool := server.NewPool(map[string]config.ServerConfig{"slow": cfg}, bus, 5*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Start(ctx); err != nil {
		t.Fatalf("pool start: %v", err)
	}
	defer func() { _ = pool.Stop(ctx) }()

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

func TestProxyForwardConcurrent(t *testing.T) {
	bus := events.NewBus()
	cfg := config.ServerConfig{
		Command: "cat",
		Timeout: config.TimeoutConfig{ToolsCall: 30 * time.Second, ToolsList: 10 * time.Second},
	}
	pool := server.NewPool(map[string]config.ServerConfig{"echo": cfg}, bus, 5*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Start(ctx); err != nil {
		t.Fatalf("pool start: %v", err)
	}
	defer func() { _ = pool.Stop(ctx) }()

	logger, _ := audit.NewJSONLinesLogger("/dev/null")
	p := NewProxy(pool, logger, nil)

	var wg sync.WaitGroup
	errCh := make(chan error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: id, Method: mcp.MethodPing}
			resp, err := p.Forward(ctx, "echo", req)
			if err != nil {
				errCh <- fmt.Errorf("forward %d: %w", id, err)
				return
			}
			if fmt.Sprint(resp.ID) != fmt.Sprint(id) {
				errCh <- fmt.Errorf("forward %d: got response with id %v, want %d", id, resp.ID, id)
			}
		}(i)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Error(err)
	}
}

func TestProxyForwardDuplicateID(t *testing.T) {
	bus := events.NewBus()
	cfg := config.ServerConfig{
		Command: "sleep",
		Args:    []string{"100"},
		Timeout: config.TimeoutConfig{ToolsCall: 30 * time.Second, ToolsList: 10 * time.Second},
	}
	pool := server.NewPool(map[string]config.ServerConfig{"slow": cfg}, bus, 5*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Start(ctx); err != nil {
		t.Fatalf("pool start: %v", err)
	}
	defer func() { _ = pool.Stop(ctx) }()

	logger, _ := audit.NewJSONLinesLogger("/dev/null")
	p := NewProxy(pool, logger, nil)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 42, Method: mcp.MethodPing}

	// Start first forward in background; it will block until timeout.
	go func() {
		_, _ = p.Forward(ctx, "slow", req)
	}()

	// Give the first request time to register as pending.
	time.Sleep(50 * time.Millisecond)

	// Second forward with same ID should fail immediately.
	_, err := p.Forward(ctx, "slow", req)
	if err == nil {
		t.Fatal("expected error for duplicate request ID")
	}
	if !strings.Contains(err.Error(), "duplicate request ID") {
		t.Fatalf("expected duplicate request ID error, got: %v", err)
	}
}

func TestProxyForwardContextCancellation(t *testing.T) {
	bus := events.NewBus()
	cfg := config.ServerConfig{
		Command: "sleep",
		Args:    []string{"100"},
		Timeout: config.TimeoutConfig{ToolsCall: 30 * time.Second, ToolsList: 10 * time.Second},
	}
	pool := server.NewPool(map[string]config.ServerConfig{"slow": cfg}, bus, 5*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Start(ctx); err != nil {
		t.Fatalf("pool start: %v", err)
	}
	defer func() { _ = pool.Stop(ctx) }()

	logger, _ := audit.NewJSONLinesLogger("/dev/null")
	p := NewProxy(pool, logger, nil)

	reqCtx, reqCancel := context.WithCancel(ctx)
	defer reqCancel()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: mcp.MethodPing}

	errCh := make(chan error, 1)
	go func() {
		_, err := p.Forward(reqCtx, "slow", req)
		errCh <- err
	}()

	time.Sleep(100 * time.Millisecond)
	reqCancel()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error after context cancellation")
		}
		if err != context.Canceled {
			t.Fatalf("expected context.Canceled, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Forward to return after cancellation")
	}
}

func TestProxyRunBasic(t *testing.T) {
	bus := events.NewBus()
	cfg := config.ServerConfig{
		Command: "cat",
		Timeout: config.TimeoutConfig{ToolsCall: 30 * time.Second, ToolsList: 10 * time.Second},
	}
	pool := server.NewPool(map[string]config.ServerConfig{"echo": cfg}, bus, 5*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := pool.Start(ctx); err != nil {
		t.Fatalf("pool start: %v", err)
	}
	defer func() { _ = pool.Stop(ctx) }()

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

func TestProxyRunMalformedJSON(t *testing.T) {
	bus := events.NewBus()
	cfg := config.ServerConfig{
		Command: "cat",
		Timeout: config.TimeoutConfig{ToolsCall: 30 * time.Second, ToolsList: 10 * time.Second},
	}
	pool := server.NewPool(map[string]config.ServerConfig{"echo": cfg}, bus, 5*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := pool.Start(ctx); err != nil {
		t.Fatalf("pool start: %v", err)
	}
	defer func() { _ = pool.Stop(ctx) }()

	logger, _ := audit.NewJSONLinesLogger("/dev/null")
	p := NewProxy(pool, logger, nil)

	stdin := strings.NewReader(`not json` + "\n")
	var stdout bytes.Buffer

	err := p.Run(ctx, stdin, &stdout, "echo")
	if err != nil && err != io.EOF {
		t.Fatalf("run: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, `"error"`) {
		t.Fatalf("expected error response, got: %s", output)
	}
}

func TestProxyRunDefaultServerEmptyMultiple(t *testing.T) {
	bus := events.NewBus()
	cfg := config.ServerConfig{
		Command: "cat",
		Timeout: config.TimeoutConfig{ToolsCall: 30 * time.Second, ToolsList: 10 * time.Second},
	}
	pool := server.NewPool(map[string]config.ServerConfig{
		"srv1": cfg,
		"srv2": cfg,
	}, bus, 5*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := pool.Start(ctx); err != nil {
		t.Fatalf("pool start: %v", err)
	}
	defer func() { _ = pool.Stop(ctx) }()

	logger, _ := audit.NewJSONLinesLogger("/dev/null")
	p := NewProxy(pool, logger, nil)

	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n")
	var stdout bytes.Buffer

	err := p.Run(ctx, stdin, &stdout, "")
	if err != nil && err != io.EOF {
		t.Fatalf("run: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, `"error"`) {
		t.Fatalf("expected error response, got: %s", output)
	}
}
