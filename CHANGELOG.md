# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-06-10

### Added
- Process pool management with graceful shutdown and auto-restart (exponential backoff).
- Health checks: JSON-RPC ping every 5s, auto-restart after 3 consecutive failures.
- JSON-RPC stdio proxy with async response routing.
- Hard timeouts: 30s for `tools/call`, 10s for `tools/list`.
- Dual audit logging: JSON Lines + SQLite (WAL mode, indexed).
- Tool permission system: per-server allow/deny lists.
- Per-tool rate limiting: sliding-window RPM / RPD.
- In-memory schema cache for `tools/list` with TTL and `list_changed` invalidation.
- HTTP API & Web UI (HTMX + SSE) on `localhost:8787`.
- OpenTelemetry tracing support.
- TOML configuration with environment variable expansion.
