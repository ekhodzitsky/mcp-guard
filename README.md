# MCP Guard 🛡️

[![Go Version](https://img.shields.io/badge/go-1.26+-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Build](https://img.shields.io/badge/build-passing-brightgreen)](Makefile)
[![MCP](https://img.shields.io/badge/MCP-Protocol-orange)](https://modelcontextprotocol.io)

> **Production-ready guardian for MCP servers.**
>
> Process management, hard timeouts, audit logging, rate limiting, and tool permissions — all in one lightweight stdio proxy.

---

## Why MCP Guard?

Running MCP servers in production is fragile:

- **Processes crash** and never come back.
- **Tool calls hang forever** with no way to cancel them.
- **Any tool can be invoked** with no access control.
- **No audit trail** of what the AI actually did.
- **Schema requests flood** the server on every context window.

**MCP Guard fixes all of that.** It sits between your AI client and MCP servers, adding reliability, security, and observability without changing a single line of server code.

---

## Features

| Feature | Description |
|---------|-------------|
| 🖥️ **Process Pool** | Start, monitor, and auto-restart multiple MCP servers |
| 💓 **Health Checks** | Ping every 5s; auto-restart after 3 consecutive failures (~15s) |
| ⏱️ **Hard Timeouts** | 30s for `tools/call`, 10s for `tools/list` — no more hangs |
| 📝 **Audit Logging** | JSON Lines + SQLite dual logging of all JSON-RPC traffic |
| 🔒 **Tool Permissions** | Whitelist / blacklist tools per server (deny takes precedence) |
| 🚦 **Rate Limiting** | Per-tool RPM/RPD sliding-window limits |
| ⚡ **Schema Cache** | `tools/list` responses cached with TTL; auto-invalidated on `list_changed` |
| 🌐 **Web UI** | HTMX-based dashboard + SSE live events at `localhost:8787` |
| 📊 **OpenTelemetry** | Distributed tracing out of the box |

---

## Quick Start

### Install

```bash
go install github.com/ekhodzitsky/mcp-guard/cmd/mcp-guard@latest
```

Or download a pre-built binary from [Releases](https://github.com/ekhodzitsky/mcp-guard/releases).

### Configure

Create `mcp-guard.toml`:

```toml
[server.filesystem]
command = "npx"
args = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
timeout = { tools_call = "30s", tools_list = "10s" }
restart = { max_attempts = 5, backoff = "exponential" }
permissions = { allow = ["read_file", "list_directory"], deny = ["write_file"] }
rate_limit = { rpm = 60, rpd = 1000 }

[server.echo]
command = "npx"
args = ["-y", "@modelcontextprotocol/server-echo"]

[guard]
health_check_interval = "5s"
audit_log_path = "~/.mcp-guard/audit"
schema_cache_ttl = "5m"
max_concurrent_calls = 100

[api]
enabled = true
addr = "localhost:8787"
```

### Run

```bash
mcp-guard -c mcp-guard.toml
```

Then point your MCP client (Claude Desktop, Cline, etc.) at `mcp-guard` instead of the raw server command.

---

## Architecture

```
┌─────────────┐      stdin       ┌─────────────┐      stdin       ┌─────────────┐
│   MCP       │ ───────────────→ │  MCP Guard  │ ───────────────→ │   MCP       │
│   Client    │                  │   Proxy     │                  │   Server    │
│ (Claude,    │ ←─────────────── │  + Guard    │ ←─────────────── │  (Pool)     │
│  Cline…)    │     stdout       │             │     stdout       │             │
└─────────────┘                  └──────┬──────┘                  └─────────────┘
                                        │
                    ┌───────────────────┼───────────────────┐
                    ↓                   ↓                   ↓
              [Audit Log]      [Timeout Check]      [Permission / Rate Limit]
                    ↓
              [Schema Cache]
                    ↓
              [Web UI / API]
```

---

## Configuration Reference

| Section | Key | Default | Description |
|---------|-----|---------|-------------|
| `server.<name>` | `command` | — | Executable to run |
| `server.<name>` | `args` | `[]` | Arguments |
| `server.<name>` | `env` | `{}` | Extra environment variables |
| `server.<name>` | `timeout` | — | `tools_call`, `tools_list` durations |
| `server.<name>` | `restart` | — | `max_attempts`, `backoff` |
| `server.<name>` | `permissions` | — | `allow`, `deny` tool lists |
| `server.<name>` | `rate_limit` | — | `rpm`, `rpd` |
| `guard` | `health_check_interval` | `5s` | Health ping frequency |
| `guard` | `audit_log_path` | `""` | Path prefix for `.jsonl` + `.db` |
| `guard` | `schema_cache_ttl` | `0` | Cache TTL (disabled if 0) |
| `guard` | `max_concurrent_calls` | `100` | Semaphore per server |
| `api` | `enabled` | `false` | Enable HTTP dashboard |
| `api` | `addr` | `:8787` | Bind address |

---

## Roadmap

- [x] Process pool & health checks
- [x] Hard timeouts & audit logging
- [x] Tool permissions & rate limiting
- [x] Schema cache & Web UI
- [ ] mTLS / auth for HTTP API
- [ ] Prometheus metrics endpoint
- [ ] Plugin system for custom guards

---

## Development

Requires Go 1.26+.

```bash
make race   # tests with race detector
make lint   # golangci-lint
make build  # build binary
```

---

## License

MIT © [Eugene Khodzitsky](https://github.com/ekhodzitsky)
