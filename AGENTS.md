# mcp-guard Agent Guide

## Project Structure

- `cmd/mcp-guard/` — CLI entry point (Cobra)
- `pkg/mcp/` — Public MCP types and protocol constants
- `internal/config/` — TOML config parsing (koanf)
- `internal/events/` — Pub/sub event bus
- `internal/server/` — Process lifecycle, health checks, pool management
- `internal/proxy/` — JSON-RPC stdio proxy with timeout enforcement
- `internal/audit/` — JSON Lines + SQLite audit logging

## Key Interfaces

- `server.Pool` — manages `Process` instances
- `proxy.Proxy` — forwards JSON-RPC requests with timeout and audit
- `audit.Logger` — logs all MCP traffic

## Testing

- Unit tests: table-driven, mock via interfaces
- Integration tests: `tests/integration/` with real processes (`cat`)
- Always run with `-race`

## Build

```bash
make test
make lint
make build
```
