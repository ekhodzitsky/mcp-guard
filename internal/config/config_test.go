package config

import (
	"testing"
	"time"
)

func TestLoadAndValidate(t *testing.T) {
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

	if err := Validate(cfg); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidateEmptyCommand(t *testing.T) {
	cfg := &Config{
		Servers: map[string]ServerConfig{
			"bad": {Command: ""},
		},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for empty command")
	}
}
