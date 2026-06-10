// Package cache provides an in-memory TTL cache for MCP schemas.
package cache

import (
	"sync"
	"time"

	"github.com/ekhodzitsky/mcp-guard/pkg/mcp"
)

// entry is a cached schema response.
type entry struct {
	resp      mcp.JSONRPCResponse
	expiresAt time.Time
}

// SchemaCache caches tools/list responses with TTL.
type SchemaCache struct {
	mu      sync.RWMutex
	entries map[string]entry
	ttl     time.Duration
}

// NewSchemaCache creates a cache with the given TTL.
func NewSchemaCache(ttl time.Duration) *SchemaCache {
	return &SchemaCache{
		entries: make(map[string]entry),
		ttl:     ttl,
	}
}

// Get returns a cached response if present and not expired.
func (c *SchemaCache) Get(server string) (mcp.JSONRPCResponse, bool) {
	c.mu.RLock()
	e, ok := c.entries[server]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return mcp.JSONRPCResponse{}, false
	}
	return e.resp, true
}

// Set stores a response for a server.
func (c *SchemaCache) Set(server string, resp mcp.JSONRPCResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[server] = entry{resp: resp, expiresAt: time.Now().Add(c.ttl)}
}

// Invalidate removes a server's cached response.
func (c *SchemaCache) Invalidate(server string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, server)
}
