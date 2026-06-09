# mcp-guard

MCP Process Manager & Proxy — production-ready guardian for MCP servers.

## Features

- **Process Pool Management**: Start, monitor, and gracefully restart MCP servers
- **Health Checks**: Ping every 5s, auto-restart after 3 failures with exponential backoff
- **Hard Timeouts**: 30s for tools/call, 10s for tools/list
- **Audit Logging**: JSON Lines + SQLite for all JSON-RPC traffic
- **Graceful Shutdown**: SIGTERM → child SIGTERM → SIGKILL after 5s

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
timeout = { tools_call = "30s", tools_list = "10s" }
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

```bash
make test   # Run tests with race detector
make lint   # Run golangci-lint
make build  # Build binary
make clean  # Remove binary
```
