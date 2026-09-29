package config

import (
	"encoding/json"
	"fmt"
	"strings"
)

// syncStrategies is the value domain for syncStrategy. Set rejects
// anything outside it; Load warns and resets persisted values outside it.
var syncStrategies = []string{"rebase", "merge"}

// field is one registry entry: how a key reads, writes (with its
// validation), and whether it is currently empty. The ordered `fields`
// slice is the single source for the key set — Get, Set, Load's empty
// fallbacks, and cmd's printConfig all derive from it.
type field struct {
	key   string
	get   func(*Config) string
	set   func(*Config, string) error
	empty func(*Config) bool
	// allowed is the value domain; empty means any non-empty value.
	allowed []string
}

// scalarField builds a registry entry for a plain string key.
func scalarField(key string, read func(*Config) string, write func(*Config, string)) field {
	return field{
		key: key,
		get: func(cfg *Config) string {
			return read(cfg)
		},
		set: func(cfg *Config, value string) error {
			write(cfg, value)
			return nil
		},
		empty: func(cfg *Config) bool {
			return read(cfg) == ""
		},
	}
}

// listField builds a registry entry for a JSON string-array key. Get
// renders the list as JSON ("[]" when empty); set parses the same shape.
func listField(key string, read func(*Config) []string, write func(*Config, []string)) field {
	return field{
		key: key,
		get: func(cfg *Config) string {
			list := read(cfg)
			if len(list) == 0 {
				return "[]"
			}
			data, err := json.Marshal(list) // []string never fails to marshal
			if err != nil {
				return "[]"
			}
			return string(data)
		},
		set: func(cfg *Config, value string) error {
			var list []string
			if err := json.Unmarshal([]byte(value), &list); err != nil {
				return fmt.Errorf("%s must be a JSON string array, got '%s'", key, value)
			}
			write(cfg, list)
			return nil
		},
		empty: func(cfg *Config) bool {
			return len(read(cfg)) == 0
		},
	}
}

// fields is the registry, in display order. Adding a key means one entry
// here plus its Config field and json tag.
var fields = []field{
	scalarField("remote",
		func(cfg *Config) string { return cfg.Remote },
		func(cfg *Config, value string) { cfg.Remote = value }),
	scalarField("defaultBranch",
		func(cfg *Config) string { return cfg.DefaultBranch },
		func(cfg *Config, value string) { cfg.DefaultBranch = value }),
	{
		key:     "syncStrategy",
		allowed: syncStrategies,
		get: func(cfg *Config) string {
			return cfg.SyncStrategy
		},
		set: func(cfg *Config, value string) error {
			normalized := strings.ToLower(value)
			if !fieldAllows(syncStrategies, normalized) {
				return fmt.Errorf("syncStrategy must be %s, got '%s'", quoteOrList(syncStrategies), value)
			}
			cfg.SyncStrategy = normalized
			return nil
		},
		empty: func(cfg *Config) bool {
			return cfg.SyncStrategy == ""
		},
	},
	listField("sensitivePatterns",
		func(cfg *Config) []string { return cfg.SensitivePatterns },
		func(cfg *Config, list []string) { cfg.SensitivePatterns = list }),
	listField("protectedBranches",
		func(cfg *Config) []string { return cfg.ProtectedBranches },
		func(cfg *Config, list []string) { cfg.ProtectedBranches = list }),
}

// lookupField finds a registry entry by key.
func lookupField(key string) (*field, bool) {
	for i := range fields {
		if fields[i].key == key {
			return &fields[i], true
		}
	}
	return nil, false
}

// Keys returns every config key in display order.
func Keys() []string {
	keys := make([]string, 0, len(fields))
	for _, f := range fields {
		keys = append(keys, f.key)
	}
	return keys
}

// applyDefaults fills any field the file left empty from def, through the
// same accessors Set and Get use. Default values are valid by
// construction, so their round-trip cannot fail.
func applyDefaults(cfg *Config, def *Config) {
	for i := range fields {
		f := &fields[i]
		if f.empty(cfg) {
			_ = f.set(cfg, f.get(def))
		}
	}
}

// fieldAllows reports whether v is inside a value domain.
func fieldAllows(domain []string, v string) bool {
	for _, candidate := range domain {
		if v == candidate {
			return true
		}
	}
	return false
}

// quoteOrList renders a value domain for an error message:
// 'rebase', 'rebase' or 'merge', 'a', 'b' or 'c'.
func quoteOrList(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, "'"+value+"'")
	}
	switch len(quoted) {
	case 0:
		return ""
	case 1:
		return quoted[0]
	default:
		return strings.Join(quoted[:len(quoted)-1], ", ") + " or " + quoted[len(quoted)-1]
	}
}
