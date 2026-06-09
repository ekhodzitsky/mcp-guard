package config

import (
	"fmt"
	"strings"
	"time"
)

// ValidateAndSetDefaults checks the configuration for errors and applies defaults.
func ValidateAndSetDefaults(cfg *Config) error {
	if len(cfg.Servers) == 0 {
		return fmt.Errorf("no servers configured")
	}

	validBackoffs := map[string]bool{"exponential": true, "linear": true, "fixed": true}
	for name, sc := range cfg.Servers {
		if strings.TrimSpace(sc.Command) == "" {
			return fmt.Errorf("server %q: command is required", name)
		}
		if sc.Timeout.ToolsCall <= 0 {
			sc.Timeout.ToolsCall = 30 * time.Second
		}
		if sc.Timeout.ToolsList <= 0 {
			sc.Timeout.ToolsList = 10 * time.Second
		}
		if sc.Restart.MaxAttempts <= 0 {
			sc.Restart.MaxAttempts = 5
		}
		if sc.Restart.Backoff == "" {
			sc.Restart.Backoff = "exponential"
		}
		if !validBackoffs[sc.Restart.Backoff] {
			return fmt.Errorf("server %q: invalid backoff %q, must be one of: exponential, linear, fixed", name, sc.Restart.Backoff)
		}
		cfg.Servers[name] = sc
	}

	if cfg.Guard.HealthCheckInterval <= 0 {
		cfg.Guard.HealthCheckInterval = 5 * time.Second
	}
	if cfg.Guard.MaxConcurrentCalls <= 0 {
		cfg.Guard.MaxConcurrentCalls = 100
	}

	return nil
}
