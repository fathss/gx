# Config Domain

**File:** `internal/config/config.go`

Persistent configuration for gx, stored at `.gx/config`. The `config`
package loads, validates, and saves the configuration file. It is **not**
part of the `cmd` → `workflow` → `git` → `runner` chain: it imports
`internal/git` (`SanitizePatterns`) and `internal/cli` (`cli.Error`), and is
consumed directly by `cmd/config.go`.

For the user-facing `gx config` CLI (usage, flags, examples), see
`docs/commands/config.md`.

## Constants

- `configDir` — `".gx"`
- `configFile` — `".gx/config"`

## Defaults

- `DefaultSensitivePatterns` — `[".env", "*.pem", "*secret*", "*.key"]`.
  Written to new configs by `gx init`. They live here (not in the `git`
  package) so the config file is the single source of truth.
- `DefaultProtectedBranches` — `["main", "master", "develop"]`. Used as the
  default when none are configured.

## Config struct

| Field              | JSON tag                  |
| ------------------ | ------------------------- |
| `Remote`           | `remote`                  |
| `DefaultBranch`    | `defaultBranch`           |
| `SyncStrategy`     | `syncStrategy`            |
| `SensitivePatterns` | `sensitivePatterns,omitempty`  |
| `ProtectedBranches` | `protectedBranches,omitempty` |

The two list fields are omitted from the file when empty.

## Default

```
Default() → *Config
```

Returns a config with `Remote: "origin"`, `DefaultBranch: "develop"`,
`SyncStrategy: "rebase"`, and `ProtectedBranches: DefaultProtectedBranches`.
No sensitive patterns are set.

## Load

```
Load() → (*Config, error)
```

1. Starts from `Default()`.
2. Reads `.gx/config`. A missing file returns the defaults with no error.
3. Invalid JSON → `cli.Error`:
   - Message: `"Failed to load configuration: .gx/config contains invalid JSON."`
   - Hint: `"Check the file for syntax errors and fix them, or delete it and run gx init."`
4. Invalid `syncStrategy` → prints a warning to stderr
   (`Warning: .gx/config has invalid syncStrategy '<value>'. Resetting to 'rebase'.`)
   and resets the value to `"rebase"`.
5. Empty fields fall back to defaults: `remote` → `"origin"`,
   `defaultBranch` → `"develop"`, `syncStrategy` → `"rebase"`, empty
   `protectedBranches` → `DefaultProtectedBranches`.

## Save

```
Save(cfg *Config) error
```

Creates the `.gx` directory if missing (`os.MkdirAll` with 0755), marshals
with `json.MarshalIndent(cfg, "", "  ")`, and writes `.gx/config` with 0644
permissions plus a trailing newline.

## Exists

```
Exists() bool
```

Returns `true` when a `.gx/config` file exists (`os.Stat` succeeds).

## Get

```
Get(key string) (string, error)
```

- Unknown key → `"unknown config key: <key>"`.
- Scalar keys → the stored value.
- List keys → JSON-marshaled string, or `"[]"` when empty.

## Set

```
Set(key, value string) error
```

1. Unknown key → `"unknown config key: <key>"`.
2. Trims the value; an empty value → `"<key> must not be empty"`.
3. `syncStrategy` is lowercased and must be `"rebase"` or `"merge"`, else
   `"syncStrategy must be 'rebase' or 'merge', got '<value>'"`.
4. List keys parse the value as a JSON string array; on failure
   `"<key> must be a JSON string array, got '<value>'"`.
5. Loads the current config, sets the field, and saves.

## Sensitive patterns (variadic CLI path)

- `AppendSensitivePatterns(patterns []string) error` — appends to the
  existing list, runs `git.SanitizePatterns` (trims, drops empties and
  duplicates), saves.
- `SetSensitivePatterns(patterns []string) error` — replaces the list,
  runs `git.SanitizePatterns`, saves.

## Protected branches

- `AppendProtectedBranches(branches []string) error` — plain append, saves.
- `SetProtectedBranches(branches []string) error` — plain replacement, saves.

No sanitization or deduplication for protected branches.

## configFields

Key map accepted by `Get`/`Set`: `remote`, `defaultBranch`, `syncStrategy`,
`sensitivePatterns`, `protectedBranches`.

## Consumers

| Caller                          | Usage                                                                   |
| ------------------------------- | ----------------------------------------------------------------------- |
| `cmd/config.go`                 | `Get`, `Set`, `Append/SetSensitivePatterns`, `Append/SetProtectedBranches`, `printConfig` |
| `internal/workflow/guard.go`    | `RequireConfig()` errors when `config.Exists()` is false                |
| `internal/workflow/init.go`     | Blocks re-init unless `--force` via `config.Exists()`; seeds `SensitivePatterns` from `DefaultSensitivePatterns`, writes via `Save` |

See `docs/commands/config.md` for the user-facing CLI.