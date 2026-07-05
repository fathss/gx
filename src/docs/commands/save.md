# gx save

Stage changes and create a commit.

## Usage

```
gx save [-m <msg>] [--patch] [--allow-sensitive] [--allow-empty] [--exclude <pattern>] [<files>...]
```

When no file arguments are given, `gx save` scans the working tree, shows a
categorized preview of modified and untracked files, prompts for confirmation
before staging untracked files, stages the changes, shows a diffstat, and
opens the editor for a commit message.

If file arguments are given, only those files are staged and the untracked
prompt is skipped. This allows precise control over what goes into the
commit:

```bash
gx save                                # scan + categorize + prompt
gx save src/foo.go                     # stage only one file
gx save src/foo.go src/bar.go         # stage specific files
```

Use `--exclude` to skip files matching glob patterns (repeatable, comma-separated). This is applied before staging and before the sensitive-pattern check, so excluded files are neither staged nor scanned for sensitive content:

---

## Flags

| Flag                | Purpose                                                   |
| ------------------- | --------------------------------------------------------- |
| `-m`, `--message`   | Commit with the given message (skip editor).              |
| `--patch`           | Select hunks interactively via `git add -p`.              |
| `--allow-sensitive` | Override sensitive-pattern blocking.                      |
| `--allow-empty`     | Allow committing with nothing staged (skip confirmation). |
| `--exclude`         | Glob patterns for files to exclude from staging (repeatable, comma-separated). |

---

## Flow

### Pre-flight

Before scanning the working tree, `gx save` checks that no rebase or merge
is in progress. If one is, it exits with an error:

```
✗ A rebase is already in progress.
hint: Resolve or abort the rebase first.
hint: Use gx sync --continue or git rebase --abort/--continue.
```

Detached HEAD is **not** blocked — saving on a detached HEAD is allowed.

### Categorized preview

When run without file arguments, `gx save` prints the working tree state:

```
Modified tracked files:
  src/main.go
  src/auth.go

New untracked files:
  notes.md

Stage 1 untracked file(s)? [y/N]:
```

- **Modified tracked files** are listed first and staged freely. These are
  already known to Git, so the risk of accidental staging is low.
- **New untracked files** are listed separately and require explicit
  confirmation (`y/N`). Default is no. This prevents unintended additions
  like scratch files or generated artifacts.

When file arguments are given, this preview is skipped and only the
specified files are staged.

### Sensitive file protection

If any file to be staged matches a sensitive pattern, `gx save` **blocks
with an error** (not just a warning):

```
✗ Sensitive file(s) detected — refusing to stage:
  .env matched .env
  config/keys.pem matched *.pem
hint: Remove or gitignore the files, or use --allow-sensitive to override.
```

Default patterns written by `gx init`: `.env`, `*.pem`, `*secret*`, `*.key`

Additional patterns can be configured under `sensitivePatterns` in
`.gx/config`:

```json
{ "sensitivePatterns": [".id_rsa", "*credentials*"] }
```

A one-time override is available via `--allow-sensitive`. This is a flag,
not a config setting — the block cannot be disabled permanently.

In `--patch` mode, sensitive files trigger a **warning** rather than a
block — the user sees which files matched and is told they can skip
those hunks during interactive staging. Use `--allow-sensitive` to
suppress the warning.

The `--exclude` flag is applied **before** the sensitive-pattern check —
excluded files are never scanned for sensitive content.

### Diffstat

After staging, `git diff --cached --stat` is printed so the developer sees
exactly what is about to be committed:

```
▸ git diff --cached --stat
 src/main.go | 5 +++++
 src/auth.go | 3 +++
 2 files changed, 8 insertions(+)
```

### Empty commit guard

If nothing is staged, `gx save` prompts before creating an empty commit:

```
Nothing staged — commit anyway? [y/N]:
```

If declined, it exits with a hint:

```
✗ Save aborted.
hint: Stage files first, or use --allow-empty to commit with nothing staged.
```

Use `--allow-empty` to skip the prompt entirely.

### Commit

If `-m <msg>` is given, the commit is created with that message.
Otherwise, the configured editor opens.

---

## Edge cases

