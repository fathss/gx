package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/fathss/gx/internal/cli"
	"github.com/fathss/gx/internal/git"
)

const configDir = ".gx"
const configFile = configDir + "/config"

// DefaultSensitivePatterns are the sensitive file patterns written to new
// configs by gx init. They live here, not in the git package, so the config
// file is the single source of truth.
var DefaultSensitivePatterns = []string{".env", "*.pem", "*secret*", "*.key"}

// DefaultProtectedBranches are the branch names protected from direct push,
// used as the default when none are configured.
var DefaultProtectedBranches = []string{"main", "master", "develop"}

type Config struct {
	Remote            string   `json:"remote"`
	DefaultBranch     string   `json:"defaultBranch"`
	SyncStrategy      string   `json:"syncStrategy"`
	SensitivePatterns  []string `json:"sensitivePatterns,omitempty"`
	ProtectedBranches []string `json:"protectedBranches,omitempty"`
}

func Default() *Config {
	return &Config{
		Remote:            "origin",
		DefaultBranch:     "develop",
		SyncStrategy:      "rebase",
		ProtectedBranches: DefaultProtectedBranches,
	}
}

// ensureDir creates the config directory if it doesn't exist.
func ensureDir() error {
	return os.MkdirAll(configDir, 0755)
}

func Load() (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		var syntaxErr *json.SyntaxError
		if errors.As(err, &syntaxErr) {
			return nil, &cli.Error{
				Message: "Failed to load configuration: .gx/config contains invalid JSON.",
				Hint:    "Check the file for syntax errors and fix them, or delete it and run gx init.",
			}
		}
		return nil, err
	}

	// Validate syncStrategy: warn and reset to "rebase" if invalid
	if cfg.SyncStrategy != "" && cfg.SyncStrategy != "rebase" && cfg.SyncStrategy != "merge" {
		fmt.Fprintf(os.Stderr, "Warning: .gx/config has invalid syncStrategy '%s'. Resetting to 'rebase'.\n", cfg.SyncStrategy)
		cfg.SyncStrategy = "rebase"
	}

	if cfg.Remote == "" {
		cfg.Remote = "origin"
	}
	if cfg.DefaultBranch == "" {
		cfg.DefaultBranch = "develop"
	}
	if cfg.SyncStrategy == "" {
		cfg.SyncStrategy = "rebase"
	}

	if len(cfg.ProtectedBranches) == 0 {
		cfg.ProtectedBranches = DefaultProtectedBranches
	}

	return cfg, nil
}

// Save writes cfg to .gx/config in JSON format.
// Creates the .gx directory if it doesn't exist.
func Save(cfg *Config) error {
	if err := ensureDir(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configFile, append(data, '\n'), 0644)
}

// Exists returns true if a .gx/config file exists.
func Exists() bool {
	_, err := os.Stat(configFile)
	return err == nil
}

// Set writes a config key to .gx/config. Supported keys:
// remote, defaultBranch, syncStrategy, sensitivePatterns.
// The sensitivePatterns value is a JSON string array, e.g. '["*.tfvars"]'.
func Set(key, value string) error {
	field, ok := configFields[key]
	if !ok {
		return fmt.Errorf("unknown config key: %s", key)
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s must not be empty", key)
	}

	if key == "syncStrategy" {
		v := strings.ToLower(value)
		if v != "rebase" && v != "merge" {
			return fmt.Errorf("syncStrategy must be 'rebase' or 'merge', got '%s'", value)
		}
		value = v
	}

	cfg, err := Load()
	if err != nil {
		return err
	}

	switch field {
	case "remote":
		cfg.Remote = value
	case "defaultBranch":
		cfg.DefaultBranch = value
	case "syncStrategy":
		cfg.SyncStrategy = value
	case "sensitivePatterns":
		var patterns []string
		if err := json.Unmarshal([]byte(value), &patterns); err != nil {
			return fmt.Errorf("sensitivePatterns must be a JSON string array, got '%s'", value)
		}
		cfg.SensitivePatterns = patterns
	case "protectedBranches":
		var branches []string
		if err := json.Unmarshal([]byte(value), &branches); err != nil {
			return fmt.Errorf("protectedBranches must be a JSON string array, got '%s'", value)
		}
		cfg.ProtectedBranches = branches
	}

	return Save(cfg)
}

// Get returns the value of a config key. Returns empty string for unknown keys.
// Supported keys: remote, defaultBranch, syncStrategy, sensitivePatterns.
func Get(key string) (string, error) {
	field, ok := configFields[key]
	if !ok {
		return "", fmt.Errorf("unknown config key: %s", key)
	}

	cfg, err := Load()
	if err != nil {
		return "", err
	}

	switch field {
	case "remote":
		return cfg.Remote, nil
	case "defaultBranch":
		return cfg.DefaultBranch, nil
	case "syncStrategy":
		return cfg.SyncStrategy, nil
	case "sensitivePatterns":
		if len(cfg.SensitivePatterns) == 0 {
			return "[]", nil
		}
		data, err := json.Marshal(cfg.SensitivePatterns)
		if err != nil {
			return "", err
		}
		return string(data), nil
	case "protectedBranches":
		if len(cfg.ProtectedBranches) == 0 {
			return "[]", nil
		}
		data, err := json.Marshal(cfg.ProtectedBranches)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return "", nil
}

// AppendSensitivePatterns appends patterns to the existing sensitivePatterns list,
// deduplicates and saves. Uses git.SanitizePatterns for dedup.
func AppendSensitivePatterns(patterns []string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.SensitivePatterns = git.SanitizePatterns(append(cfg.SensitivePatterns, patterns...))
	return Save(cfg)
}

// SetSensitivePatterns replaces the sensitivePatterns list entirely with the given patterns,
// sanitizes and saves. Uses git.SanitizePatterns.
func SetSensitivePatterns(patterns []string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.SensitivePatterns = git.SanitizePatterns(patterns)
	return Save(cfg)
}

// SetProtectedBranches replaces the protectedBranches list entirely with the given branches.
func SetProtectedBranches(branches []string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.ProtectedBranches = branches
	return Save(cfg)
}

// AppendProtectedBranches appends branches to the existing protectedBranches list and saves.
func AppendProtectedBranches(branches []string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.ProtectedBranches = append(cfg.ProtectedBranches, branches...)
	return Save(cfg)
}

var configFields = map[string]string{
	"remote":             "remote",
	"defaultBranch":      "defaultBranch",
	"syncStrategy":       "syncStrategy",
	"sensitivePatterns":  "sensitivePatterns",
	"protectedBranches":  "protectedBranches",
}
