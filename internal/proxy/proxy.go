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
	"github.com/ekhodzitsky/mcp-guard/internal/server"
	"github.com/ekhodzitsky/mcp-guard/pkg/mcp"
)

// Proxy bridges client stdio with backend MCP server stdio.
type Proxy struct {
	pool       *server.Pool
	logger     audit.Logger
	semaphores map[string]chan struct{}
	mu         sync.RWMutex
}

// NewProxy creates a new proxy.
func NewProxy(pool *server.Pool, logger audit.Logger, maxCalls map[string]int) *Proxy {
	semaphores := make(map[string]chan struct{})
	if maxCalls != nil {
		for name, limit := range maxCalls {
			if limit > 0 {
				semaphores[name] = make(chan struct{}, limit)
			}
		}
	}
	return &Proxy{
		pool:       pool,
		logger:     logger,
		semaphores: semaphores,
	}
}

// Forward sends a JSON-RPC request to the named server and returns the response.
func (p *Proxy) Forward(ctx context.Context, serverName string, req mcp.JSONRPCRequest) (mcp.JSONRPCResponse, error) {
	var zero mcp.JSONRPCResponse

	proc := p.pool.Get(serverName)
	if proc == nil {
		return zero, fmt.Errorf("unknown server %q: %w", serverName, mcp.ErrProcessDead)
	}
	if !proc.Running() {
		return zero, fmt.Errorf("server %q not running: %w", serverName, mcp.ErrProcessDead)
	}

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

func (p *Proxy) doForward(ctx context.Context, proc *server.Process, req mcp.JSONRPCRequest) (mcp.JSONRPCResponse, error) {
	var zero mcp.JSONRPCResponse

	stdin := proc.Stdin()
	if stdin == nil {
		return zero, mcp.ErrProcessDead
	}

	b, err := json.Marshal(req)
	if err != nil {
		return zero, fmt.Errorf("marshal request: %w", err)
	}

	if _, err := fmt.Fprintf(stdin, "%s\n", b); err != nil {
		return zero, fmt.Errorf("write request: %w", err)
	}

	responses := proc.Responses()
	if responses == nil {
		return zero, mcp.ErrProcessDead
	}

	select {
	case line, ok := <-responses:
		if !ok {
			return zero, io.EOF
		}
		var resp mcp.JSONRPCResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			return zero, fmt.Errorf("unmarshal response: %w", err)
		}
		return resp, nil
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}

// Run starts a blocking loop that reads JSON-RPC requests from stdin and writes responses to stdout.
func (p *Proxy) Run(ctx context.Context, stdin io.Reader, stdout io.Writer, defaultServer string) error {
	scanner := bufio.NewScanner(stdin)
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

		resp.ID = req.ID
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
