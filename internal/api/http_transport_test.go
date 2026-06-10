package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ekhodzitsky/mcp-guard/internal/audit"
	"github.com/ekhodzitsky/mcp-guard/internal/config"
	"github.com/ekhodzitsky/mcp-guard/internal/events"
	"github.com/ekhodzitsky/mcp-guard/internal/proxy"
	"github.com/ekhodzitsky/mcp-guard/internal/server"
	"github.com/ekhodzitsky/mcp-guard/pkg/mcp"
)

func TestHTTPTransport(t *testing.T) {
	bus := events.NewBus()
	cfg := config.ServerConfig{
		Command: "cat",
		Timeout: config.TimeoutConfig{ToolsCall: 30 * time.Second, ToolsList: 10 * time.Second},
	}
	pool := server.NewPool(map[string]config.ServerConfig{"echo": cfg}, bus, 5*time.Second)
	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pool.Stop(ctx) }()

	p := proxy.NewProxy(pool, &audit.NoopLogger{}, nil, nil, nil, nil)
	transport := NewHTTPTransport(p, "echo")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: mcp.MethodPing}
	body, _ := json.Marshal(req)

	r := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	w := httptest.NewRecorder()
	transport.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
