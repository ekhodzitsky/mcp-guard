package proxy

import (
	"context"
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
