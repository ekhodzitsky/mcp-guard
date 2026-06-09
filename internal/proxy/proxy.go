package proxy

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
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
	timeout := 30 * time.Second
	if strings.Contains(req.Method, "list") {
		timeout = 10 * time.Second
	}

	// Audit log request.
	if p.logger != nil {
		_ = p.logger.Log(ctx, audit.LogEntry{
			Timestamp: time.Now().UTC(),
			Server:    serverName,
			Direction: "request",
			Message:   req,
		})
	}

	resp, err := WithTimeout(ctx, timeout, func(ctx context.Context) (mcp.JSONRPCResponse, error) {
		return p.doForward(ctx, proc, req)
	})
	if err != nil {
		return zero, err
	}

	// Audit log response.
	if p.logger != nil {
		_ = p.logger.Log(ctx, audit.LogEntry{
			Timestamp: time.Now().UTC(),
			Server:    serverName,
			Direction: "response",
			Message:   resp,
		})
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

	scanner := proc.Scanner()
	if scanner == nil {
		return zero, mcp.ErrProcessDead
	}

	// Use a channel to read the response so we can respect context cancellation.
	type result struct {
		resp mcp.JSONRPCResponse
		err  error
	}
	resCh := make(chan result, 1)
	go func() {
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				resCh <- result{err: fmt.Errorf("scan response: %w", err)}
				return
			}
			resCh <- result{err: io.EOF}
			return
		}
		var resp mcp.JSONRPCResponse
		if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
			resCh <- result{err: fmt.Errorf("unmarshal response: %w", err)}
			return
		}
		resCh <- result{resp: resp}
	}()

	select {
	case res := <-resCh:
		return res.resp, res.err
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
				continue
			}
		}

		resp, err := p.Forward(ctx, serverName, req)
		if err != nil {
			slog.Error("forward request", "error", err, "server", serverName)
			errResp := mcp.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &mcp.JSONRPCError{
					Code:    -32000,
					Message: err.Error(),
				},
			}
			b, _ := json.Marshal(errResp)
			fmt.Fprintln(stdout, string(b))
			continue
		}

		resp.ID = req.ID
		b, err := json.Marshal(resp)
		if err != nil {
			slog.Error("marshal response", "error", err)
			continue
		}
		fmt.Fprintln(stdout, string(b))
	}
}
