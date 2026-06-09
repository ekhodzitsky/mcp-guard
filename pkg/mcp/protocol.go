package mcp

import "errors"

// JSONRPCVersion is the JSON-RPC protocol version.
const JSONRPCVersion = "2.0"

// MCP JSON-RPC method constants.
const (
	MethodInitialize       = "initialize"
	MethodInitialized      = "notifications/initialized"
	MethodToolsList        = "tools/list"
	MethodToolsCall        = "tools/call"
	MethodPing             = "ping"
	MethodToolsListChanged = "notifications/tools/list_changed"
	MethodProgress         = "notifications/progress"
	MethodCancelled        = "notifications/cancelled"
)

// Sentinel errors.
var (
	ErrTimeout       = errors.New("request timed out")
	ErrProcessDead   = errors.New("server process is not running")
	ErrInvalidConfig = errors.New("invalid configuration")
)
