# gx init

Initialize gx configuration for the current repository.

---

## Usage

```bash
gx init [<remote> [<branch>]]
gx init [--remote <name>] [--branch <name>] [--force]
```

`gx init` inspects repository settings, automatically detects the primary remote and default branch, and writes the initial `.gx/config` file.

Detection rules:

- **Remote**: Detected from current branch tracking information, falling back to the first configured git remote.
- **Default Branch**: Detected from `<remote>/HEAD` symbolic reference, falling back to local branches (`main`, `master`, `develop`).
- **Sensitive Patterns**: Automatically seeded with default values (`.env`, `*.pem`, `*secret*`, `*.key`).

---

## Flags

| Flag        | Shorthand | Description                                           |
| ----------- | --------- | ----------------------------------------------------- |
| `--remote`  |           | Remote name to use (overrides auto-detection).        |
| `--branch`  |           | Default base branch name (overrides auto-detection).  |
| `--force`   | `-f`      | Overwrite existing `.gx/config` if it already exists. |
| `--verbose` | `-v`      | Show underlying git plumbing commands.                |

---

## Examples

### Auto-detect repository settings

```bash
gx init
# Wrote .gx/config
#   remote:            origin
#   defaultBranch:     main
#   syncStrategy:      rebase
#   sensitivePatterns: [.env *.pem *secret* *.key]
```

### Explicit overrides via positional arguments

```bash
# Override remote only
gx init upstream

# Override remote and default branch
gx init upstream develop
```

### Overwrite existing configuration

```bash
gx init --force --remote origin --branch main
```

---

## Edge Cases & Recovery

| Scenario                        | Behavior & Recovery                                                                                                                            |
| ------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| Outside a Git repository        | Blocked by central guard: `Not inside a Git repository.`                                                                                       |
| Existing configuration          | Refuses to overwrite unless `--force` is provided: `Configuration already exists. Edit .gx/config directly or use gx config to update values.` |
| No remotes configured           | Auto-detection fails: `Failed to detect remote. Specify it explicitly: gx init <remote>`                                                       |
| No remote HEAD or base branch   | Auto-detection fails: `Failed to detect default branch. Specify it explicitly: gx init <remote> <branch>`                                      |
| Read-only directory permissions | Fails with write error: `Failed to write configuration. Check permissions on the .gx/ directory.`                                              |
