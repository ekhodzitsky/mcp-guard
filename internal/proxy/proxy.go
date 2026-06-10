package proxy

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/ekhodzitsky/mcp-guard/internal/audit"
	"github.com/ekhodzitsky/mcp-guard/internal/cache"
	"github.com/ekhodzitsky/mcp-guard/internal/config"
	"github.com/ekhodzitsky/mcp-guard/internal/guard"
	"github.com/ekhodzitsky/mcp-guard/internal/server"
	"github.com/ekhodzitsky/mcp-guard/pkg/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

type pendingResp struct {
	ch      chan mcp.JSONRPCResponse
	created time.Time
}

// Proxy bridges client stdio with backend MCP server stdio.
type Proxy struct {
	pool       *server.Pool
	logger     audit.Logger
	semaphores map[string]chan struct{}
	mu         sync.RWMutex

	pending   map[string]pendingResp
	pendingMu sync.Mutex

	readers   map[*server.Process]struct{}
	readersMu sync.Mutex

	permissions  map[string]*guard.PermissionChecker
	rateLimiters map[string]*guard.RateLimiter
	schemaCache  *cache.SchemaCache
}

// NewProxy creates a new proxy.
func NewProxy(pool *server.Pool, logger audit.Logger, maxCalls map[string]int,
	permissions map[string]*guard.PermissionChecker,
	rateLimiters map[string]*guard.RateLimiter,
	schemaCache *cache.SchemaCache) *Proxy {
	semaphores := make(map[string]chan struct{})
	for name, limit := range maxCalls {
		if limit > 0 {
			semaphores[name] = make(chan struct{}, limit)
		}
	}
	return &Proxy{
		pool:         pool,
		logger:       logger,
		semaphores:   semaphores,
		pending:      make(map[string]pendingResp),
		readers:      make(map[*server.Process]struct{}),
		permissions:  permissions,
		rateLimiters: rateLimiters,
		schemaCache:  schemaCache,
	}
}

func extractToolName(params json.RawMessage) (string, error) {
	var p struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid tool call params: %w", err)
	}
	if p.Name == "" {
		return "", fmt.Errorf("tool name is required")
	}
	return p.Name, nil
}

// Forward sends a JSON-RPC request to the named server and returns the response.
func (p *Proxy) Forward(ctx context.Context, serverName string, req mcp.JSONRPCRequest) (mcp.JSONRPCResponse, error) {
	ctx, span := otel.Tracer("mcp-guard").Start(ctx, "proxy.Forward")
	defer span.End()
	span.SetAttributes(
		attribute.String("server", serverName),
		attribute.String("method", req.Method),
	)
	var zero mcp.JSONRPCResponse

	// Schema cache for tools/list.
	if req.Method == mcp.MethodToolsList && p.schemaCache != nil {
		if cached, ok := p.schemaCache.Get(serverName); ok {
			// Audit log cached response.
			if p.logger != nil {
				_ = p.logger.Log(ctx, audit.LogEntry{
					Timestamp: time.Now().UTC(),
					Server:    serverName,
					Direction: "response",
					Message:   cached,
				})
			}
			return cached, nil
		}
	}

	// Permission and rate limit checks for tools/call.
	if req.Method == mcp.MethodToolsCall {
		toolName, err := extractToolName(req.Params)
		if err != nil {
			return zero, err
		}

		if checker := p.permissions[serverName]; checker != nil {
			if !checker.IsAllowed(toolName) {
				return zero, fmt.Errorf("tool %q is not permitted: %w", toolName, config.ErrInvalidConfig)
			}
		}

		if lim := p.rateLimiters[serverName]; lim != nil {
			if !lim.Allow(toolName) {
				return zero, fmt.Errorf("rate limit exceeded for tool %q: %w", toolName, ErrTimeout)
			}
		}
	}

	proc := p.pool.Get(serverName)
	if proc == nil {
		return zero, fmt.Errorf("unknown server %q: %w", serverName, server.ErrProcessDead)
	}
	if !proc.Running() {
		return zero, fmt.Errorf("server %q not running: %w", serverName, server.ErrProcessDead)
	}

	p.ensureReaderStarted(proc)

	// Semaphore for max concurrent calls.
	p.mu.RLock()
	sem := p.semaphores[serverName]
	p.mu.RUnlock()
	if sem != nil {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
		case <-ctx.Done():
			return zero, ctx.Err()
		}
	}

	// Determine timeout.
	timeout := proc.TimeoutConfig().ToolsCall
	if req.Method == mcp.MethodToolsList {
		timeout = proc.TimeoutConfig().ToolsList
	}

	// Audit log request.
	if p.logger != nil {
		if err := p.logger.Log(ctx, audit.LogEntry{
			Timestamp: time.Now().UTC(),
			Server:    serverName,
			Direction: "request",
			Message:   req,
		}); err != nil {
			slog.Warn("audit log request failed", "error", err)
		}
	}

	resp, err := WithTimeout(ctx, timeout, func(ctx context.Context) (mcp.JSONRPCResponse, error) {
		return p.doForward(ctx, proc, req)
	})
	if err != nil {
		return zero, err
	}

	// Cache tools/list responses.
	if req.Method == mcp.MethodToolsList && p.schemaCache != nil {
		p.schemaCache.Set(serverName, resp)
	}

	// Invalidate cache on list_changed notification.
	if req.Method == mcp.MethodToolsListChanged && p.schemaCache != nil {
		p.schemaCache.Invalidate(serverName)
	}

	// Audit log response.
	if p.logger != nil {
		if err := p.logger.Log(ctx, audit.LogEntry{
			Timestamp: time.Now().UTC(),
			Server:    serverName,
			Direction: "response",
			Message:   resp,
		}); err != nil {
			slog.Warn("audit log response failed", "error", err)
		}
	}

	return resp, nil
}

