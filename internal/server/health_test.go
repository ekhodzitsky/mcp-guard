package server

import (
	"context"
	"testing"
	"time"

	"github.com/ekhodzitsky/mcp-guard/internal/config"
	"github.com/ekhodzitsky/mcp-guard/internal/events"
)

func TestHealthChecker(t *testing.T) {
	bus := events.NewBus()
	cfg := config.ServerConfig{Command: "cat"}
	p := NewProcess("test", cfg, bus)
	checker := NewHealthChecker(p, bus, 100*time.Millisecond, 2)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	checker.Start(ctx)
	<-ctx.Done()
	checker.Stop()
}
