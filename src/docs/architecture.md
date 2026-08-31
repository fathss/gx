# Architecture

gx follows a strict layered architecture:

```
cmd  →  internal/workflow  →  internal/git  →  internal/runner
```

Each layer has a specific job and **must not** reach across boundaries.

---

## Layer responsibilities

### `cmd/` — entrypoints

**Can:**
- Define cobra commands and flags
- Define `no_repo` and `no_config` annotations to opt out of central pre-flight checks
- Load config and pass it to the workflow
- Call exactly one workflow function

**Cannot:**
- Execute git commands directly
- Contain workflow/business logic

**Exception — `gx config`** reads/writes `.gx/config` directly via the `config` package with no workflow or git layer. Only command that bypasses the normal architecture.

### `internal/workflow/` — orchestration

**Can:**
- Combine git operations into user-facing features
- Convert every error into `cli.Error{Message, Hint}`

**Cannot:**
- Call `exec.Command` or invoke git directly
- Import or know about cobra internals

### `internal/git/` — one function per git operation

**Can:**
- Wrap runner calls into named functions (`Fetch`, `Rebase`, etc.)
- Return raw errors from the runner

**Cannot:**
- Print messages to the terminal
- Know about workflows or create `cli.Error`

### `internal/runner/` — git execution

**Can:**
- Execute `git` subprocesses
- Log verbose output when `--verbose` is set
- Pipe stdout/stderr to the terminal

**Cannot:**
- Understand what the git commands mean
- Implement business logic

### `internal/config/`

- Loads and saves `.gx/config` from the current directory
- Falls back to defaults if the file is missing
- Provides `Set`, `Get`, `Exists`, and `Default()` helpers, plus the defaults `DefaultSensitivePatterns` and `DefaultProtectedBranches`
- Provides list helpers `AppendSensitivePatterns`/`SetSensitivePatterns` and `AppendProtectedBranches`/`SetProtectedBranches` — used by `gx config` (`cmd/config.go`)
- `gx init` seeds `sensitivePatterns` with `DefaultSensitivePatterns` (used by `cmd/config.go` and `internal/workflow/guard.go`)

### `internal/cli/`

- Defines the `Error{Message, Hint}` type and `Warn{Message, Hint}` — a non-fatal warning struct
- Only `main.go` prints errors — no other package

### `internal/forge/`

- Detects the remote git host from a `RemoteURL` (`forge.go`, `github.go`, `gitlab.go`, `bitbucket.go`, `url.go`)
- Builds PR/MR create + existing-PR discovery URLs via the `Forge` interface (`PRCreateURL`, `ExistingPRURL`)
- Consumed by `internal/workflow/ship.go` after a successful push; forge errors never fail the ship flow since the push already completed
- See `docs/domains/forge.md`

---

## Error handling flow

```
runner  →  raw exec errors (never wrapped)
  ↓
git     →  raw git errors (never wrapped, never creates cli.Error)
  ↓
workflow →  cli.Error{Message, Hint}  ← every external failure becomes this
  ↓
cmd     →  passes error up to main
  ↓
main    →  prints the error via printError() — only main.go renders errors
```

See `docs/error-handling.md` for detailed rules per layer.

---

## Central pre-flight guard

Every command passes through `PersistentPreRunE` in `cmd/root.go` before its `RunE` fires. The guard checks two conditions in order:

1. **Repo required** — if the leaf command does NOT have `no_repo` annotation, `RequireRepository()` blocks outside git repos.
2. **Config required** — if the leaf command does NOT have `no_config` annotation, `RequireConfig()` blocks without `.gx/config`.

Commands opt out with annotations:
```go
Annotations: map[string]string{"no_config": "", "no_repo": ""},
```

| Annotation | Meaning |
|---|---|
| `no_config` | Works without `.gx/config` (e.g. `init`, `config`) |
| `no_repo` | Works outside a git repo (e.g. `help`, `completion`) |

The guard logic lives in `internal/workflow/guard.go` (two functions: `RequireRepository`, `RequireConfig`). Workflows no longer check `IsRepository` or `Exists` individually — the guard handles it centrally.

Built-in commands (`help`, `completion`) are annotated dynamically in `sync.Once` inside `PersistentPreRunE`. The `annotateSubtree` function recurses into all sub-commands so leaf commands like `completion bash` are also covered.

---

## Runner methods

| Method | Visibility | Use case |
|---|---|---|
| `Run(args...) error` | Always prints `▸ git <args>` | User-visible git operations (fetch, rebase, commit, etc.) |
| `RunWithEnv(env, args...) error` | Always prints header | Git operations needing extra env vars (`GIT_EDITOR=true`) |
| `Output(args...) (string, error)` | Printed only with `--verbose` | Plumbing — capturing output (current branch, porcelain status, etc.) |
| `CombinedOutput(args...) (string, error)` | Printed only with `--verbose` | Plumbing — capturing both stdout and stderr |
| `Warn(msg string)` / `Warnf(msg, hint string)` | Via `WarnFn` | Emit a non-fatal warning through the wired handler |

The `Runner` exposes a `WarnFn` (type `WarnFunc func(msg, hint string)`) wired by `cmd/root.go` `initRunner` to print a `⚠` warning glyph plus hint lines to stderr.

`--verbose` / `-v` flag prints the plumbing commands (`Output()`/`CombinedOutput()`) that are hidden by default. The `▸ git <args>` headers from `Run()` / `RunWithEnv()` are always-on.

---

## Workflow files

