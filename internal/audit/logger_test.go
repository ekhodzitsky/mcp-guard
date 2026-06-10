package audit

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ekhodzitsky/mcp-guard/pkg/mcp"
)

type mockLogger struct {
	logErr   error
	closeErr error
	entries  []LogEntry
	closed   bool
}

func (m *mockLogger) Log(_ context.Context, entry LogEntry) error {
	if m.closed {
		return errors.New("mock logger: closed")
	}
	m.entries = append(m.entries, entry)
	return m.logErr
}

func (m *mockLogger) Close() error {
	m.closed = true
	return m.closeErr
}

func TestJSONLinesLogger(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []LogEntry
	}{
		{
			name: "single entry",
			entries: []LogEntry{
				{
					Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					Server:    "echo",
					Direction: "request",
					Message:   mcp.JSONRPCRequest{JSONRPC: "2.0", Method: mcp.MethodPing},
				},
			},
		},
		{
			name: "multiple entries",
			entries: []LogEntry{
				{
					Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					Server:    "echo",
					Direction: "request",
					Message:   mcp.JSONRPCRequest{JSONRPC: "2.0", Method: mcp.MethodPing},
				},
				{
					Timestamp: time.Date(2024, 1, 1, 0, 0, 1, 0, time.UTC),
					Server:    "echo",
					Direction: "response",
					Message:   mcp.JSONRPCResponse{JSONRPC: "2.0"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := filepath.Join(dir, "audit.jsonl")
			logger, err := NewJSONLinesLogger(path)
			if err != nil {
				t.Fatalf("new logger: %v", err)
			}

			ctx := context.Background()
			for _, entry := range tt.entries {
				if err := logger.Log(ctx, entry); err != nil {
					t.Fatalf("log: %v", err)
				}
			}

			if err := logger.Close(); err != nil {
				t.Fatalf("close: %v", err)
			}

			// Read back and assert
			// #nosec G304
			f, err := os.Open(path)
			if err != nil {
				t.Fatalf("open file: %v", err)
			}
			defer func() { _ = f.Close() }()

			var readEntries []LogEntry
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.TrimSpace(line) == "" {
					continue
				}
				var re LogEntry
				if err := json.Unmarshal([]byte(line), &re); err != nil {
					t.Fatalf("unmarshal line: %v", err)
				}
				readEntries = append(readEntries, re)
			}
			if err := scanner.Err(); err != nil {
				t.Fatalf("scan file: %v", err)
			}

			if len(readEntries) != len(tt.entries) {
				t.Fatalf("expected %d entries, got %d", len(tt.entries), len(readEntries))
			}

			for i, expected := range tt.entries {
				got := readEntries[i]
				if got.Server != expected.Server {
					t.Errorf("entry %d server: want %q, got %q", i, expected.Server, got.Server)
				}
				if got.Direction != expected.Direction {
					t.Errorf("entry %d direction: want %q, got %q", i, expected.Direction, got.Direction)
				}
				wantMsg, err := json.Marshal(expected.Message)
				if err != nil {
					t.Fatalf("marshal expected message: %v", err)
				}
				gotMsg, err := json.Marshal(got.Message)
				if err != nil {
					t.Fatalf("marshal got message: %v", err)
				}
				if string(wantMsg) != string(gotMsg) {
					t.Errorf("entry %d message: want %s, got %s", i, wantMsg, gotMsg)
				}
			}

			// Edge case: Log after Close should error
			logger2, err := NewJSONLinesLogger(filepath.Join(dir, "audit2.jsonl"))
			if err != nil {
				t.Fatalf("new logger2: %v", err)
			}
			if err := logger2.Close(); err != nil {
				t.Fatalf("close logger2: %v", err)
			}
			if err := logger2.Log(ctx, tt.entries[0]); err == nil {
				t.Fatal("expected error logging after close, got nil")
			}
		})
	}
}