func (p *Proxy) ensureReaderStarted(proc *server.Process) {
	p.readersMu.Lock()
	defer p.readersMu.Unlock()
	if _, ok := p.readers[proc]; ok {
		return
	}
	p.readers[proc] = struct{}{}
	go p.readResponses(proc)
}

func (p *Proxy) readResponses(proc *server.Process) {
	defer func() {
		p.readersMu.Lock()
		delete(p.readers, proc)
		p.readersMu.Unlock()
	}()

	ch := proc.Responses()
	if ch == nil {
		return
	}
	for line := range ch {
		var resp mcp.JSONRPCResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			slog.Warn("unmarshal response", "error", err)
			continue
		}
		idStr := mcp.RequestID{Value: resp.ID}.String()
		p.pendingMu.Lock()
		pr, ok := p.pending[idStr]
		if ok {
			delete(p.pending, idStr)
		}
		p.pendingMu.Unlock()
		if ok {
			select {
			case pr.ch <- resp:
			default:
			}
		} else {
			slog.Warn("orphan response", "id", resp.ID)
		}
	}
}

func (p *Proxy) doForward(ctx context.Context, proc *server.Process, req mcp.JSONRPCRequest) (mcp.JSONRPCResponse, error) {
	ctx, span := otel.Tracer("mcp-guard").Start(ctx, "proxy.doForward")
	defer span.End()
	span.SetAttributes(attribute.String("process", proc.Name()))
	var zero mcp.JSONRPCResponse

	stdin := proc.Stdin()
	if stdin == nil {
		return zero, server.ErrProcessDead
	}

	b, err := json.Marshal(req)
	if err != nil {
		return zero, fmt.Errorf("marshal request: %w", err)
	}

	// Notifications have no ID and do not expect a response.
	if req.ID == nil {
		if _, err := fmt.Fprintf(stdin, "%s\n", b); err != nil {
			return zero, fmt.Errorf("write request: %w", err)
		}
		return zero, nil
	}

	// Register pending response.
	idStr := mcp.RequestID{Value: req.ID}.String()
	respCh := make(chan mcp.JSONRPCResponse, 1)

	p.pendingMu.Lock()
	if _, exists := p.pending[idStr]; exists {
		p.pendingMu.Unlock()
		return zero, fmt.Errorf("duplicate request ID %v", req.ID)
	}
	p.pending[idStr] = pendingResp{ch: respCh, created: time.Now()}
	p.pendingMu.Unlock()

	// Unregister on exit.
	defer func() {
		p.pendingMu.Lock()
		delete(p.pending, idStr)
		p.pendingMu.Unlock()
	}()

	if _, err := fmt.Fprintf(stdin, "%s\n", b); err != nil {
		return zero, fmt.Errorf("write request: %w", err)
	}

	select {
	case resp, ok := <-respCh:
		if !ok {
			return zero, io.EOF
		}
		return resp, nil
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}

// Run starts a blocking loop that reads JSON-RPC requests from stdin and writes responses to stdout.
func (p *Proxy) Run(ctx context.Context, stdin io.Reader, stdout io.Writer, defaultServer string) error {
	ctx, span := otel.Tracer("mcp-guard").Start(ctx, "proxy.Run")
	defer span.End()
	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 4096), 10*1024*1024)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("scan stdin: %w", err)
			}
			return io.EOF
		}

		var req mcp.JSONRPCRequest
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			slog.Error("unmarshal request", "error", err)
			if werr := p.sendError(stdout, nil, -32700, "parse error"); werr != nil {
				return werr
			}
			continue
		}

		serverName := defaultServer
		// If no default server and multiple exist, this is an error for MVP.
		if serverName == "" {
			names := p.pool.Names()
			if len(names) == 1 {
				serverName = names[0]
			} else {
				slog.Error("cannot determine target server", "method", req.Method)
				if werr := p.sendError(stdout, req.ID, -32000, "cannot determine target server"); werr != nil {
					return werr
				}
				continue
			}
		}

		resp, err := p.Forward(ctx, serverName, req)
		if err != nil {
			slog.Error("forward request", "error", err, "server", serverName)
			if werr := p.sendError(stdout, req.ID, -32000, err.Error()); werr != nil {
				return werr
			}
			continue
		}

		if req.ID == nil {
			// Notification: no response expected.
			continue
		}

		b, err := json.Marshal(resp)
		if err != nil {
			slog.Error("marshal response", "error", err)
			if werr := p.sendError(stdout, req.ID, -32603, "internal error"); werr != nil {
				return werr
			}
			continue
		}
		if _, werr := fmt.Fprintln(stdout, string(b)); werr != nil {
			return fmt.Errorf("write response: %w", werr)
		}
	}
}

func (p *Proxy) sendError(stdout io.Writer, id any, code int, message string) error {
	errResp := mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &mcp.JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
	b, err := json.Marshal(errResp)
	if err != nil {
		slog.Error("marshal error response", "error", err)
		b = []byte(`{"jsonrpc":"2.0","id":null,"error":{"code":-32603,"message":"internal error"}}`)
	}
	if _, werr := fmt.Fprintln(stdout, string(b)); werr != nil {
		return fmt.Errorf("write error response: %w", werr)
	}
	return nil
}
