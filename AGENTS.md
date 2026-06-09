# mcp-guard Agent Guide

## Project Structure

- `cmd/mcp-guard/` — CLI entry point (Cobra)
- `pkg/mcp/` — Public MCP types and protocol constants
- `internal/config/` — TOML config parsing (koanf)
- `internal/events/` — Pub/sub event bus
- `internal/server/` — Process lifecycle, health checks, pool management
- `internal/proxy/` — JSON-RPC stdio proxy with timeout enforcement
- `internal/audit/` — JSON Lines + SQLite audit logging

## Key Types

- `server.Pool` — struct that manages `Process` instances
- `server.Process` — struct representing a single MCP server process
- `proxy.Proxy` — struct that forwards JSON-RPC requests with timeout and audit
- `events.Bus` — pub/sub event bus for internal communication
- `audit.Logger` — logs all MCP traffic
- `HealthChecker` — monitors process health (verifies the process is running and stdin is writable; does not read responses) and triggers restarts

## Testing

- Unit tests: table-driven, mock via interfaces
- Integration tests: `tests/integration/` with real processes (`cat`)
- Always run with `-race`

## Build

```bash
make race
make lint
make build
```
