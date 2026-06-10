// Package config handles TOML configuration parsing.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// ErrInvalidConfig is returned when the configuration is invalid.
var ErrInvalidConfig = errors.New("invalid configuration")

// Config is the top-level configuration.
type Config struct {
	Servers map[string]ServerConfig `koanf:"server"`
	Guard   GuardConfig             `koanf:"guard"`
	API     APIConfig               `koanf:"api"`
}

// ServerConfig defines a single MCP server.
type ServerConfig struct {
	Command     string            `koanf:"command"`
	Args        []string          `koanf:"args"`
	Env         map[string]string `koanf:"env"`
	Timeout     TimeoutConfig     `koanf:"timeout"`
	Restart     RestartConfig     `koanf:"restart"`
	Permissions PermissionsConfig `koanf:"permissions"`
}

// TimeoutConfig holds request timeouts.
type TimeoutConfig struct {
	ToolsCall time.Duration `koanf:"tools_call"`
	ToolsList time.Duration `koanf:"tools_list"`
}

// RestartConfig holds restart policy.
type RestartConfig struct {
	MaxAttempts int    `koanf:"max_attempts"`
	Backoff     string `koanf:"backoff"`
}

// PermissionsConfig holds tool allow/deny lists.
type PermissionsConfig struct {
	Allow []string `koanf:"allow"`
	Deny  []string `koanf:"deny"`
}

// GuardConfig holds global guard settings.
type GuardConfig struct {
	HealthCheckInterval time.Duration `koanf:"health_check_interval"`
	AuditLogPath        string        `koanf:"audit_log_path"`
	SchemaCacheTTL      time.Duration `koanf:"schema_cache_ttl"`
	MaxConcurrentCalls  int           `koanf:"max_concurrent_calls"`
}

// APIConfig holds HTTP API settings.
type APIConfig struct {
	Enabled bool   `koanf:"enabled"`
	Addr    string `koanf:"addr"`
}

// Load reads a TOML config file and returns a parsed Config.
func Load(path string) (*Config, error) {
	k := koanf.New(".")
	if err := k.Load(file.Provider(path), toml.Parser()); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	var cfg Config
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "koanf"}); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	for name, sc := range cfg.Servers {
		for k, v := range sc.Env {
			sc.Env[k] = os.ExpandEnv(v)
		}
		cfg.Servers[name] = sc
	}

	if cfg.Guard.AuditLogPath != "" {
		expanded, err := expandHome(cfg.Guard.AuditLogPath)
		if err != nil {
			return nil, err
		}
		cfg.Guard.AuditLogPath = expanded
	}

	if err := ValidateAndSetDefaults(&cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

func expandHome(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand home: %w", err)
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}
