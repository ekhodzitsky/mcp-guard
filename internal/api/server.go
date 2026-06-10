// Package api provides the HTTP API and web UI.
package api

import (
	"context"
	"embed"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/ekhodzitsky/mcp-guard/internal/audit"
	"github.com/ekhodzitsky/mcp-guard/internal/events"
	"github.com/ekhodzitsky/mcp-guard/internal/server"
)

//go:embed templates/*.html
var templatesFS embed.FS

// Server is the HTTP API server.
type Server struct {
	router *chi.Mux
	addr   string
	h      *handlerSet
}

type handlerSet struct {
	pool       *server.Pool
	auditStore *audit.SQLiteStore
	bus        *events.Bus
}

// NewServer creates an HTTP API server.
func NewServer(addr string, pool *server.Pool, auditStore *audit.SQLiteStore, bus *events.Bus) *Server {
	s := &Server{addr: addr}
	s.h = &handlerSet{pool: pool, auditStore: auditStore, bus: bus}
	s.router = chi.NewRouter()
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.router.Get("/", s.h.handleIndex)
	s.router.Get("/servers", s.h.handleServers)
	s.router.Post("/servers/{name}/restart", s.h.handleRestart)
	s.router.Get("/events", s.h.handleSSE)
	s.router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

// Run starts the HTTP server and blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{Addr: s.addr, Handler: s.router}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	slog.Info("api server listening", "addr", s.addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
