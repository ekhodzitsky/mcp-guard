package integration

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ekhodzitsky/mcp-guard/internal/audit"
	"github.com/ekhodzitsky/mcp-guard/internal/config"
	"github.com/ekhodzitsky/mcp-guard/internal/events"
	"github.com/ekhodzitsky/mcp-guard/internal/proxy"
	"github.com/ekhodzitsky/mcp-guard/internal/server"
	"github.com/ekhodzitsky/mcp-guard/pkg/mcp"
)

func TestProxyWithCat(t *testing.T) {
	bus := events.NewBus()
	cfg := config.ServerConfig{
		Command: "cat",
	}
	pool := server.NewPool(map[string]config.ServerConfig{"echo": cfg}, bus)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Start(ctx); err != nil {
		t.Fatalf("pool start: %v", err)
	}
	defer pool.Stop(ctx)

	p := proxy.NewProxy(pool, &audit.NoopLogger{}, nil)

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
}

func TestProxyRunLoop(t *testing.T) {
	bus := events.NewBus()
	cfg := config.ServerConfig{
		Command: "cat",
	}
	pool := server.NewPool(map[string]config.ServerConfig{"echo": cfg}, bus)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Start(ctx); err != nil {
		t.Fatalf("pool start: %v", err)
	}
	defer pool.Stop(ctx)

	p := proxy.NewProxy(pool, &audit.NoopLogger{}, nil)

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      42,
		Method:  mcp.MethodPing,
	}
	reqBytes, _ := json.Marshal(req)
	stdin := bytes.NewReader(append(reqBytes, '\n'))
	var stdout bytes.Buffer

	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	_ = p.Run(ctx, stdin, &stdout, "echo")

	scanner := bufio.NewScanner(&stdout)
	if !scanner.Scan() {
		t.Fatal("expected response")
	}

	var resp mcp.JSONRPCResponse
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// cat echoes back what we sent; response may not be valid JSON-RPC
	// but the proxy should have written something.
	if resp.JSONRPC == "" && !strings.Contains(scanner.Text(), "jsonrpc") {
		t.Fatalf("expected some JSON-RPC output, got: %s", scanner.Text())
	}
}
