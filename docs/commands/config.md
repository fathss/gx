# gx config

Inspect or update persistent configuration values in `.gx/config`.

---

## Usage

```bash
gx config
gx config list
gx config <key>
gx config <key> <value>
gx config sensitivePatterns <pattern>... [--overwrite]
gx config protectedBranches <branch>... [--overwrite]
```

- **`gx config` / `gx config list`**: Display all keys and their resolved values.
- **`gx config <key>`**: Output the current value of a single key.
- **`gx config <key> <value>`**: Set a scalar configuration value.
- **`gx config <listKey> <value>...`**: Append or overwrite entries for array-based keys (`sensitivePatterns`, `protectedBranches`).

For detailed schema definitions, see the [Configuration Specification](../concepts/configuration.md).

---

## Flags

| Flag              | Description                                                                                             |
| ----------------- | ------------------------------------------------------------------------------------------------------- |
| `--overwrite`     | Replace the target list key (`sensitivePatterns` or `protectedBranches`) entirely instead of appending. |
| `--verbose`, `-v` | Show internal command plumbing.                                                                         |

`--overwrite` is valid only with list keys (`sensitivePatterns` and `protectedBranches`).

---

## Examples

### View current configuration

```bash
gx config
# remote: origin
# defaultBranch: develop
# syncStrategy: rebase
# sensitivePatterns: [".env", "*.pem", "*secret*", "*.key"]
# protectedBranches: ["main", "master", "develop"]
```

### Get and set scalar settings

```bash
# Inspect remote
gx config remote

# Update default branch and strategy
gx config defaultBranch main
gx config syncStrategy merge
```

### Manage pattern and branch lists

```bash
# Append extra sensitive file patterns
gx config sensitivePatterns .env.local *.crt

# Overwrite protected branches list completely
gx config protectedBranches main release --overwrite
```

---

## Edge Cases & Recovery

| Scenario                     | Behavior & Recovery                                                                                                |
| ---------------------------- | ------------------------------------------------------------------------------------------------------------------ |
| Outside a Git repository     | Blocked by pre-flight guard: `Not inside a Git repository.`                                                        |
| Missing `.gx/config` file    | Allowed. `gx config` can read default values and creates `.gx/config` upon setting any value.                      |
| Unknown key                  | Setting or reading an unrecognized key returns an error: `unknown config key: <key>`.                              |
| Empty string value           | Attempting to set an empty value returns: `<key> must not be empty`.                                               |
| Invalid `syncStrategy` value | Returns: `syncStrategy must be 'rebase' or 'merge', got '<value>'`.                                                |
| `--overwrite` on scalar key  | Returns error: `--overwrite is only valid for sensitivePatterns and protectedBranches`.                            |
| Corrupt JSON in `.gx/config` | Returns: `Failed to load configuration: .gx/config contains invalid JSON.` Fix syntax or re-run `gx init --force`. |
