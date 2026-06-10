package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootCommandExists(t *testing.T) {
	if rootCmd == nil {
		t.Fatal("rootCmd should not be nil")
	}
}

func TestRunWithConfig_LoadFailure(t *testing.T) {
	err := runWithConfig("/tmp/this-config-does-not-exist-12345.toml")
	if err == nil {
		t.Fatal("expected error for missing config")
	}
	if !strings.Contains(err.Error(), "load config") {
		t.Fatalf("expected 'load config' error, got: %v", err)
	}
}

func TestRunWithConfig_PoolStartFailure(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	content := `
[server.test]
command = "this-command-definitely-does-not-exist-12345"
`
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	err := runWithConfig(configPath)
	if err == nil {
		t.Fatal("expected error for pool start failure")
	}
	if !strings.Contains(err.Error(), "start pool") {
		t.Fatalf("expected 'start pool' error, got: %v", err)
	}
}

func TestRunWithConfig_AuditInitFailure(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")
	auditPath := filepath.Join(tmpDir, "audit")

	// Pre-create the .db as a directory so NewSQLiteStore fails.
	if err := os.MkdirAll(auditPath+".db", 0750); err != nil {
		t.Fatalf("create audit db directory: %v", err)
	}

	content := `
[server.test]
command = "echo"
args = ["hello"]

[guard]
audit_log_path = "` + auditPath + `"
`
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	err := runWithConfig(configPath)
	if err == nil {
		t.Fatal("expected error for audit init failure")
	}
	if !strings.Contains(err.Error(), "audit sqlite") {
		t.Fatalf("expected 'audit sqlite' error, got: %v", err)
	}
}
