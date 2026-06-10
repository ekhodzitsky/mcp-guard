package mcp

import "fmt"

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

// RequestID represents a JSON-RPC request identifier.
type RequestID struct {
	Value any
}

// String returns the string representation of the request ID.
func (r RequestID) String() string {
	if r.Value == nil {
		return "null"
	}
	return fmt.Sprint(r.Value)
}

// Equal reports whether two request IDs are equal.
func (r RequestID) Equal(other RequestID) bool {
	if r.Value == nil && other.Value == nil {
		return true
	}
	if r.Value == nil || other.Value == nil {
		return false
	}
	return r.String() == other.String()
}
