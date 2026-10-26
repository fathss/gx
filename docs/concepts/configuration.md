# Configuration

Persistent configuration for `gx` is stored at `.gx/config` relative to the repository root.

---

## Schema

```json
{
  "remote": "origin",
  "defaultBranch": "develop",
  "syncStrategy": "rebase",
  "sensitivePatterns": [".env", "*.pem", "*secret*", "*.key"],
  "protectedBranches": ["main", "master", "develop"]
}
```

### Fields

| Field               | JSON Tag                      | Type     | Default                                  | Description                                                                  |
| ------------------- | ----------------------------- | -------- | ---------------------------------------- | ---------------------------------------------------------------------------- |
| `remote`            | `remote`                      | string   | `"origin"`                               | Default remote name for fetch, sync, and ship.                               |
| `defaultBranch`     | `defaultBranch`               | string   | `"develop"`                              | Trunk/base branch to sync against and check merged candidates.               |
| `syncStrategy`      | `syncStrategy`                | string   | `"rebase"`                               | Strategy used by `gx sync` (`"rebase"` or `"merge"`).                        |
| `sensitivePatterns` | `sensitivePatterns,omitempty` | []string | `[".env", "*.pem", "*secret*", "*.key"]` | Glob patterns blocked from accidental staging in `gx save`.                  |
| `protectedBranches` | `protectedBranches,omitempty` | []string | `["main", "master", "develop"]`          | Branches protected from direct push in `gx ship` and deletion in `gx clean`. |

---

## Resolution & Fallback Rules

1. **File Existence**:
   - Central guard `RequireConfig()` blocks operations when `.gx/config` is missing, prompting the user to run `gx init`.
   - Commands annotated with `no_config` (`gx init`, `gx config`) can execute without the file.

2. **Loading (`Load`)**:
   - Starts with `Default()`.
   - Reads `.gx/config`. Missing file returns defaults with no error.
   - Syntax errors in JSON return a clear `cli.Error` with recovery hints.
   - Values outside allowed domains (e.g. invalid `syncStrategy`) log a warning to stderr and reset to `"rebase"`.
   - Any empty scalar or list field falls back to its default value from `Default()`.

3. **Precedence**:
   - Command-line flags (e.g. `--remote`, `--branch`, `--rebase`, `--merge`) override configuration for that invocation.
   - Positional arguments (e.g. `gx sync origin develop`) override configuration for that invocation.
   - `.gx/config` values apply when flags/arguments are not passed.
   - Defaults apply when fields are empty or unset.

---

## Modifying Configuration

Configuration can be inspected and updated via:

- CLI commands: `gx config <key> [<value>]` (see [gx config](../commands/config.md))
- Direct file editing: `.gx/config` is plain JSON with 2-space indentation.
- Re-initialization: `gx init --force` overwrites with freshly detected values.
