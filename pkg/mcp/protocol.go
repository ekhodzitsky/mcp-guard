package mcp

import "errors"

// MCP JSON-RPC method constants.
const (
	MethodInitialize  = "initialize"
	MethodInitialized = "notifications/initialized"
	MethodToolsList   = "tools/list"
	MethodToolsCall   = "tools/call"
	MethodPing        = "ping"
	MethodListChanged = "notifications/tools/list_changed"
	MethodProgress    = "notifications/progress"
	MethodCancelled   = "notifications/cancelled"
)

// Sentinel errors.
var (
	ErrTimeout       = errors.New("request timed out")
	ErrProcessDead   = errors.New("server process is not running")
	ErrInvalidConfig = errors.New("invalid configuration")
)
