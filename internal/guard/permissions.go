// Package guard provides permission and rate-limiting guards.
package guard

import (
	"github.com/ekhodzitsky/mcp-guard/internal/config"
)

// PermissionChecker checks if a tool call is permitted.
type PermissionChecker struct {
	allow map[string]struct{}
	deny  map[string]struct{}
}

// NewPermissionChecker creates a checker from config.
func NewPermissionChecker(cfg config.PermissionsConfig) *PermissionChecker {
	allow := make(map[string]struct{}, len(cfg.Allow))
	for _, t := range cfg.Allow {
		allow[t] = struct{}{}
	}
	deny := make(map[string]struct{}, len(cfg.Deny))
	for _, t := range cfg.Deny {
		deny[t] = struct{}{}
	}
	return &PermissionChecker{allow: allow, deny: deny}
}

// IsAllowed reports whether the named tool is permitted.
// Deny list takes precedence. If allow list is non-empty,
// the tool must be in it.
func (c *PermissionChecker) IsAllowed(tool string) bool {
	if _, ok := c.deny[tool]; ok {
		return false
	}
	if len(c.allow) == 0 {
		return true
	}
	_, ok := c.allow[tool]
	return ok
}
