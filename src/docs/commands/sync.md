# gx sync

Synchronize the current branch with the configured base branch.

## Usage

```
gx sync [<remote> [<base_branch>]]
```

Positional args override the corresponding config values for a single invocation:

```bash
gx sync                      # uses config values (requires .gx/config)
gx sync origin               # remote overridden, base from config
gx sync origin develop       # remote and base overridden
```

When no config file exists and no positional args are given, `gx sync` exits with instructions to run `gx init`.

The equivalent manual sequence (default config) is:

```bash
git stash          # only if working tree is dirty
git fetch origin
git checkout develop
git merge --ff-only origin/develop
git checkout feature
git rebase develop
git stash pop      # only if stashed earlier
```

---

## Configuration

| Field           | Used for                     | Default   |
| --------------- | ---------------------------- | --------- |
| `defaultBranch` | Which branch to sync against | `develop` |
| `remote`        | Remote to fetch from         | `origin`  |
| `syncStrategy`  | `rebase` or `merge`          | `rebase`  |

## Flags

| Flag         | Purpose                                                       |
| ------------ | ------------------------------------------------------------- |
| `--continue` | Continue a paused gx-orchestrated rebase or merge             |
| `--abort`    | Abort a paused gx-orchestrated rebase or merge                |
| `--rebase`   | Override config — use rebase strategy                         |
| `--merge`    | Override config — use merge strategy                          |
| `--skip`     | Skip the current commit during a paused rebase (Q4 only)      |

---

## Flow

### Pre-flight checks (runs first)

Before any operation, the central guard in `PersistentPreRunE` checks:

1. **Inside a git repo?** — if not, blocks with `✗ Not inside a Git repository.`
2. **Config file exists?** — if not, blocks with `✗ No existing configuration\nhint: run gx init to start using gx`

If both pass, the sync state machine runs its own guards for rebase/merge state and stash entries. A stash created by gx sync (label starting with `gx-sync/`) signals that gx orchestrated the paused operation.

| State | Rebase in progress? | Merge in progress? | GX stash exists? | Behaviour                                                 |
| ----- | ------------------- | ------------------ | ---------------- | --------------------------------------------------------- |
| Q1    | no                  | no                 | no               | Normal flow (continue)                                    |
| Q2    | no                  | no                 | yes              | Warn orphaned stash, proceed normally                     |
| Q3    | yes                 | no                 | no               | **Block** — manual, gx refuses to touch                   |
| Q4    | yes                 | no                 | yes              | **Block** — gx-owned, pass `--continue` or `--abort` |
| Q5    | no                  | yes                | no               | **Block** — manual, gx refuses to touch                   |
| Q6    | no                  | yes                | yes              | **Block** — gx-owned, pass `--continue` or `--abort` |

All paused states (Q3–Q6) exit with an error on plain `gx sync`. Q4 accepts `--continue` to resume, `--skip` to skip the current commit, or `--abort` to cancel. Q6 accepts `--continue` or `--abort`. Q3 and Q5 are manual operations — gx sync only manages its own sync operations, so you must use git commands directly.

State Q2 prints a warning but proceeds normally.

### Normal flow (Q1 / Q2 only)

```
1. Get current branch name
2. Refuse if: detached HEAD
3. Stash dirty working tree (label: gx-sync/<branch>/<timestamp>)
4. git fetch <remote>
5. If on base branch: git merge --ff-only → pop stash
6. Otherwise:
   a. git checkout <base>
   b. git merge --ff-only <remote>/<base>
   c. git checkout <current>
   d. git rebase <base> (default) or git merge <base>
   e. Pop stash
```

### On conflict (first run)

The rebase or merge is **left in progress**. The stash is **preserved** (`gx-sync/<branch>/<timestamp>`). Neither is aborted or restored — the user resolves conflicts and re-runs `gx sync`.

Error message includes the conflicted commit info and file list:

```
✗ Rebase stopped because of conflicts while applying commit 'Fix login bug' (abc1234). Conflicted files: src/auth.go
hint: Resolve the conflicts, stage the files, then run gx sync again. Your changes were stashed and will be restored automatically.
```

### On re-run after resolving conflicts (Q4 / Q6)

gx detects the rebase/merge is still paused and a gx stash exists. Plain `gx sync` blocks — pass `--continue` to resume:

