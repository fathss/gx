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
	SensitivePatterns []string `json:"sensitivePatterns,omitempty"`
	ProtectedBranches []string `json:"protectedBranches,omitempty"`
}

func Default() *Config {
	return &Config{
		Remote:            "origin",
		DefaultBranch:     "develop",
		SyncStrategy:      "rebase",
		SensitivePatterns: DefaultSensitivePatterns,
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

	// Warn and reset persisted values outside the syncStrategy domain.
	if cfg.SyncStrategy != "" && !fieldAllows(syncStrategies, cfg.SyncStrategy) {
		fmt.Fprintf(os.Stderr, "Warning: .gx/config has invalid syncStrategy '%s'. Resetting to 'rebase'.\n", cfg.SyncStrategy)
		cfg.SyncStrategy = "rebase"
	}

	// Every field the file left empty falls back to its default, through
	// the registry accessors.
	applyDefaults(cfg, Default())

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

// Set writes a config key to .gx/config. The key set and per-key
// validation live in the registry (see Keys()).
func Set(key string, value string) error {
	field, ok := lookupField(key)
	if !ok {
		return fmt.Errorf("unknown config key: %s", key)
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s must not be empty", key)
	}

	cfg, err := Load()
	if err != nil {
		return err
	}
	if err := field.set(cfg, value); err != nil {
		return err
	}

	return Save(cfg)
}

// Get returns the value of a config key. Returns an error for unknown
// keys; the key set is the registry (see Keys()).
func Get(key string) (string, error) {
	field, ok := lookupField(key)
	if !ok {
		return "", fmt.Errorf("unknown config key: %s", key)
	}

	cfg, err := Load()
	if err != nil {
		return "", err
	}
	return field.get(cfg), nil
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
