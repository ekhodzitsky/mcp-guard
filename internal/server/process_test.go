package server

import (
	"context"
	"testing"
	"time"

	"github.com/ekhodzitsky/mcp-guard/internal/config"
)

func TestProcess(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{"happy path", testProcessHappyPath},
		{"double start errors", testProcessDoubleStart},
		{"stop not running", testProcessStopNotRunning},
		{"sigkill after timeout", testProcessSigkillTimeout},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}

func testProcessHappyPath(t *testing.T) {
	cfg := config.ServerConfig{
		Command: "cat",
	}
	p := NewProcess("test", cfg, nil)
	ctx := context.Background()

	if err := p.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if !p.Running() {
		t.Fatal("expected process to be running")
	}

	stopCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = p.Stop(stopCtx) // process exits via SIGTERM; cmd.Wait returns signal error

	if p.Running() {
		t.Fatal("expected process to be stopped")
	}
}

func testProcessDoubleStart(t *testing.T) {
	cfg := config.ServerConfig{
		Command: "cat",
	}
	p := NewProcess("test", cfg, nil)
	ctx := context.Background()

	if err := p.Start(ctx); err != nil {
		t.Fatalf("first start: %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = p.Stop(stopCtx)
	}()

	if err := p.Start(ctx); err == nil {
		t.Fatal("expected second start to error")
	}
}

func testProcessStopNotRunning(t *testing.T) {
	p := NewProcess("test", config.ServerConfig{}, nil)
	ctx := context.Background()

	if err := p.Stop(ctx); err != nil {
		t.Fatalf("stop not-running process: %v", err)
	}
}

func testProcessSigkillTimeout(t *testing.T) {
	cfg := config.ServerConfig{
		Command: "sleep",
		Args:    []string{"30"},
	}
	p := NewProcess("test", cfg, nil)
	ctx := context.Background()

	if err := p.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if !p.Running() {
		t.Fatal("expected process to be running")
	}

	// Use an already-cancelled context to force the SIGKILL path.
	stopCtx, cancel := context.WithCancel(ctx)
	cancel()

	_ = p.Stop(stopCtx)

	if p.Running() {
		t.Fatal("expected process to be killed")
	}
}
