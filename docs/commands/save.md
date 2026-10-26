# gx save

Stage changes and create a commit with safety checks for sensitive files and empty commits.

---

## Usage

```bash
gx save [-m <msg>] [--patch] [--allow-sensitive] [--allow-empty] [--exclude <pattern>] [<files>...]
```

`gx save` automates staging and committing with guardrails:

- **Full working tree scan**: When run without file arguments, scans the repository and shows categorized previews of modified tracked files and new untracked files.
- **Untracked files prompt**: Asks for confirmation before staging untracked files (`[y/N]`, default No).
- **Specific files**: Passing file arguments bypasses untracked confirmation and stages only the named files.
- **Sensitive file protection**: Refuses to stage files matching configured sensitive patterns (e.g., `.env`, `*.pem`, `*secret*`, `*.key`) unless overridden with `--allow-sensitive`.
- **Hunk staging**: `--patch` runs interactive staging via `git add -p`.

---

## Flags

| Flag                  | Shorthand | Description                                                          |
| --------------------- | --------- | -------------------------------------------------------------------- |
| `--message <msg>`     | `-m`      | Commit message (skips editor).                                       |
| `--patch`             |           | Interactively select hunks to stage (`git add -p`).                  |
| `--allow-sensitive`   |           | Override sensitive-pattern block or suppress warnings.               |
| `--allow-empty`       |           | Allow committing with nothing staged (skips empty guard prompt).     |
| `--exclude <pattern>` |           | Glob patterns to exclude from staging (repeatable, comma-separated). |
| `--verbose`           | `-v`      | Show underlying git plumbing commands.                               |

---

## Examples

### Interactive commit (scan, review, stage, commit)

```bash
gx save
# Modified tracked files:
#   src/main.go
# New untracked files:
#   notes.md
# Stage 1 untracked file(s)? [y/N]: y
# ▸ git add src/main.go notes.md
# ▸ git diff --cached --stat
#  src/main.go | 5 +++++
#  notes.md    | 10 ++++++++++
#  2 files changed, 15 insertions(+)
# ▸ git commit
```

### Direct commit with message

```bash
gx save -m "fix(auth): handle expired token redirect"
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

| Scenario                           | Behavior & Recovery                                                                                                          |
| ---------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| Rebase or merge in progress        | Refuses to save: `A rebase is already in progress.` Resolve or abort with `gx sync --continue` or `git rebase --abort`.      |
| Sensitive file detected            | Hard block: `Sensitive file(s) detected — refusing to stage`. Add to `.gitignore`, remove file, or pass `--allow-sensitive`. |
| Sensitive file in `--patch`        | Non-fatal warning is displayed; user can skip sensitive hunks interactively.                                                 |
| Nothing staged                     | Prompts: `Nothing staged — commit anyway? [y/N]:`. Answer `y` to create empty commit, or pass `--allow-empty`.               |
| Empty or whitespace commit message | Error: `Commit message cannot be empty.` Provide meaningful message or omit `-m` to open editor.                             |
| Unresolved conflict markers staged | Warning with list of files containing conflict markers; prompts before allowing commit.                                      |
| Submodule paths detected           | Submodule directory is skipped from staging with a notice to use manual `git add`.                                           |
| Detached HEAD                      | Allowed. Commits are created directly on detached HEAD.                                                                      |
| Pre-existing staged files          | Warns if staging is already partially populated and confirms before proceeding.                                              |
