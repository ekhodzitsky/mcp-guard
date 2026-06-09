# ADR 001: Stdio Transport for MVP

## Status

Accepted

## Context

MCP supports stdio and Streamable HTTP. We need to choose the transport for MVP.

## Decision

Use stdio transport for MVP because:
1. Most MCP servers today are stdio-based
2. Simpler process management model
3. HTTP bridge can be added in v3

## Consequences

- Simpler initial implementation
- Need to handle newline-delimited JSON carefully
- Future work: add Streamable HTTP bridge
