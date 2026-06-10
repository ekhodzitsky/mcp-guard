package cache

import (
	"testing"
	"time"

	"github.com/ekhodzitsky/mcp-guard/pkg/mcp"
)

func TestCacheGetSet(t *testing.T) {
	c := NewSchemaCache(5 * time.Minute)

	resp := mcp.JSONRPCResponse{JSONRPC: "2.0", ID: 1, Result: []byte(`{"tools":[]}`)}
	c.Set("echo", resp)

	got, ok := c.Get("echo")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if string(got.Result) != `{"tools":[]}` {
		t.Fatalf("unexpected result: %s", string(got.Result))
	}
}

func TestCacheExpiration(t *testing.T) {
	c := NewSchemaCache(50 * time.Millisecond)
	c.Set("echo", mcp.JSONRPCResponse{JSONRPC: "2.0", ID: 1})

	time.Sleep(100 * time.Millisecond)
	_, ok := c.Get("echo")
	if ok {
		t.Fatal("expected cache miss after expiration")
	}
}

func TestCacheInvalidate(t *testing.T) {
	c := NewSchemaCache(5 * time.Minute)
	c.Set("echo", mcp.JSONRPCResponse{JSONRPC: "2.0", ID: 1})
	c.Invalidate("echo")
	_, ok := c.Get("echo")
	if ok {
		t.Fatal("expected cache miss after invalidation")
	}
}