```
Continuing rebase — applying 'Fix login bug' (abc1234)...
Restoring stashed changes...
```

If there are still unresolved conflicts:

```
Continuing rebase — applying 'Fix login bug' (abc1234)...
✗ There are still unresolved conflicts while applying commit 'Fix login bug' (abc1234). Conflicted files: src/auth.go
hint: Resolve the remaining conflicts, stage the files, then run gx sync --continue again.
```

### Manual rebase / merge (Q3 / Q5)

If a rebase or merge was started manually (no gx stash), gx blocks both plain `gx sync` and `gx sync --continue`:

```
✗ A rebase is already in progress.
hint: gx sync only manages its own sync operations.
hint: Use git rebase --continue, --skip, or --abort directly.
```

### Aborting with `--abort`

Pass `--abort` to abort a paused gx-orchestrated rebase or merge:

```bash
gx sync --abort
```

`--abort` only works on gx-owned operations (Q4, Q6). Manual operations (Q3, Q5) are blocked:

| State | Scenario                                | Behaviour                          |
| ----- | --------------------------------------- | ---------------------------------- |
| Q3    | Manual rebase in progress (no stash)    | **Block** — "gx sync only manages" |
| Q4    | Gx-orchestrated rebase in progress      | `git rebase --abort` → pop stash   |
| Q5    | Manual merge in progress (no stash)     | **Block** — "gx sync only manages" |
| Q6    | Gx-orchestrated merge in progress       | `git merge --abort` → pop stash    |

If nothing is in progress (Q1 or Q2), `--abort` exits with an error:

```
✗ Nothing to abort.
hint: Run gx sync to start a new sync.
```

Abort output for gx-owned operations:

```
Aborting rebase...
▸ git rebase --abort
Restoring stashed changes...
▸ git stash pop
```

`--abort` is mutually exclusive with `--continue`, `--skip`, `--rebase`, and `--merge`.

### Orphaned gx stash (Q2)

If a gx stash entry exists but no rebase or merge is paused, gx restores
the stash and proceeds normally:

```
Restoring stashed changes from previous sync...
▸ git stash pop
```

---

## Edge cases

| Condition                                   | Behaviour                                                         |
| ------------------------------------------- | ----------------------------------------------------------------- |
| Not a git repository                        | Central guard blocks before RunE: "Not inside a Git repository."  |
| Detached HEAD                               | Error: "Detached HEAD is not supported. Checkout a branch first." |
| Rebase already in progress (manual, Q3)     | Block. `--continue` refused, use git directly.                    |
| Merge already in progress (manual, Q5)      | Block. `--continue` refused, use git directly.                    |
| Rebase already in progress (gx-owned, Q4)   | Block. `--continue` or `--abort` accepted.                        |
| Merge already in progress (gx-owned, Q6)    | Block. `--continue` or `--abort` accepted.                        |
| Fetch fails (offline, auth)                 | Error + hint. Stash is restored.                                  |
| Checkout fails (dirty tree, missing branch) | Error + hint. Stash is restored.                                  |
| Fast-forward fails (diverged base)          | Error + hint. Stash restored, original branch restored.           |
| Rebase conflict (first run)                 | Leave paused. Stash preserved. Show commit + files.               |
| Merge conflict (first run)                  | Leave paused. Stash preserved. Show files.                        |
| `--continue` with nothing paused            | Error: "Nothing to continue."                                     |
| `--abort` with nothing paused               | Error: "Nothing to abort."                                        |
| `--abort` + `--continue` together           | Error: "Cannot specify both --abort and --continue."              |
| `--rebase` + `--merge` together             | Error: "Cannot specify both --rebase and --merge."                |
| Abort rebase (Q4)                           | `git rebase --abort`, pop stash. Exit 0.                          |
| Abort merge (Q6)                            | `git merge --abort`, pop stash. Exit 0.                           |
| `--abort` on manual rebase/merge (Q3, Q5)   | Block. "gx sync only manages its own sync operations."            |
| `--skip` during gx-orchestrated rebase (Q4) | Skip current commit, continue or finish rebase. Pop stash when done. |
| `--skip` during merge (Q6)                  | Block: "--skip is not valid during a merge."                      |
| `--skip` + `--continue` together            | Error: "Cannot specify both --skip and --continue."               |
| `--skip` + `--abort` together               | Error: "Cannot specify both --skip and --abort."                  |
| `--abort` + `--rebase` together             | Error: "Cannot specify both --abort and --rebase."                |
| `--abort` + `--merge` together              | Error: "Cannot specify both --abort and --merge."                 |
| `--continue` + `--rebase` together          | Error: "Cannot specify both --continue and --rebase."             |
| `--continue` + `--merge` together           | Error: "Cannot specify both --continue and --merge."              |
| `--continue`/`--skip` with nothing paused   | Error: "Nothing to continue. Rebase appears to have completed already." |
| Empty commit after conflict resolution      | Warns and suggests --skip or --abort.                             |
| Remote default branch has changed           | Warning printed with command to update config.                    |
| Orphaned gx stash cleanup                   | Stash dropped on successful sync. Warning on failure.             |
| Stash pop fails (auto-continue)             | Error with hint. Stash stays safe on stack.                       |

