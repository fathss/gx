# gx config

Get or set persistent configuration values in `.gx/config`.

`gx config` is the only command that **bypasses the normal workflow
architecture** — it reads and writes `.gx/config` directly via the `config`
package, with no workflow or git layer in between.

## Usage

```
gx config [<key> [<value>]]
gx config list
```

- `gx config` — print all values.
- `gx config list` — print all values. `list` is a **positional
  subcommand**, not a flag.
- `gx config <key>` — print one value.
- `gx config <key> <value>` — set one value.
- `gx config sensitivePatterns <pattern>...` / `gx config protectedBranches
  <branch>...` — append list values (variadic positional form).

## Flags

| Flag          | Description                                                                     |
| ------------- | ------------------------------------------------------------------------------- |
| `--overwrite` | Replace the `sensitivePatterns` or `protectedBranches` list instead of appending |

`--overwrite` is only valid for the list keys `sensitivePatterns` and
`protectedBranches`. It has no effect on scalar keys — using it with any
other key and `gx config` with no arguments produces an error.
Appending is the default; `--overwrite` replaces the entire list.

## Keys

| Key                 | Default                                             | Notes                                             |
| ------------------- | --------------------------------------------------- | ------------------------------------------------- |
| `remote`            | `origin`                                            | Remote name to fetch from                        |
| `defaultBranch`     | `develop`                                           | Branch to sync against                           |
| `syncStrategy`      | `rebase`                                            | `rebase` or `merge`                              |
| `sensitivePatterns` | `[]` (seeded by `gx init`)                          | List key — variadic positional values            |
| `protectedBranches` | `["main", "master", "develop"]`                     | List key — variadic positional values            |

`gx init` seeds `sensitivePatterns` with the default patterns (`.env`,
`*.pem`, `*secret*`, `*.key`). Reading a key with no stored list prints
`[]`.

## List keys

`sensitivePatterns` and `protectedBranches` accept **variadic positional
values only** — one value per argument. The CLI always dispatches these two
keys to the variadic `Append/Set{...}Patterns` helpers, so a JSON array
string passed as a single argument is treated as one literal value. By
default values are appended to the existing list; with `--overwrite` the
list is replaced entirely.

```bash
# Variadic positional values (append, default)
gx config sensitivePatterns .env .gitignore

# Variadic positional values (replace)
gx config sensitivePatterns .env .gitignore --overwrite
```

The JSON-array-string form exists only at the `config.Set()` API level (see
`docs/domains/config.md`) — the CLI never calls it for list keys.

## Error cases

| Condition                                    | Behaviour                                                                                             |
| -------------------------------------------- | ----------------------------------------------------------------------------------------------------- |
| Not a Git repository                         | Central guard blocks: "Not inside a Git repository." (hint: "Run gx inside a cloned repository.")     |
| `gx config --overwrite` (no key)             | "sensitivePatterns or protectedBranches: no patterns specified" + usage hint                          |
| `gx config <listkey> --overwrite` (no values) | "<key>: no patterns specified" + usage hint                                                           |
| `--overwrite` with a non-list key            | "--overwrite is only valid for sensitivePatterns and protectedBranches"                               |
| `gx config list <extra>...`                  | "config list takes no arguments"                                                                      |
| Scalar key with more than 1 value            | "config <key> takes at most 1 value"                                                                  |
| Unknown key (single-key read)                | "Failed to get config key: <key>."                                                                    |
| Unknown key (set)                            | "unknown config key: <key>"                                                                           |
| Empty value                                  | "<key> must not be empty"                                                                             |
| Invalid `syncStrategy`                       | "syncStrategy must be 'rebase' or 'merge', got '<value>'"                                             |
| Non-JSON-array list value via `config.Set`   | API-level only: "sensitivePatterns must be a JSON string array, got '<value>'" (likewise `protectedBranches`); not reachable through the CLI, which uses the variadic helpers |
| Single-key read with no value                | Prints `(not set)`                                                                                    |
| Invalid JSON in `.gx/config`                 | `config.Load()` fails; surfaces as e.g. "Failed to load config: ..." (show-all) or "Failed to get config key: <key>." |

## Examples

```bash
# Show all values
gx config
gx config list

# Print a single value
gx config remote

# Set a value
gx config remote upstream
gx config defaultBranch main
gx config syncStrategy merge

# Append sensitive patterns (default)
gx config sensitivePatterns .env .gitignore

# Overwrite sensitive patterns
gx config sensitivePatterns .env .gitignore --overwrite

# Append protected branches (default)
gx config protectedBranches main release

# Overwrite protected branches
gx config protectedBranches main release --overwrite
```

## Notes

- Must be inside a Git repository: `gx config` has no `no_repo` annotation,
  so the central pre-flight guard blocks it outside repos.
- Works **without** `.gx/config`: the `no_config` annotation skips the config
  guard, and `config.Load()` falls back to defaults — effectively acting on a
  fresh default config.
- Values are persisted in `.gx/config` at the repository root.
- Patterns added via `gx config sensitivePatterns` are deduplicated
  (`git.SanitizePatterns`); `protectedBranches` values are not.
- Run `gx init` to auto-detect remote and default branch before using `gx sync`.
- The config file can also be created and edited by hand — `gx config` is a
  convenience wrapper.