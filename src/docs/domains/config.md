# gx config

Get or set persistent configuration values in `.gx/config`.

## Usage

```
gx config [<key> [<value>]]
gx config list
```

## Flags

| Flag | Description |
| ---- | ----------- |
| `--overwrite` | Replace `sensitivePatterns` or `protectedBranches` list instead of appending |

## Keys

| Key | Default | Description |
|---|---|---|---|
| `remote` | `origin` | Remote name to fetch from |
| `defaultBranch` | `develop` | Branch to sync against |
| `syncStrategy` | `rebase` | Sync strategy (`rebase` or `merge`) |
| `sensitivePatterns` | `[]` (seeded by `gx init`) | Sensitive file glob patterns (variadic or JSON array) |
| `protectedBranches` | `["main", "master", "develop"]` | Branches protected from direct push |

## Examples

```bash
# List all values
gx config list

# Print a single value
gx config remote

# Set a value
gx config remote upstream
gx config defaultBranch main
gx config syncStrategy merge

# Set sensitive patterns — append mode (default)
gx config sensitivePatterns '["*.tfvars", "*credentials*"]'

# Variadic append form (repeatable patterns)
gx config sensitivePatterns .env .gitignore

# Overwrite the entire list
gx config sensitivePatterns .env .gitignore --overwrite
gx config sensitivePatterns '["*.tfvars"]' --overwrite
```

## Notes

- Must be inside a Git repository — the central pre-flight guard blocks outside repos.
- Values are persisted in `.gx/config` in the current directory (repository root).
- The config file can also be created and edited by hand — `gx config` is a convenience wrapper.
- Run `gx init` to auto-detect remote and default branch before using `gx sync`.
- `gx config` is the only command that **bypasses the normal workflow architecture** — it reads and writes `.gx/config` directly via the `config` package with no workflow or git layer.
- `sensitivePatterns` accepts both a JSON string array or a variadic list of patterns (one per argument). By default, patterns are appended to the existing list. Use `--overwrite` to replace the list entirely.
