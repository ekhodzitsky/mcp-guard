// Package cache provides an in-memory TTL cache for MCP schemas.
package cache

import (
	"encoding/json"
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
	mu       sync.RWMutex
	entries  map[string]entry
	ttl      time.Duration
	stopCh   chan struct{}
	stopOnce sync.Once
}

// NewSchemaCache creates a cache with the given TTL.
func NewSchemaCache(ttl time.Duration) *SchemaCache {
	c := &SchemaCache{
		entries: make(map[string]entry),
		ttl:     ttl,
		stopCh:  make(chan struct{}),
	}
	if ttl > 0 {
		go c.sweep()
	}
	return c
}

// sweep periodically removes expired entries.
func (c *SchemaCache) sweep() {
	ticker := time.NewTicker(c.ttl)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.deleteExpired()
		case <-c.stopCh:
			return
		}
	}
}

// deleteExpired removes all expired entries while holding the write lock.
func (c *SchemaCache) deleteExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for k, v := range c.entries {
		if now.After(v.expiresAt) {
			delete(c.entries, k)
		}
	}
}

// Stop shuts down the background sweeper goroutine.
func (c *SchemaCache) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
}

// Get returns a cached response if present and not expired.
func (c *SchemaCache) Get(server string) (mcp.JSONRPCResponse, bool) {
	c.mu.RLock()
	e, ok := c.entries[server]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return mcp.JSONRPCResponse{}, false
	}
	return cloneResponse(e.resp), true
}

// cloneResponse performs a deep copy of a JSONRPCResponse.
func cloneResponse(resp mcp.JSONRPCResponse) mcp.JSONRPCResponse {
	if resp.Result != nil {
		r := make(json.RawMessage, len(resp.Result))
		copy(r, resp.Result)
		resp.Result = r
	}
	if resp.Error != nil {
		errCopy := *resp.Error
		if errCopy.Data != nil {
			d := make(json.RawMessage, len(errCopy.Data))
			copy(d, errCopy.Data)
			errCopy.Data = d
		}
		resp.Error = &errCopy
	}
	return resp
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