| File | Contents |
|---|---|---|
| `internal/workflow/guard.go` | `RequireRepository()`, `RequireConfig()` — central pre-flight |
| `internal/workflow/init.go` | `Init()` — auto-detect remote/branch, write config |
| `internal/workflow/sync.go` | `Sync()` — state machine (Q1–Q6), `autoContinueSync()`, `autoContinueMergeSync()`, `abortWithStashPop()`, `autoSkipSync()` |
| `internal/workflow/save.go` | `Save()` — categorized preview, sensitive check, stage, commit, `promptConfirm()` |
| `internal/workflow/ship.go` | `Ship()` — protected-branch check, divergence check, push, PR-url orchestration |
| `internal/workflow/status.go` | `Status()` — assemble six-section snapshot, `upstreamRef()`, `parseUpstreamRef()` |

---

## Git domain files

| File | Operations |
|---|---|---|
| `branch.go` | `CurrentBranch`, `Checkout`, `LocalBranchExists`, `GetHEADState`, `ShortHeadHash`, `HEADState` |
| `commit.go` | `Commit`, `CommitEditor`, `CommitAllowEmpty`, `CommitAllowEmptyEditor` |
| `detect.go` | `DetectRemote`, `DetectDefaultBranch` |
| `diff.go` | `DiffStatCached`, `HasStagedChanges`, `DiffUnstagedFiles`, `CheckCachedDiff` |
| `divergence.go` | `AheadBehind` |
| `log.go` | `RecentCommits`, `CommitInfo` |
| `merge.go` | `Merge`, `IsMergeInProgress`, `MergeContinue`, `MergeAbort`, `ConflictedFiles` |
| `patterns.go` | `MatchSensitivePatterns`, `FilterExcluded`, `SanitizePatterns` |
| `rebase.go` | `Rebase`, `RebaseAbort`, `RebaseContinue`, `RebaseSkip`, `IsRebaseInProgress`, `CurrentRebasePatchInfo` |
| `remote.go` | `Fetch`, `FastForward`, `RemoteExists`, `RemoteBranchExists`, `RemoteHEAD`, `Push`, `PushSetUpstream`, `PushForceWithLease`, `HasUpstream`, `RemoteURL` |
| `repository.go` | `IsRepository` |
| `stage.go` | `AddAll`, `Add`, `AddPatch` |
| `stash.go` | `StashList`, `StashPush`, `StashPop`, `findStashByPrefix`, `DropGXStash`, `IsClean`, `HasGXStash` |
| `status.go` | `Status`, `NonEmptyStatus`, `UntrackedFiles`, `SubmodulePaths`, `GetParsedStatus`, `FormatStagedLabel`, `StatusLabels`, `ParsedStatus` |

---

## Config (`.gx/config`)

```json
{ "remote": "origin", "defaultBranch": "develop", "syncStrategy": "rebase", "sensitivePatterns": [".env", "*.pem", "*secret*", "*.key"], "protectedBranches": ["main", "master", "develop"] }
```

| Field | Default | Used by |
|---|---|---|
| `remote` | `origin` | `sync` — remote to fetch from |
| `defaultBranch` | `develop` | `sync` — branch to sync against |
| `syncStrategy` | `rebase` | `sync` — `rebase` or `merge` |
| `sensitivePatterns` | `[]` (seeded by `gx init`) | `save` — glob patterns for sensitive file detection |
| `protectedBranches` | `["main", "master", "develop"]` | `ship` — branches protected from direct push |

Missing file = `RequireConfig()` guard blocks most commands with "run gx init". The `config` package (`Load()`) falls back to defaults when the file is missing — only `gx config` and `gx init` use this path (both annotated `no_config`). Empty fields in file = fall back to defaults.

---

## Adding a new command: `gx foo`

You need three files:

| File | What goes in it |
|---|---|
| `cmd/foo.go` | Cobra command definition, load config, call workflow |
| `internal/workflow/foo.go` | Orchestrate git ops, return `cli.Error` on failure |
| `docs/commands/foo.md` | User-facing docs for the command |

**Exception — `gx config`** bypasses the workflow layer (reads/writes `.gx/config` via the `config` package directly).

If the command introduces a **new git capability** (e.g. pushing, tagging, stashing), also add:

| File | What goes in it |
|---|---|
| `internal/git/push.go` | Git operations (one function per git command) |
| `docs/domains/push.md` | Docs for developers extending that domain |

Existing commands reuse existing domain files. For example:
- `gx ship` reuses `remote.go` (push functions live there) and adds `divergence.go`
- `gx resolve` would add `conflict.go`
- `gx tag` would add `tag.go`

---

## Safety contract

Every `gx` command that mutates the repo follows this order:

1. **check** — validate preconditions (clean state, right branch, semver format, etc.)
2. **warn** — surface anything unusual before acting (sensitive files, force-push, not on base branch)
3. **act** — execute git operations
4. **recover** — on any failure, undo partial state and exit with a clear message

No command leaves the repo in a silent broken state.

---

## Output philosophy

`Run()` always prints `▸ git <args>` to visually group each git command's output. Gx's own `fmt.Print` messages are limited to things git doesn't already say: stash context, conflict summaries with recovery hints. No "Done." footer or unnecessary announcements.

---

## Sync state machine

See `docs/commands/sync.md` for the full Q1–Q6 state machine. Key signals:

| Signal | Detection |
|---|---|
| `RebaseInProgress` | `os.Stat` on `.git/rebase-merge` or `.git/rebase-apply` |
| `MergeInProgress` | `git rev-parse -q --verify MERGE_HEAD` |
| `HasGXStash` | `git stash list` parsed for `gx-sync/` prefix |

The gx stash presence is the signal distinguishing gx-orchestrated pauses from manual ones.

---

## Running tests

```bash
# Single command suite
bash tests/gx-ship-test.sh

# All suites (builds gx once, reports per-suite failures)
bash tests/run-all.sh
```

New suites are auto-discovered — any `tests/*-test.sh` file is picked up.
