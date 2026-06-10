package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/ekhodzitsky/mcp-guard/internal/api"
	"github.com/ekhodzitsky/mcp-guard/internal/audit"
	"github.com/ekhodzitsky/mcp-guard/internal/cache"
	"github.com/ekhodzitsky/mcp-guard/internal/config"
	"github.com/ekhodzitsky/mcp-guard/internal/events"
	"github.com/ekhodzitsky/mcp-guard/internal/guard"
	"github.com/ekhodzitsky/mcp-guard/internal/proxy"
	"github.com/ekhodzitsky/mcp-guard/internal/server"
	"github.com/ekhodzitsky/mcp-guard/internal/telemetry"
)

var (
	configPath string
	httpAddr   string
	rootCmd    *cobra.Command

	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func init() {
	rootCmd = &cobra.Command{
		Use:     "mcp-guard",
		Short:   "MCP Process Manager & Proxy",
		Long:    "mcp-guard manages MCP server processes, enforces timeouts, and logs all JSON-RPC traffic.",
		RunE:    run,
		Version: version,
	}
	rootCmd.SetVersionTemplate("mcp-guard version {{.Version}} (commit: " + commit + ", built: " + date + ")\n")
	rootCmd.Flags().StringVarP(&configPath, "config", "c", "mcp-guard.toml", "path to config file")
	rootCmd.Flags().StringVar(&httpAddr, "http-addr", "", "HTTP listen address (e.g., :8080)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		slog.Error("execute", "error", err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, _ []string) error {
	return runWithConfig(configPath)
}

func runWithConfig(configPath string) error {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// procCtx controls process lifecycle independently from proxy shutdown.
	procCtx, procCancel := context.WithCancel(context.Background())
	defer procCancel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shutdown, err := telemetry.Init(ctx, "mcp-guard")
	if err != nil {
		return fmt.Errorf("init telemetry: %w", err)
	}
	defer func() { _ = shutdown(ctx) }()

	bus := events.NewBus()
	defer bus.Close()

	// Setup signal handling before starting the pool to avoid races.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-sigCh
		if sig != nil {
			slog.Info("shutdown signal received", "signal", sig)
			cancel()
		}
	}()

	pool := server.NewPool(cfg.Servers, bus, cfg.Guard.HealthCheckInterval)
	if err := pool.Start(procCtx); err != nil {
		return fmt.Errorf("start pool: %w", err)
	}

	// Setup audit logging.
	var auditLogger audit.Logger
	var sqliteStore *audit.SQLiteStore
	if cfg.Guard.AuditLogPath != "" {
		jsonl, err := audit.NewJSONLinesLogger(cfg.Guard.AuditLogPath + ".jsonl")
		if err != nil {
			return fmt.Errorf("audit jsonl: %w", err)
		}
		sqliteStore, err = audit.NewSQLiteStore(cfg.Guard.AuditLogPath + ".db")
		if err != nil {
			_ = jsonl.Close()
			return fmt.Errorf("audit sqlite: %w", err)
		}
		auditLogger = audit.NewMultiLogger(jsonl, sqliteStore)
	} else {
		auditLogger = &audit.NoopLogger{}
	}
	defer func() { _ = auditLogger.Close() }()

	maxCalls := make(map[string]int)
	for name := range cfg.Servers {
		maxCalls[name] = cfg.Guard.MaxConcurrentCalls
	}

	permissions := make(map[string]*guard.PermissionChecker)
	rateLimiters := make(map[string]*guard.RateLimiter)
	var schemaCache *cache.SchemaCache
	if cfg.Guard.SchemaCacheTTL > 0 {
		schemaCache = cache.NewSchemaCache(cfg.Guard.SchemaCacheTTL)
	}

	for name, sc := range cfg.Servers {
		if len(sc.Permissions.Allow) > 0 || len(sc.Permissions.Deny) > 0 {
			permissions[name] = guard.NewPermissionChecker(sc.Permissions)
		}
		// Rate limits would come from config; for now, skip if not configured.
	}

	p := proxy.NewProxy(pool, auditLogger, maxCalls, permissions, rateLimiters, schemaCache)

	// Determine default server deterministically.
	var defaultServer string
	names := make([]string, 0, len(cfg.Servers))
	for name := range cfg.Servers {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) > 0 {
		defaultServer = names[0]
	}

	if httpAddr != "" {
		httpTransport := api.NewHTTPTransport(p, defaultServer)
		srv := &http.Server{Addr: httpAddr, Handler: httpTransport, ReadHeaderTimeout: 10 * time.Second}
		go func() {
			slog.Info("http transport listening", "addr", httpAddr)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("http transport", "error", err)
			}
		}()
		go func() {
			<-ctx.Done()
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			if err := srv.Shutdown(shutdownCtx); err != nil {
				slog.Error("http transport shutdown", "error", err)
			}
		}()
	}

	if cfg.API.Enabled {
		apiServer := api.NewServer(cfg.API.Addr, pool, sqliteStore, bus)
		go func() {
			if err := apiServer.Run(ctx); err != nil {
				slog.Error("api server", "error", err)
			}
		}()
	}

	if err := p.Run(ctx, os.Stdin, os.Stdout, defaultServer); err != nil {
		if errors.Is(err, context.Canceled) {
			slog.Info("shutting down")
		} else {
			slog.Error("proxy run", "error", err)
		}
	}

	signal.Stop(sigCh)
	close(sigCh)

	// Graceful shutdown of pool.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := pool.Stop(shutdownCtx); err != nil {
		slog.Error("pool stop", "error", err)
	}

	return nil
}
