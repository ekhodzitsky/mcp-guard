# mcp-guard

MCP Process Manager & Proxy — production-ready guardian for MCP servers.

## Features

- **Process Pool Management**: Start, monitor, and gracefully restart MCP servers
- **Health Checks**: Health checks write a ping every 5s; auto-restart triggers after 3 consecutive failed health checks (~15s of unresponsiveness).
- **Hard Timeouts**: 30s for tools/call, 10s for tools/list
- **Audit Logging**: JSON Lines + SQLite for all JSON-RPC traffic
- **Graceful Shutdown**: SIGTERM → child SIGTERM → SIGKILL after 10s

## Installation

```bash
go install github.com/ekhodzitsky/mcp-guard/cmd/mcp-guard@latest
```

## Usage

Create a `mcp-guard.toml`:

```toml
[server.echo]
command = "npx"
args = ["-y", "@modelcontextprotocol/server-echo"]
restart = { max_attempts = 5, backoff = "exponential" }

[guard]
health_check_interval = "5s"
audit_log_path = "~/.mcp-guard/audit"
max_concurrent_calls = 100
```

Run:

```bash
mcp-guard -c mcp-guard.toml
```

## Architecture

```
Client stdin → mcp-guard proxy → Server stdin
Client stdout ← mcp-guard proxy ← Server stdout
                    ↓
              [Audit Log]
              [Timeout Check]
```

## Development

Requires Go 1.23+.

```bash
make test   # Run tests
make race   # Run tests with race detector
make lint   # Run golangci-lint
make build  # Build binary
make clean  # Remove binary
```
