package guard

import (
	"testing"

	"github.com/ekhodzitsky/mcp-guard/internal/config"
)

func TestPermissionChecker(t *testing.T) {
	cfg := config.PermissionsConfig{
		Allow: []string{"read_file"},
		Deny:  []string{"write_file"},
	}
	checker := NewPermissionChecker(cfg)

	if !checker.IsAllowed("read_file") {
		t.Fatal("expected read_file to be allowed")
	}
	if checker.IsAllowed("write_file") {
		t.Fatal("expected write_file to be denied")
	}
	if checker.IsAllowed("delete_file") {
		t.Fatal("expected delete_file to be denied when not in allow list")
	}
}

func TestPermissionCheckerEmptyAllow(t *testing.T) {
	cfg := config.PermissionsConfig{
		Deny: []string{"dangerous"},
	}
	checker := NewPermissionChecker(cfg)

	if !checker.IsAllowed("safe_tool") {
		t.Fatal("expected safe_tool to be allowed when allow is empty")
	}
	if checker.IsAllowed("dangerous") {
		t.Fatal("expected dangerous to be denied")
	}
}
