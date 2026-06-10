package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestValidateAndSetDefaults(t *testing.T) {
	cfg := &Config{
		Servers: map[string]ServerConfig{
			"echo": {
				Command: "echo",
				Args:    []string{"hello"},
				Timeout: TimeoutConfig{ToolsCall: 30 * time.Second, ToolsList: 10 * time.Second},
				Restart: RestartConfig{MaxAttempts: 5, Backoff: "exponential"},
			},
		},
		Guard: GuardConfig{
			HealthCheckInterval: 5 * time.Second,
			MaxConcurrentCalls:  100,
		},
	}

	if err := ValidateAndSetDefaults(cfg); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidateEmptyCommand(t *testing.T) {
	cfg := &Config{
		Servers: map[string]ServerConfig{
			"bad": {Command: ""},
		},
	}
	if err := ValidateAndSetDefaults(cfg); err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestLoad(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	content := `
[server.echo]
command = "echo"
args = ["hello"]
env = { FOO = "bar", HOME_VAR = "$HOME" }

[server.echo.timeout]
tools_call = "30s"
tools_list = "10s"

[server.echo.restart]
max_attempts = 5
backoff = "exponential"

[guard]
health_check_interval = "5s"
max_concurrent_calls = 100
audit_log_path = "~/mcp-guard-test-audit"
`
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if len(cfg.Servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(cfg.Servers))
	}

	echo, ok := cfg.Servers["echo"]
	if !ok {
		t.Fatal("expected server 'echo'")
	}

	if echo.Command != "echo" {
		t.Errorf("expected command 'echo', got %q", echo.Command)
	}
	if len(echo.Args) != 1 || echo.Args[0] != "hello" {
		t.Errorf("expected args ['hello'], got %v", echo.Args)
	}
	if echo.Env["FOO"] != "bar" {
		t.Errorf("expected env FOO='bar', got %q", echo.Env["FOO"])
	}

	// Test env expansion in server env map
	if home := os.Getenv("HOME"); home != "" {
		if echo.Env["HOME_VAR"] != home {
			t.Errorf("expected env HOME_VAR='%s', got %q", home, echo.Env["HOME_VAR"])
		}
	}

	if echo.Timeout.ToolsCall != 30*time.Second {
		t.Errorf("expected tools_call timeout 30s, got %v", echo.Timeout.ToolsCall)
	}
	if echo.Timeout.ToolsList != 10*time.Second {
		t.Errorf("expected tools_list timeout 10s, got %v", echo.Timeout.ToolsList)
	}
	if echo.Restart.MaxAttempts != 5 {
		t.Errorf("expected max_attempts 5, got %d", echo.Restart.MaxAttempts)
	}
	if echo.Restart.Backoff != "exponential" {
		t.Errorf("expected backoff 'exponential', got %q", echo.Restart.Backoff)
	}

	if cfg.Guard.HealthCheckInterval != 5*time.Second {
		t.Errorf("expected health_check_interval 5s, got %v", cfg.Guard.HealthCheckInterval)
	}
	if cfg.Guard.MaxConcurrentCalls != 100 {
		t.Errorf("expected max_concurrent_calls 100, got %d", cfg.Guard.MaxConcurrentCalls)
	}

	// Test ~ expansion in audit_log_path
	if home := os.Getenv("HOME"); home != "" {
		expectedPath := filepath.Join(home, "mcp-guard-test-audit")
		if cfg.Guard.AuditLogPath != expectedPath {
			t.Errorf("expected audit_log_path %q, got %q", expectedPath, cfg.Guard.AuditLogPath)
		}
	}
}

func TestLoadNonexistentFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.toml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadInvalidTOML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(configPath, []byte("not valid toml {{"), 0600); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}
	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}

func TestLoadEmptyCommand(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")
	content := `
[server.bad]
command = ""
`
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestValidateAndSetDefaultsInvalidBackoff(t *testing.T) {
	cfg := &Config{
		Servers: map[string]ServerConfig{
			"bad": {
				Command: "echo",
				Restart: RestartConfig{MaxAttempts: 1, Backoff: "invalid"},
			},
		},
	}
	if err := ValidateAndSetDefaults(cfg); err == nil {
		t.Fatal("expected error for invalid backoff")
	}
}