func TestSQLiteStore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []LogEntry
	}{
		{
			name: "single entry",
			entries: []LogEntry{
				{
					Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					Server:    "echo",
					Direction: "request",
					Message:   mcp.JSONRPCRequest{JSONRPC: "2.0", Method: mcp.MethodPing},
				},
			},
		},
		{
			name: "multiple entries",
			entries: []LogEntry{
				{
					Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					Server:    "echo",
					Direction: "request",
					Message:   mcp.JSONRPCRequest{JSONRPC: "2.0", Method: mcp.MethodPing},
				},
				{
					Timestamp: time.Date(2024, 1, 1, 0, 0, 1, 0, time.UTC),
					Server:    "echo",
					Direction: "response",
					Message:   mcp.JSONRPCResponse{JSONRPC: "2.0"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := filepath.Join(dir, "audit.db")
			store, err := NewSQLiteStore(path)
			if err != nil {
				t.Fatalf("new store: %v", err)
			}

			ctx := context.Background()
			for _, entry := range tt.entries {
				if err := store.Log(ctx, entry); err != nil {
					t.Fatalf("log: %v", err)
				}
			}

			// Query back and assert
			rows, err := store.db.Query(
				"SELECT timestamp, server, direction, message FROM audit_log ORDER BY id",
			)
			if err != nil {
				t.Fatalf("query: %v", err)
			}
			defer func() { _ = rows.Close() }()

			var readEntries []LogEntry
			for rows.Next() {
				var ts string
				var server, direction, msgJSON string
				if err := rows.Scan(&ts, &server, &direction, &msgJSON); err != nil {
					t.Fatalf("scan: %v", err)
				}
				var msg any
				if err := json.Unmarshal([]byte(msgJSON), &msg); err != nil {
					t.Fatalf("unmarshal message: %v", err)
				}
				readEntries = append(readEntries, LogEntry{
					Server:    server,
					Direction: direction,
					Message:   msg,
				})
			}
			if err := rows.Err(); err != nil {
				t.Fatalf("rows error: %v", err)
			}

			if len(readEntries) != len(tt.entries) {
				t.Fatalf("expected %d entries, got %d", len(tt.entries), len(readEntries))
			}

			for i, expected := range tt.entries {
				got := readEntries[i]
				if got.Server != expected.Server {
					t.Errorf("entry %d server: want %q, got %q", i, expected.Server, got.Server)
				}
				if got.Direction != expected.Direction {
					t.Errorf("entry %d direction: want %q, got %q", i, expected.Direction, got.Direction)
				}
				wantMsg, err := json.Marshal(expected.Message)
				if err != nil {
					t.Fatalf("marshal expected message: %v", err)
				}
				gotMsg, err := json.Marshal(got.Message)
				if err != nil {
					t.Fatalf("marshal got message: %v", err)
				}
				if string(wantMsg) != string(gotMsg) {
					t.Errorf("entry %d message: want %s, got %s", i, wantMsg, gotMsg)
				}
			}

			if err := store.Close(); err != nil {
				t.Fatalf("close: %v", err)
			}

			// Edge case: Log after Close should error
			if err := store.Log(ctx, tt.entries[0]); err == nil {
				t.Fatal("expected error logging after close, got nil")
			}
		})
	}
}

func TestMultiLogger(t *testing.T) {
	t.Parallel()

	t.Run("logs to all backends", func(t *testing.T) {
		t.Parallel()
		m1 := &mockLogger{}
		m2 := &mockLogger{}
		ml := NewMultiLogger(m1, m2)

		ctx := context.Background()
		entry := LogEntry{Server: "test", Direction: "request"}
		if err := ml.Log(ctx, entry); err != nil {
			t.Fatalf("log: %v", err)
		}
		if len(m1.entries) != 1 {
			t.Errorf("m1 entries: want 1, got %d", len(m1.entries))
		}
		if len(m2.entries) != 1 {
			t.Errorf("m2 entries: want 1, got %d", len(m2.entries))
		}
	})

	t.Run("collects all log errors", func(t *testing.T) {
		t.Parallel()
		err1 := errors.New("log error 1")
		err2 := errors.New("log error 2")
		m1 := &mockLogger{logErr: err1}
		m2 := &mockLogger{logErr: err2}
		ml := NewMultiLogger(m1, m2)

		ctx := context.Background()
		entry := LogEntry{Server: "test", Direction: "request"}
		err := ml.Log(ctx, entry)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, err1) {
			t.Errorf("expected error to contain err1")
		}
		if !errors.Is(err, err2) {
			t.Errorf("expected error to contain err2")
		}
	})

	t.Run("collects all close errors", func(t *testing.T) {
		t.Parallel()
		err1 := errors.New("close error 1")
		err2 := errors.New("close error 2")
		m1 := &mockLogger{closeErr: err1}
		m2 := &mockLogger{closeErr: err2}
		ml := NewMultiLogger(m1, m2)

		err := ml.Close()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, err1) {
			t.Errorf("expected error to contain err1")
		}
		if !errors.Is(err, err2) {
			t.Errorf("expected error to contain err2")
		}
	})
}
