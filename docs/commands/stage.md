# gx stage

Stage modified tracked and untracked files into the Git index with sensitive file guardrails and preview.

---

## Usage

```bash
gx stage [-a | --all] [-p | --patch] [--allow-sensitive] [--exclude <pattern>] [<files>...]
gx add ...
```

`gx stage` (alias: `gx add`) prepares changes for commit with safety checks:

- **Categorized preview**: When run without file arguments, scans the repository and shows unstaged tracked files (with status badges and diffstats) and untracked files.
- **Untracked files prompt**: Asks for confirmation before staging untracked files (`[y/N]`, default No). Bypassed when `-a` or specific file paths are supplied.
- **Specific files**: Passing file arguments stages only the named files, bypassing preview and confirmation prompts.
- **Sensitive file protection**: Refuses to stage files matching configured sensitive patterns (e.g., `.env`, `*.pem`, `*secret*`, `*.key`) unless overridden with `--allow-sensitive`.
- **Hunk staging**: `-p` / `--patch` runs interactive staging via `git add -p`.
- **Exclude filtering**: `--exclude` filters out matching files from staging.
- **Staged summary**: Reports the net count of files staged during the command along with diffstats (`(+ins | -del)`).

---

## Flags

| Flag                  | Shorthand | Description                                                                   |
| --------------------- | --------- | ----------------------------------------------------------------------------- |
| `--all`               | `-a`      | Stage all modified and untracked files without prompting.                     |
| `--patch`             | `-p`      | Interactively select hunks to stage (`git add -p`).                           |
| `--allow-sensitive`   |           | Override sensitive-pattern blocking.                                          |
| `--exclude <pattern>` |           | Glob patterns to exclude from staging (repeatable, comma-separated).          |
| `--no-pager`          |           | Suppress the pager when preview exceeds terminal height.                      |
| `--verbose`           | `-v`      | Show underlying git plumbing commands.                                        |

---

## Examples

### Interactive staging with preview

```bash
gx stage
# Unstaged tracked files:
#   M  src/main.go +5 -2
#
# New untracked files:
#   U  notes.md
# Stage 1 untracked file(s)? [y/N]: y
# Staged 2 files (+15)
```

### Stage all modified and untracked files immediately

```bash
gx stage -a
# Staged 3 files (+42 | -5)
```

### Stage specific files

```bash
gx stage src/main.go src/auth.go
# Staged 2 files (+20 | -3)
```

### Interactive patch mode

```bash
gx stage -p
```

### Stage excluding patterns

```bash
gx stage --exclude "*.log,tmp/**"
```

### Using the `add` alias

```bash
gx add -a
```

### Staging followed by save

```bash
gx stage src/main.go
# Staged 1 file (+10)
gx save -m "feat(core): implement core runner logic"
# Saved commit 7a8b9c0 (feat/runner)
# "feat(core): implement core runner logic"
# ...
```

---

## Edge Cases & Recovery

| Scenario                           | Behavior & Recovery                                                                                                          |
| ---------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| Rebase or merge in progress        | Refuses to stage: `A rebase is already in progress.` Resolve or abort with `gx sync --continue` or `git rebase --abort`.      |
| Sensitive file detected            | Hard block: `Sensitive file(s) detected — refusing to stage`. Add to `.gitignore`, remove file, or pass `--allow-sensitive`. |
| Sensitive file in `-p` / `--patch` | Non-fatal warning is displayed; user can skip sensitive hunks interactively.                                                 |
| Working tree clean                 | Informs: `Working tree is clean, nothing to stage.` Exits with code 0.                                                       |
| No new changes to stage            | If changes are already staged and no unstaged changes exist, informs: `No new changes to stage.` Exits with code 0.          |
| Submodule paths detected           | Submodule directory is skipped from staging with a notice to use manual `git add`.                                           |
| Outside a Git repository           | Blocked by central guard: `Not inside a Git repository.`                                                                     |
| Missing configuration              | Blocked by central guard: run `gx init` first.                                                                               |
