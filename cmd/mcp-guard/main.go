package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/ekhodzitsky/mcp-guard/internal/audit"
	"github.com/ekhodzitsky/mcp-guard/internal/config"
	"github.com/ekhodzitsky/mcp-guard/internal/events"
	"github.com/ekhodzitsky/mcp-guard/internal/proxy"
	"github.com/ekhodzitsky/mcp-guard/internal/server"
)

var (
	configPath string
	rootCmd    *cobra.Command
)

func init() {
	rootCmd = &cobra.Command{
		Use:   "mcp-guard",
		Short: "MCP Process Manager & Proxy",
		Long:  "mcp-guard manages MCP server processes, enforces timeouts, and logs all JSON-RPC traffic.",
		RunE:  run,
	}
	rootCmd.Flags().StringVarP(&configPath, "config", "c", "mcp-guard.toml", "path to config file")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		slog.Error("execute", "error", err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, _ []string) error {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := config.ValidateAndSetDefaults(cfg); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := events.NewBus()

	pool := server.NewPool(cfg.Servers, bus)
	if err := pool.Start(ctx); err != nil {
		return fmt.Errorf("start pool: %w", err)
	}

	// Setup audit logging.
	var auditLogger audit.Logger
	if cfg.Guard.AuditLogPath != "" {
		jsonl, err := audit.NewJSONLinesLogger(cfg.Guard.AuditLogPath + ".jsonl")
		if err != nil {
			return fmt.Errorf("audit jsonl: %w", err)
		}
		sqlite, err := audit.NewSQLiteStore(cfg.Guard.AuditLogPath + ".db")
		if err != nil {
			return fmt.Errorf("audit sqlite: %w", err)
		}
		auditLogger = audit.NewMultiLogger(jsonl, sqlite)
	} else {
		auditLogger = &audit.NoopLogger{}
	}
	defer auditLogger.Close()

	maxCalls := make(map[string]int)
	for name := range cfg.Servers {
		maxCalls[name] = cfg.Guard.MaxConcurrentCalls
	}
	p := proxy.NewProxy(pool, auditLogger, maxCalls)

	// Handle graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-sigCh
		slog.Info("shutdown signal received")
		cancel()
	}()

	// Determine default server.
	var defaultServer string
	for name := range cfg.Servers {
		defaultServer = name
		break
	}

	if err := p.Run(ctx, os.Stdin, os.Stdout, defaultServer); err != nil {
		if err == context.Canceled {
			slog.Info("shutting down")
		} else {
			slog.Error("proxy run", "error", err)
		}
	}

	// Graceful shutdown of pool.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := pool.Stop(shutdownCtx); err != nil {
		slog.Error("pool stop", "error", err)
	}

	return nil
}
