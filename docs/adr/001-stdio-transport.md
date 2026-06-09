# ADR 001: Stdio Transport for MVP

**Date:** 2026-06-09

## Status

Accepted

## Context

MCP supports stdio and Streamable HTTP. We need to choose the transport for MVP.

## Decision

Use stdio transport for MVP because:
1. Most MCP servers today are stdio-based
2. Simpler process management model
3. HTTP bridge can be added later

## Consequences

- Simpler initial implementation and faster time-to-market
- Need to handle newline-delimited JSON carefully to avoid framing errors
- Process lifecycle management (start/stop/restart) is fully under our control
- Future work: add Streamable HTTP bridge
- Future work: expose configuration via `APIConfig` for programmatic use
