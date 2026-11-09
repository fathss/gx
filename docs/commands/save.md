# gx save

Stage changes and create a commit with safety checks for sensitive files and empty commits.

---

## Usage

```bash
gx save [-a | --all] [-m <msg>] [--patch] [--allow-sensitive] [--allow-empty] [--exclude <pattern>] [<files>...]
```

`gx save` automates staging and committing with guardrails:

- **Pre-staged files support**: If changes are already staged (for example, via `gx stage`), `gx save` directly commits them without re-staging working tree files or prompting.
- **Full working tree scan**: When run without pre-staged changes or file arguments, scans the repository and shows categorized previews of modified tracked files and new untracked files before staging.
- **Untracked files prompt**: Asks for confirmation before staging untracked files (`[y/N]`, default No). Bypassed when `-a` or specific file arguments are provided.
- **Stage all flag**: `-a` / `--all` stages all modified tracked files and untracked files before committing without prompting.
- **Specific files**: Passing file arguments bypasses untracked confirmation and stages only the named files.
- **Sensitive file protection**: Refuses to stage files matching configured sensitive patterns (e.g., `.env`, `*.pem`, `*secret*`, `*.key`) and verifies pre-staged files in the index before committing unless overridden with `--allow-sensitive`.
- **Hunk staging**: `--patch` runs interactive staging via `git add -p`.
- **Commit summary card**: Displays formatted post-commit summary showing commit hash, branch, subject, categorized file changes with diffstats, and total insertions/deletions.

---

## Flags

| Flag                  | Shorthand | Description                                                                   |
| --------------------- | --------- | ----------------------------------------------------------------------------- |
| `-a`, `--all`         | `-a`      | Stage all modified tracked files and untracked files before committing.       |
| `--message <msg>`     | `-m`      | Commit message (skips editor).                                                |
| `--patch`             |           | Interactively select hunks to stage (`git add -p`).                           |
| `--allow-sensitive`   |           | Override sensitive-pattern block or suppress warnings.                        |
| `--allow-empty`       |           | Allow committing with nothing staged (skips empty guard prompt).              |
| `--exclude <pattern>` |           | Glob patterns to exclude from staging (repeatable, comma-separated).          |
| `--no-pager`          |           | Suppress the pager when preview exceeds terminal height.                      |
| `--verbose`           | `-v`      | Show underlying git plumbing commands.                                        |

---

## Examples

### Interactive commit (scan, review, stage, commit)

```bash
gx save
# Modified tracked files:
#   src/main.go +5 -2
# New untracked files:
#   notes.md
# Stage 1 untracked file(s)? [y/N]: y
# Saved commit 7a8b9c0 (feat/runner)
# "feat: add runner progress"
#
# Modified:
#   src/main.go +5 -2
#
# New files:
#   notes.md +10
#
# 2 files committed (+15 | -2)
```

### Direct commit with message

```bash
gx save -m "fix(auth): handle expired token redirect"
```

### Commit with pre-staged changes (paired with `gx stage`)

```bash
# Stage specific changes first
gx stage app.js
# Staged 1 file (+12 -3)

# Commit only what is staged
gx save -m "feat: update application entrypoint"
# Saved commit 9f8e7d6 (feat/login)
# "feat: update application entrypoint"
#
# Modified:
#   app.js +12 -3
#
# 1 file committed (+12 | -3)
```

### Stage all modified and untracked files and commit immediately

```bash
gx save -a -m "feat: complete user profile feature"
```

### Commit specific files

```bash
gx save src/user.go src/user_test.go -m "feat(user): add email validation"
```

### Exclude patterns from commit

```bash
# Exclude environment files and temp directories
gx save --exclude ".env*,tmp/**" -m "chore: update dependencies"
```

### Interactive patch mode

```bash
gx save --patch
```

---

## Edge Cases & Recovery

| Scenario                           | Behavior & Recovery                                                                                                                          |
| ---------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| Rebase or merge in progress        | Refuses to save: `A rebase is already in progress.` Resolve or abort with `gx sync --continue` or `git rebase --abort`.                      |
| Pre-existing staged files          | Commits already-staged changes directly without re-scanning. Pass `-a` / `--all` if you also want to stage and commit remaining unstaged changes. |
| Sensitive file detected            | Hard block before staging (`refusing to stage`) and before committing (`refusing to commit`). Remove file, gitignore it, or use `--allow-sensitive`. |
| Sensitive file in `--patch`        | Non-fatal warning is displayed; user can skip sensitive hunks interactively.                                                                 |
| Nothing staged                     | Prompts: `Nothing staged — commit anyway? [y/N]:`. Answer `y` to create empty commit, or pass `--allow-empty`. Hint: `Stage files first with gx stage, or use --allow-empty to commit with nothing staged.` |
| Empty or whitespace commit message | Error: `Commit message cannot be empty.` Provide meaningful message with `-m` or omit `-m` to open editor.                                   |
| Unresolved conflict markers staged | Warning with list of files containing conflict markers; prompts before allowing commit.                                                      |
| Submodule paths detected           | Submodule directory is skipped from staging with a notice to use manual `git add`.                                                           |
| Detached HEAD                      | Allowed. Commits are created directly on detached HEAD.                                                                                      |
| Outside a Git repository           | Blocked by central guard: `Not inside a Git repository.`                                                                                     |
| Missing configuration              | Blocked by central guard: run `gx init` first.                                                                                               |
