package integration

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/ekhodzitsky/mcp-guard/internal/audit"
	"github.com/ekhodzitsky/mcp-guard/internal/config"
	"github.com/ekhodzitsky/mcp-guard/internal/events"
	"github.com/ekhodzitsky/mcp-guard/internal/proxy"
	"github.com/ekhodzitsky/mcp-guard/internal/server"
	"github.com/ekhodzitsky/mcp-guard/pkg/mcp"
)

func setupProxyIntegration(t *testing.T) (*proxy.Proxy, *server.Pool, context.Context, context.CancelFunc) {
	t.Helper()
	bus := events.NewBus()
	cfg := config.ServerConfig{
		Command: "cat",
		Timeout: config.TimeoutConfig{ToolsCall: 30 * time.Second, ToolsList: 10 * time.Second},
	}
	pool := server.NewPool(map[string]config.ServerConfig{"echo": cfg}, bus, 5*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	if err := pool.Start(ctx); err != nil {
		cancel()
		t.Fatalf("pool start: %v", err)
	}

	p := proxy.NewProxy(pool, &audit.NoopLogger{}, nil, nil, nil, nil)
	return p, pool, ctx, cancel
}

func TestProxyWithCat(t *testing.T) {
	p, pool, ctx, cancel := setupProxyIntegration(t)
	defer cancel()
	defer func() { _ = pool.Stop(ctx) }()

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  mcp.MethodPing,
	}

	resp, err := p.Forward(ctx, "echo", req)
	if err != nil {
		t.Fatalf("forward: %v", err)
	}

	if resp.JSONRPC != "2.0" {
		t.Fatalf("expected jsonrpc 2.0, got %s", resp.JSONRPC)
	}
	reqID, ok := req.ID.(int)
	if !ok {
		t.Fatalf("expected req.ID to be int, got %T", req.ID)
	}
	if resp.ID != float64(reqID) {
		t.Fatalf("expected id %v, got %v", req.ID, resp.ID)
	}
	if resp.Error != nil {
		t.Fatalf("expected no error, got %v", resp.Error)
	}
}

func TestProxyRunLoop(t *testing.T) {
	p, pool, ctx, cancel := setupProxyIntegration(t)
	defer cancel()
	defer func() { _ = pool.Stop(ctx) }()

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      42,
		Method:  mcp.MethodPing,
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	stdin := bytes.NewReader(append(reqBytes, '\n'))
	pr, pw := io.Pipe()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, stdin, pw, "echo")
		_ = pw.Close()
	}()

	scanner := bufio.NewScanner(pr)
	if !scanner.Scan() {
		t.Fatal("expected response")
	}
	cancel()

	runErr := <-errCh
	// context.Canceled is expected because we explicitly cancel after reading
	// the first response.
	if runErr != nil && runErr != context.Canceled {
		t.Fatalf("unexpected run error: %v", runErr)
	}

	var resp mcp.JSONRPCResponse
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.JSONRPC != "2.0" {
		t.Fatalf("expected jsonrpc 2.0, got %s", resp.JSONRPC)
	}
	reqID, ok := req.ID.(int)
	if !ok {
		t.Fatalf("expected req.ID to be int, got %T", req.ID)
	}
	if resp.ID != float64(reqID) {
		t.Fatalf("expected id %v, got %v", req.ID, resp.ID)
	}
	if resp.Error != nil {
		t.Fatalf("expected no error, got %v", resp.Error)
	}
}
