package server

import (
	"context"
	"testing"
	"time"

	"github.com/ekhodzitsky/mcp-guard/internal/config"
)

func TestProcessStartStop(t *testing.T) {
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
	if err := p.Stop(stopCtx); err != nil {
		t.Fatalf("stop: %v", err)
	}

	if p.Running() {
		t.Fatal("expected process to be stopped")
	}
}
