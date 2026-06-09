# mcp-guard Design Specification

## Overview

mcp-guard is a production-ready open-source MCP (Model Context Protocol) Process Manager & Proxy written in Go. It solves critical operational problems that AI CLI tools face when running MCP servers directly: stdio hangs, orphaned processes, lack of observability, and security gaps.

## Goals

1. Provide robust process pool management for MCP servers with auto-restart and health checking
2. Enforce hard timeouts on MCP operations
3. Offer comprehensive audit logging (JSON Lines + SQLite)
4. Support graceful shutdown without orphaned child processes
5. Serve as a portfolio-quality codebase demonstrating idiomatic Go, clean architecture, and thorough testing

## Non-Goals (v3 / Out of Scope for MVP+v2)

- OpenTelemetry tracing
- Sandboxing implementation (only recommendations/hints)
- stdio ↔ Streamable HTTP bridge

## Architecture

### Package Layout

```
cmd/mcp-guard/          # entry point (Cobra CLI)
internal/
  config/               # TOML config parsing + validation (koanf)
  server/               # MCP server process lifecycle
    pool.go             # pool of server processes
    process.go          # single process lifecycle
    health.go           # health checker with exponential backoff
  proxy/                # JSON-RPC proxy / stdio bridge
    proxy.go            # main proxy logic
    timeout.go          # timeout wrapper for requests
  audit/                # audit logging
    logger.go           # JSON Lines logger
    sqlite.go           # SQLite audit store
  cache/                # schema cache
    cache.go            # in-memory TTL cache (xsync)
  guard/                # permissions + rate limiting
    rate_limiter.go
    permissions.go
  api/                  # HTTP API for Web UI (Chi)
    handlers.go
    sse.go              # SSE streaming for live events
  events/               # internal pub/sub event bus
    bus.go
pkg/
  mcp/                  # public MCP types and constants
    types.go            # JSON-RPC types
    protocol.go         # protocol constants
tests/
  integration/          # integration tests with real MCP servers
docs/
  adr/                  # Architecture Decision Records
```

### Key Interfaces

- `ServerPool` — manages a collection of `ServerProcess` instances
- `ServerProcess` — abstraction over a single MCP server lifecycle
- `Proxy` — bridges client stdio with backend server stdio
- `AuditLogger` — writes JSON-RPC messages to JSON Lines and SQLite
- `RateLimiter` — per-tool RPM/RPD limits
- `PermissionGuard` — whitelist/blacklist per server
- `EventBus` — pub/sub for internal server events

### Data Flow

```
Client stdin ──► [mcp-guard proxy] ──► Server A stdin
Client stdout ◄── [mcp-guard proxy] ◄── Server A stdout
                      │
                 [Audit Log] ──► JSON Lines + SQLite
                 [Timeout Check]
                 [Rate Limit]
                 [Permission Guard]
```

## Core Algorithms

### Process Lifecycle

```
Start → Health Check (ping every 5s) → Healthy
            ↓ (3 consecutive failures)
    Restart with exponential backoff (1s → 2s → 4s → 8s...)
            ↓ (max attempts exceeded)
    Failed (permanent, no more restarts)
```

### Graceful Shutdown

```
SIGTERM received
→ Stop accepting new client requests
→ Cancel all in-flight request contexts
→ Send SIGTERM to all child processes
→ Wait 5s
→ Send SIGKILL to remaining children
→ Flush audit logs to disk
→ Exit
```

### JSON-RPC Proxy

Each line from client stdin is:
1. Parsed as JSON-RPC 2.0 request
2. Checked against permissions (tool whitelist/blacklist)
3. Checked against rate limits
4. Forwarded to appropriate backend server
5. Response read from server stdout
6. Timeout enforced (30s tools/call, 10s tools/list)
7. Response written to client stdout
8. Both request and response logged to audit

## Configuration (TOML)

```toml
[server.filesystem]
command = "npx"
args = ["-y", "@modelcontextprotocol/server-filesystem", "/Users/you/projects"]
timeout = { tools_call = "30s", tools_list = "10s" }
restart = { max_attempts = 5, backoff = "exponential" }
permissions = { allow = ["read_file", "list_directory"], deny = ["write_file"] }

[server.github]
command = "npx"
args = ["-y", "@modelcontextprotocol/server-github"]
env = { GITHUB_PERSONAL_ACCESS_TOKEN = "$GITHUB_TOKEN" }

[guard]
health_check_interval = "5s"
audit_log_path = "~/.mcp-guard/audit"
schema_cache_ttl = "5m"
max_concurrent_calls = 100

[api]
enabled = true
addr = "localhost:8787"
```

## Technology Stack

- **Go 1.23+**
- **CLI:** github.com/spf13/cobra
- **Config:** github.com/knadh/koanf/v2 (TOML)
- **SQLite:** github.com/mattn/go-sqlite3
- **HTTP Router:** github.com/go-chi/chi/v5
- **Lock-free structures:** github.com/puzpuzpuz/xsync/v3
- **Logging:** log/slog (structured)
- **JSON-RPC:** net/http + bufio.Scanner (newline-delimited JSON)

## Quality Standards

1. **Idiomatic Go:** interfaces for abstractions, context propagation, error wrapping (`%w`), sentinel errors
2. **Clean Architecture:** `internal/` for private, `pkg/` for public, dependency injection via constructors, zero global state
3. **Testing:** table-driven unit tests, mocking via interfaces, integration tests with `@modelcontextprotocol/server-echo`, race detector, 80%+ coverage target
4. **Documentation:** README, AGENTS.md, Go doc comments for all exported types, ADRs in docs/adr/
5. **CI/CD:** GitHub Actions (test, lint, build, release), goreleaser, dependabot
6. **Observability:** structured slog, health endpoint (/health)

## Phases

### Phase 1: MVP
- Process pool management (start, monitor, graceful restart)
- Health check with auto-restart and exponential backoff
- Hard timeouts on tools/call and tools/list
- Audit log (JSON Lines + SQLite)
- Stdio proxy
- Graceful shutdown

### Phase 2: v2 Features
- Schema cache with TTL and list_changed invalidation
- Rate limiting (per-tool RPM/RPD)
- Web UI with HTMX (status, live audit log, manual restart)
- Config validation at startup
- Tool permissions (whitelist/blacklist)

### Phase 3: v3 Ideas (Future)
- stdio ↔ Streamable HTTP bridge
- OpenTelemetry tracing
- Sandboxing hints (firejail/bubblewrap)
