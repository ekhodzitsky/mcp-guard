package audit

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ekhodzitsky/mcp-guard/pkg/mcp"
)

func TestJSONLinesLogger(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewJSONLinesLogger(filepath.Join(dir, "audit.jsonl"))
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	defer logger.Close()

	ctx := context.Background()
	entry := LogEntry{
		Timestamp: time.Now().UTC(),
		Server:    "echo",
		Direction: "request",
		Message:   mcp.JSONRPCRequest{JSONRPC: "2.0", Method: mcp.MethodPing},
	}

	if err := logger.Log(ctx, entry); err != nil {
		t.Fatalf("log: %v", err)
	}
}

func TestSQLiteStore(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSQLiteStore(filepath.Join(dir, "audit.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	entry := LogEntry{
		Timestamp: time.Now().UTC(),
		Server:    "echo",
		Direction: "response",
		Message:   mcp.JSONRPCResponse{JSONRPC: "2.0"},
	}

	if err := store.Log(ctx, entry); err != nil {
		t.Fatalf("log: %v", err)
	}
}
