# gx init

Initialize gx configuration for this repository. Auto-detects the remote and
default branch, then writes `.gx/config`.

Running `gx init` is required before using any gx command that needs
configuration (`sync`, `save`, `ship`, etc.). The central pre-flight guard
blocks those commands with:

```
✗ No existing configuration
hint: Run gx init to start using gx
```

## Requirements

- Must be inside a Git repository. The central guard blocks outside repos with:
  ```
  ✗ Not inside a Git repository.
  hint: Run gx inside a cloned repository.
  ```
- Must not have an existing `.gx/config`. If it exists, `gx init` exits with
  instructions to use `gx config` to update values.

## Usage

```
gx init [<remote> [<branch>]]
gx init --remote <name> --branch <name>
```

## Flags

| Flag              | Description                                    |
| ----------------- | ---------------------------------------------- |
| `--remote`        | Remote name (overrides auto-detection)         |
| `--branch`        | Default branch name (overrides auto-detection) |
| `-f`, `--force`   | Overwrite existing `.gx/config` if it exists   |

## Detection

- **Remote**: determined from the current branch's tracking branch, or the
  first configured remote.
- **Default branch**: resolved from the remote's HEAD symbolic ref, or falls
  back to checking local branches (`main`, `master`, `develop`).

If detection fails, specify the value explicitly using positional arguments or
flags. The real error is included in parenthesized form on a second hint
line:

```
✗ Failed to detect remote.
hint: (no remotes configured)
hint: Specify it explicitly: gx init <remote>

✗ Failed to detect default branch.
hint: (could not detect default branch)
hint: Specify it explicitly: gx init <remote> <branch>
```

## Examples

```bash
# Auto-detect and write config
gx init

# Force remote name
gx init upstream

# Force both remote and default branch
gx init upstream main

# Flag form
gx init --remote upstream --branch develop
```

## Notes

- Fails if `.gx/config` already exists — edit it directly or use `gx config`
  to update values.
- Only measurable settings are detected. `syncStrategy` defaults to `rebase`
  and can be changed with `gx config syncStrategy merge`.
- `gx init` seeds `sensitivePatterns` with default patterns (`.env`, `*.pem`,
  `*secret*`, `*.key`). Additional patterns can be added later with
  `gx config sensitivePatterns <pattern>...`.