| Condition                          | Behaviour                                                              |
| ---------------------------------- | ---------------------------------------------------------------------- |
| Not a Git repository               | Central guard blocks: "Not inside a Git repository."                   |
| No `.gx/config`                    | Central guard blocks: "run gx init to start using gx"                  |
| Rebase in progress                 | Block: "A rebase is already in progress." Hint to abort or continue.   |
| Merge in progress                  | Block: "A merge is already in progress." Hint to abort or continue.    |
| Detached HEAD                      | Allowed — commit is created on detached HEAD.                          |
| Sensitive file detected            | Block with file list. `--allow-sensitive` overrides.                   |
| No files to stage                  | Prompt "Nothing staged — commit anyway?" `--allow-empty` skips prompt. |
| `--patch` with file arguments      | Files are passed to `git add -p`. Sensitive files trigger a warning (not a block). |
| `--allow-sensitive` with `--patch` | Suppresses the sensitive-file warning in patch mode.                   |
| `--patch` with `--exclude`         | Excluded files are filtered before `git add -p` is called.             |
| Commit fails (hook, auth, editor)  | Error with hint: "Check the error above. Your changes remain staged." (if staged) or "Check the error above. You may try again with gx save." (if not). |
| Diffstat fails                     | Non-fatal — says "(could not compute diffstat)" and proceeds.          |
| File arg does not exist            | `git add` error propagates from git.                                   |
| `--exclude` filters all provided files | `hasSpecificFiles` resets to false, falls through to empty-commit guard. |
| `--patch` with no staged content   | Commit guard prompts before creating empty commit.                     |
| Empty/whitespace `-m` message      | Error: "Commit message cannot be empty."                               |
| Unresolved conflict markers staged | Warning with file list, prompts to confirm before committing.          |
| Submodule paths detected           | Skipped from staging with a `[submodule]` notice — use `git add` manually. |
| Partial staging already in place   | Warns: "Some files are already staged." Prompts before overwriting.    |

---

## User-visible output

Every git command prints `▸ git <args>` followed by git's own output (via
`Run()`). Plumbing commands (`Status`, `NonEmptyStatus`, `MatchSensitivePatterns`)
use `Output()` and are hidden by default; pass `--verbose` to show them.

gx-specific messages:

| Message                                           | Trigger                                      |
| ------------------------------------------------- | -------------------------------------------- |
| `Modified tracked files:` + listing               | Working tree scan (no file args)             |
| `New untracked files:` + listing                  | Working tree scan (no file args)             |
| `Stage N untracked file(s)? [y/N]:`               | Untracked files found, awaiting confirmation |
| `Sensitive file(s) detected — refusing to stage:` | Sensitive-pattern match blocked              |
| `(could not compute diffstat)`                    | Diffstat failed (non-fatal)                  |
| `Nothing staged — commit anyway? [y/N]:`          | Empty commit guard prompt                    |
| `Save aborted.`                                   | User declined empty commit                   |
| `Commit failed.`                                  | `git commit` returned non-zero               |

### Normal success

```
Modified tracked files:
  src/main.go

New untracked files:
  (none)

▸ git add src/main.go
▸ git diff --cached --stat
 src/main.go | 5 +++++
 1 file changed, 5 insertions(+)
▸ git commit
```

### With untracked files

```
Modified tracked files:
  src/main.go

New untracked files:
  notes.md

Stage 1 untracked file(s)? [y/N]: y
▸ git add src/main.go notes.md
▸ git diff --cached --stat
 src/main.go | 5 +++++
 notes.md    | 10 ++++++++++
 2 files changed, 15 insertions(+)
▸ git commit
```

### With `-m` flag

```
Modified tracked files:
  src/main.go

New untracked files:
  (none)

▸ git add src/main.go
▸ git diff --cached --stat
 src/main.go | 5 +++++
 1 file changed, 5 insertions(+)
▸ git commit -m "fix: login bug"
```

### Sensitive file blocked

```
Modified tracked files:
  .env

New untracked files:
  (none)

✗ Sensitive file(s) detected — refusing to stage:
  .env matched .env
hint: Remove or gitignore the files, or use --allow-sensitive to override.
```

---

## For developers

| File                        | Contents                                                 |
| --------------------------- | -------------------------------------------------------- |
| `cmd/save.go`               | Cobra command, flags, config loading                     |
| `internal/workflow/save.go` | Categorized scan, sensitive check, stage, commit         |
| `internal/git/status.go`    | `Status()`, `NonEmptyStatus()`                           |
| `internal/git/stage.go`     | `Add()`, `AddPatch()`                                    |
| `internal/git/diff.go`      | `DiffStatCached()`                                       |
| `internal/git/commit.go`    | `Commit()`, `CommitEditor()`, `CommitAllowEmpty()`, `CommitAllowEmptyEditor()` |
| `internal/git/patterns.go`  | `MatchSensitivePatterns()`, `FilterExcluded()` |
| `internal/config/config.go` | `Load()`, `SensitivePatterns` field, `DefaultSensitivePatterns` |
| `tests/gx-save-test.sh`     | Bash integration tests                                   |
