package cache

import (
	"testing"
	"time"

	"github.com/ekhodzitsky/mcp-guard/pkg/mcp"
)

func TestCacheGetSet(t *testing.T) {
	c := NewSchemaCache(5 * time.Minute)
	defer c.Stop()

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
	defer c.Stop()
	c.Set("echo", mcp.JSONRPCResponse{JSONRPC: "2.0", ID: 1})

	time.Sleep(100 * time.Millisecond)
	_, ok := c.Get("echo")
	if ok {
		t.Fatal("expected cache miss after expiration")
	}
}

func TestCacheInvalidate(t *testing.T) {
	c := NewSchemaCache(5 * time.Minute)
	defer c.Stop()
	c.Set("echo", mcp.JSONRPCResponse{JSONRPC: "2.0", ID: 1})
	c.Invalidate("echo")
	_, ok := c.Get("echo")
	if ok {
		t.Fatal("expected cache miss after invalidation")
	}
}

func TestCacheDeepClone(t *testing.T) {
	c := NewSchemaCache(5 * time.Minute)
	defer c.Stop()

	resp := mcp.JSONRPCResponse{JSONRPC: "2.0", ID: 1, Result: []byte(`{"tools":[]}`)}
	c.Set("echo", resp)

	got, ok := c.Get("echo")
	if !ok {
		t.Fatal("expected cache hit")
	}

	// Mutate the returned result.
	got.Result[0] = 'X'

	got2, ok := c.Get("echo")
	if !ok {
		t.Fatal("expected cache hit after mutation")
	}
	if string(got2.Result) != `{"tools":[]}` {
		t.Fatalf("cached result was mutated: %s", string(got2.Result))
	}
}

func TestCacheSweeperRemovesExpired(t *testing.T) {
	c := NewSchemaCache(50 * time.Millisecond)
	defer c.Stop()

	c.Set("echo", mcp.JSONRPCResponse{JSONRPC: "2.0", ID: 1})

	// Wait for expiration and at least one sweep cycle.
	time.Sleep(200 * time.Millisecond)

	c.mu.RLock()
	_, ok := c.entries["echo"]
	c.mu.RUnlock()
	if ok {
		t.Fatal("expected sweeper to remove expired entry")
	}
}
