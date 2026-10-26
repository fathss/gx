package config

import (
	"strings"
	"testing"
)

func TestKeysReturnsRegistryOrder(t *testing.T) {
	want := []string{"remote", "defaultBranch", "syncStrategy", "sensitivePatterns", "protectedBranches"}
	got := Keys()
	if len(got) != len(want) {
		t.Fatalf("Keys() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Keys()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestLookupFieldUnknownKey(t *testing.T) {
	if _, ok := lookupField("nope"); ok {
		t.Error("lookupField(\"nope\") found, want not found")
	}
	if _, ok := lookupField("remote"); !ok {
		t.Error(`lookupField("remote") not found, want found`)
	}
}

func TestSetRejectsUnknownKeyAndEmptyValue(t *testing.T) {
	if err := Set("nope", "x"); err == nil || err.Error() != "unknown config key: nope" {
		t.Errorf("Set(nope) = %v, want unknown config key", err)
	}
	if err := Set("remote", "  "); err == nil || err.Error() != "remote must not be empty" {
		t.Errorf("Set(remote, blank) = %v, want must not be empty", err)
	}
}

func TestGetRejectsUnknownKey(t *testing.T) {
	if _, err := Get("nope"); err == nil || err.Error() != "unknown config key: nope" {
		t.Errorf("Get(nope) = %v, want unknown config key", err)
	}
}

func TestScalarFieldRoundTrip(t *testing.T) {
	f, ok := lookupField("remote")
	if !ok {
		t.Fatal("remote not registered")
	}
	cfg := &Config{}
	if !f.empty(cfg) {
		t.Error("fresh cfg not empty")
	}
	if err := f.set(cfg, "fork"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if f.get(cfg) != "fork" {
		t.Errorf("get = %q, want fork", f.get(cfg))
	}
	if f.empty(cfg) {
		t.Error("cfg empty after set")
	}
}

func TestListFieldRendersAndParsesJSON(t *testing.T) {
	f, ok := lookupField("sensitivePatterns")
	if !ok {
		t.Fatal("sensitivePatterns not registered")
	}
	cfg := &Config{}
	if got := f.get(cfg); got != "[]" {
		t.Errorf("empty list get = %q, want []", got)
	}
	if err := f.set(cfg, `["*.tfvars"]`); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := f.get(cfg); got != `["*.tfvars"]` {
		t.Errorf("get = %q, want [\"*.tfvars\"]", got)
	}
	err := f.set(cfg, "not-json")
	if err == nil || err.Error() != `sensitivePatterns must be a JSON string array, got 'not-json'` {
		t.Errorf("set(not-json) = %v, want JSON string array error", err)
	}
}

func TestSyncStrategySetValidatesAndNormalizes(t *testing.T) {
	f, ok := lookupField("syncStrategy")
	if !ok {
		t.Fatal("syncStrategy not registered")
	}
	cfg := &Config{}
	if err := f.set(cfg, "MERGE"); err != nil {
		t.Fatalf("set(MERGE): %v", err)
	}
	if cfg.SyncStrategy != "merge" {
		t.Errorf("SyncStrategy = %q, want merge", cfg.SyncStrategy)
	}
	err := f.set(cfg, "ff")
	if err == nil || err.Error() != "syncStrategy must be 'rebase' or 'merge', got 'ff'" {
		t.Errorf("set(ff) = %v, want domain error", err)
	}
	if cfg.SyncStrategy != "merge" {
		t.Errorf("failed set mutated SyncStrategy to %q, want merge", cfg.SyncStrategy)
	}
}

func TestApplyDefaultsFillsOnlyEmptyFields(t *testing.T) {
	cfg := &Config{
		Remote:            "fork",
		DefaultBranch:     "",
		SyncStrategy:      "merge",
		SensitivePatterns: []string{},
	}
	applyDefaults(cfg, Default())

	if cfg.Remote != "fork" {
		t.Errorf("Remote = %q, want fork (untouched)", cfg.Remote)
	}
	if cfg.DefaultBranch != "develop" {
		t.Errorf("DefaultBranch = %q, want develop", cfg.DefaultBranch)
	}
	if cfg.SyncStrategy != "merge" {
		t.Errorf("SyncStrategy = %q, want merge (untouched)", cfg.SyncStrategy)
	}
	wantPatterns := strings.Join(DefaultSensitivePatterns, ",")
	if strings.Join(cfg.SensitivePatterns, ",") != wantPatterns {
		t.Errorf("SensitivePatterns = %v, want %v", cfg.SensitivePatterns, DefaultSensitivePatterns)
	}
	if len(cfg.ProtectedBranches) == 0 {
		t.Error("ProtectedBranches not seeded")
	}
}

func TestDefaultSeedsSensitivePatterns(t *testing.T) {
	cfg := Default()
	if len(cfg.SensitivePatterns) != len(DefaultSensitivePatterns) {
		t.Fatalf("Default().SensitivePatterns = %v, want %v", cfg.SensitivePatterns, DefaultSensitivePatterns)
	}
	for i, pattern := range DefaultSensitivePatterns {
		if cfg.SensitivePatterns[i] != pattern {
			t.Errorf("SensitivePatterns[%d] = %q, want %q", i, cfg.SensitivePatterns[i], pattern)
		}
	}
}

func TestQuoteOrList(t *testing.T) {
	if got := quoteOrList(syncStrategies); got != "'rebase' or 'merge'" {
		t.Errorf("quoteOrList = %q", got)
	}
}