---

## User-visible output

Every git command prints `▸ git <args>` followed by git's own stdout/stderr (via `Run()`). Plumbing commands (`CurrentBranch`, `IsRebaseInProgress`, etc.) use `Output()` and are hidden by default; pass `--verbose` to show them. The central pre-flight checks (`IsRepository`, `Exists`) are always silent.

Examples below assume default config (`remote: origin`, `defaultBranch: develop`). Actual values depend on `.gx/config`.

gx-specific messages:

| Message                                        | Trigger                                |
| ---------------------------------------------- | -------------------------------------- |
| `Stashing local changes...`                    | Dirty working tree before sync         |
| `Restoring stashed changes...`                 | After successful sync or auto-continue |
| `Restoring stashed changes from previous sync...` | Q2 — orphaned gx stash restored before proceeding |
| `Continuing rebase — applying <commit>...`     | Q4 auto-continue                       |
| `Continuing rebase...`                         | Q4 auto-continue (no patch info)       |
| `Continuing merge...`                          | Q6 auto-continue                       |
| `Aborting rebase...`                           | Q4 `--abort`                            |
| `Aborting merge...`                            | Q6 `--abort`                            |

### Normal success (clean tree)

```
▸ git fetch origin
▸ git checkout develop
▸ git merge --ff-only origin/develop
▸ git checkout feature/login
▸ git rebase develop
```

### With stash (dirty tree)

```
Stashing local changes...
▸ git stash push -m gx-sync/feature/login/1741234567
▸ git fetch origin
▸ git checkout develop
▸ git merge --ff-only origin/develop
▸ git checkout feature/login
▸ git rebase develop
Restoring stashed changes...
▸ git stash pop
```

### gx sync --continue (Q4 / Q6)

```
Continuing rebase — applying 'Fix login bug' (abc1234)...
▸ git rebase --continue
Restoring stashed changes...
▸ git stash pop
```

---

## For developers

| File                         | Contents                                                                                                                 |
| ---------------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| `cmd/sync.go`                | Cobra command, flags (`--continue`, `--abort`, `--rebase`, `--merge`), config loading                                     |
| `internal/workflow/sync.go`  | State machine (Q1-Q6), normal flow, continue/abort helpers                                                                |
| `internal/git/branch.go`     | `CurrentBranch()`, `Checkout()`                                                                                          |
| `internal/git/remote.go`     | `Fetch()`, `FastForward()`                                                                                               |
| `internal/git/rebase.go`     | `Rebase()`, `RebaseAbort()`, `RebaseContinue()`, `RebaseSkip()`, `IsRebaseInProgress()`, `CurrentRebasePatchInfo()`        |
| `internal/git/merge.go`      | `Merge()`, `MergeContinue()`, `MergeAbort()`, `IsMergeInProgress()`, `ConflictedFiles()`                                  |
| `internal/git/stash.go`      | `StashPush()`, `StashPop()`, `IsClean()`, `HasGXStash()`                                                                 |
| `internal/git/repository.go` | `IsRepository()`                                                                                                         |
| `internal/workflow/guard.go` | `RequireRepository()`, `RequireConfig()` — central pre-flight                                                            |
| `internal/config/config.go`  | `Exists()`, `Load()`, `Save()`                                                                                           |
| `tests/gx-sync-test.sh`      | Bash integration tests                                                                                                   |
