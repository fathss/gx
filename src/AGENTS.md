# gx — AGENTS.md

Compact, high-signal facts for an agent working on this repo.

## Project

Opinionated Git CLI wrapping common workflows into single commands. Go module `github.com/fathss/gx`.

### Design boundary

gx is NOT a git wrapper. It does not aim to replace `git ` as the universal entrypoint.
gx's job is to combine multiple git commands into single safe, opinionated workflows
(`gx sync`, `gx save`, `gx ship`, etc.). When you need raw git — for edge cases,
manual operations, or anything gx doesn't own — use `git ` directly.

This means:

- No catch-all pass-through (`gx pull` will never work)
- No `gx git` subcommand
- `gx rebase`, `gx merge`, `gx stash` wrappers are not planned
- Hints tell users to use `git <cmd>` when gx can't handle the operation

**Dependencies**: `spf13/cobra` (CLI). Test framework: bash scripts (`tests/lib.sh`).

## Build

```bash
go build -o gx .
go vet ./...
```

## Strict layered architecture

```
cmd/  →  internal/workflow/  →  internal/git/  →  internal/runner/
```

**Layer rules** (violations are bugs):

| Layer                | Must                                                     | Must NOT                                          |
| -------------------- | -------------------------------------------------------- | ------------------------------------------------- |
| `cmd/`               | cobra defs, flags, config loading, invoke workflow       | execute git directly, contain workflow logic      |
| `internal/workflow/` | orchestrate git funcs, convert all errors to `cli.Error` | call `exec.Command`, know cobra internals         |
| `git/`               | one func per git op, thin wrappers over runner           | print messages, know workflow, create `cli.Error` |
| `runner/`            | exec git, verbose logging, pipe stdio                    | understand git, implement logic                   |
| `config/`            | load `.gx/config`, provide defaults                      | —                                                 |
| `cli/`               | `Error{Message, Hint}` type                              | —                                                 |

**Only `main.go` prints errors.** No other package.

**Error pattern** (workflow only):
Hint supports `\n` — main.go splits into separate lowercase `hint:` lines (git-style).

```go
if err := git.SomeOp(...); err != nil {
    return &cli.Error{
        Message: "user-facing message",
        Hint:    "actionable hint",
    }
}
```

**cli.Error output visibility**: Only `Message` and `Hint` are printed to stderr.
**cli.Error.Error()**: Required for `error` interface compliance. Return value (`e.Message`) is never consumed — `main.go` reads `Message` directly via `errors.As`.

## Currently implemented (code)

`gx sync`, `gx save`, `gx ship`, `gx status`, `gx init`, and `gx config` are fully implemented.
Other commands (`resolve`, `start`, `tag`, `undo`, `pr`, `clean`, `log`, `stash`) are planned but not yet implemented.

## Safety contract

Every `gx` command that mutates the repo follows this order:

1. **check** — validate preconditions (clean state, right branch, semver format, etc.)
2. **warn** — surface anything unusual before acting (sensitive files, force-push, not on base branch)
3. **act** — execute git operations
4. **recover** — on any failure, undo partial state and exit with a clear message

No command leaves the repo in a silent broken state.

## Central pre-flight guard

`cmd/root.go` `PersistentPreRunE` validates repo + config before every command:

| Annotation  | Behavior                      |
| ----------- | ----------------------------- |
| `no_repo`   | Skip repo check               |
| `no_config` | Skip config check             |
| (none)      | Requires both repo AND config |

**Adding annotations** — new commands set them in the cobra `Annotations` map:

```go
Annotations: map[string]string{"no_config": ""}
```

**Built-in commands** (`help`, `completion`) are annotated dynamically in `sync.Once` inside `PersistentPreRunE`. Annotates recursively via `annotateSubtree` — annotating only the parent misses leaf commands like `completion bash`.

## Adding a new command

Per architecture docs — `gx foo` requires exactly:

```
cmd/foo.go           # cobra command, loads config, calls workflow
workflow/foo.go      # orchestrates git ops, creates cli.Error
docs/commands/foo.md
```

Only add `internal/git/xxx.go` + `docs/domains/xxx.md` when introducing a brand-new Git capability.

**Exception — `gx config`** reads/writes `.gx/config` directly via the `config` package with no workflow or git layer. Only command that bypasses the normal architecture.

## Config (`.gx/config`)

`gx init` seeds `sensitivePatterns` with `config.DefaultSensitivePatterns`
(`.env`, `*.pem`, `*secret*`, `*.key`) — the config file is the single source
of truth. `MatchSensitivePatterns` does NOT concatenate any hardcoded defaults.

```json
{
  "remote": "origin",
  "defaultBranch": "develop",
  "syncStrategy": "rebase",
  "protectedBranches": ["main", "master", "develop"]
}
```

Missing file = `RequireConfig()` guard blocks most commands with "run gx init". The `config` package (`Load()`) falls back to defaults when the file is missing — only `gx config` and `gx init` use this path (both annotated `no_config`). Empty fields in file = fall back to defaults.

**`gx config` uses `ArbitraryArgs` for `sensitivePatterns` and `protectedBranches`** — variadic positional
patterns (e.g. `gx config sensitivePatterns .env .gitignore`, `gx config protectedBranches main release`). Other keys enforce max 1 value.

## Runner methods

```go
run.Run(args...) error              // prints "▸ git <args>" header, streams git's stdout/stderr to terminal
run.RunWithEnv(env, args...) error  // same as Run but with extra env vars, always prints header
run.Output(args...) (string, error)     // captures stdout, trims whitespace
run.CombinedOutput(args...) (string, error)  // captures both, trims whitespace
```

`--verbose` / `-v` flag prints the plumbing commands (`Output()`/`CombinedOutput()`) that are hidden by default. The `▸ git <args>` headers from `Run()` are always-on.

## Git domain patterns

See `docs/domains/*.md` for per-file function reference. Key conventions:

- Stash ops match by label prefix (`gx-sync/`), never `stash@{0}` — user stash ops shift the stack.
- Rebase/merge detection uses `os.Stat` on `.git/rebase-merge`/`.git/MERGE_HEAD`, not `git rev-parse --show-current-patch` (false negatives).

## Sync state machine

```
Q3: rebaseInProgress + !hasGXStash → manual rebase → block (use git directly)
Q4: rebaseInProgress + hasGXStash  → gx-orchestrated → --continue/--skip/--abort
Q5: mergeInProgress  + !hasGXStash → manual merge → block (use git directly)
Q6: mergeInProgress  + hasGXStash  → gx-orchestrated → --continue/--abort
```

The gx stash presence is the signal. Manual pauses (no gx stash) block with a hint — use git commands directly.

## Output philosophy

`Run()` always prints `▸ git <args>` to visually group each git command's output. Gx's own `fmt.Print` messages are limited to things git doesn't already say: stash context, continue signals, conflict summaries with recovery hints. No "Done." footer, no "Fetching origin..." announcement (git's own output covers that).

## Testing

Shared test library at `tests/lib.sh` with helpers for setup, assertions, and running gx.
One command-specific file per command:

```
tests/lib.sh              # shared helpers (setup_tempdir, run_gx, assertions, ..)
tests/gx-sync-test.sh     # sources lib.sh
tests/gx-save-test.sh     # sources lib.sh
tests/gx-ship-test.sh     # ...
...
```

Run a test with:

```bash
bash tests/gx-COMMAND-test.sh
```

Pattern — source `lib.sh`, run gx, assert on state:

```bash
run_gx
assert_exit_code "exits 0" 0
assert_output_contains "progress message" "rebase"
```

See `tests/gx-sync-test.sh` for a full example.

Bottom "RUN ALL" section wraps each test with `run_test()` (test-level tracking + markers):

```bash
run_test test_01_success_feature "Test 01: success — feature branch"
run_test test_02_outside_repo "Test 02: outside repo"
...
print_test_summary
[ "$TEST_FAIL" -eq 0 ]
```

Shared test library at `tests/lib.sh` (setup, assertions, running gx) —
see the file directly for the full helper list; it's the source of truth
and will drift from any copy kept here. Two non-obvious things:

- `run_test()` wraps each test in a **subshell** — plain `$FAIL`/`$PASS`
  counters won't propagate. Use `$TEST_FAIL`/`$TEST_PASS` for exit decisions.
- `run_gx` provides no stdin — interactive-prompt tests need a separate
  helper that pipes input.

**Assertion rule**: `assert_output_contains(keyword)` — assert the minimum unique
substring that proves the assertion (e.g. `"conflicts"`, `"Git repository"`,
`"Merge stopped"`). Never assert a full error message or sentence. Always pair
with `assert_exit_code` so every test has two independent signals: process
outcome + output content.

**Run all suites**:

```bash
bash tests/run-all.sh   # builds gx once, runs every *-test.sh
```

`run-all.sh` uses `tests/*-test.sh` glob — new test files are auto-discovered without editing `run-all.sh`.

**Run-all failure reporting**: `run_all.sh` captures each suite's output via `tee`, parses `##TEST_FAIL:<label>` markers, and prints which individual tests failed per suite. New test scripts automatically get this reporting if they use `run_test()` wrappers.

**Guard test gotcha**: Tests for outside-repo and inside-repo states have conflicting setup constraints. Each test does `cleanup_tempdir; setup_tempdir` independently — no shared trap can span both states.

## Agent skills

### Issue tracker

Issues live as local markdown files under `.scratch/<feature>/`. See `.agents/docs/issue-tracker.md`.

### Triage labels

Five canonical labels: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `.agents/docs/triage-labels.md`.

## Common agent mistakes (Keep this as the last section)

Running list of traps agents commonly hit in this repo that aren't obvious from
the surrounding docs. Append new entries as they're discovered.

### Assertions & output

- `assert_output_not_contains` keywords that are too short —
  `"Merge"` matched irrelevant git output; `"Merge stopped"` was the minimum
  unique substring. Always verify a not-contains keyword doesn't appear
  elsewhere in the output.

- `assert_output_contains` full sentences instead of keywords —
  Wording changes to error messages break the test even when behavior is
  correct. Prefer the key noun, config key, or value (e.g. `"nonexistent"`,
  `"syncStrategy"`, `"conflicts"`).

### Git operations

- `MergeContinue()` must use `GIT_EDITOR=true git merge --continue` —
  `--no-edit` is invalid with `--continue`. The `--no-edit` flag is silently
  rejected, causing a hard error.

- `RebaseContinue()` also needs `GIT_EDITOR=true` — same reason.

- `StashPop()` matches by stash message prefix (`gx-sync/...`), **not**
  `stash@{0}`. User stash operations shift the stack, so `stash@{0}` pops
  the wrong entry.

- Rebase detection via `git rev-parse --show-current-patch` returns
  false-negatives during interactive rebase pauses. Use `os.Stat` on
  `.git/rebase-merge` / `.git/rebase-apply` instead.

### Architecture

- `cli.Error.Error()` return value is **never consumed** — `main.go`
  reads `Message` directly via `errors.As`. Do not labor over the
  `Error()` method text; it only exists for `error` interface compliance.

- Only `main.go` prints errors — no other package (including workflow)
  should call `fmt.Println` on a `cli.Error`.

- `annotateSubtree` must recurse into leaf commands (e.g. `completion bash`).
  Annotating only the parent cobra command is insufficient — the guard check
  runs per-command.

- `runner.Output()` calls `strings.TrimSpace` on the full captured output, stripping
  leading whitespace from the first line. Parse plumbing output with `strings.Fields`
  instead of fixed-width substring indices.

- `internal/forge/` package (GitHub/GitLab/Bitbucket PR URL detection) is NOT
  listed in the architecture layer table or domain docs. When updating the
  ship command docs, check forge.go + github.go + gitlab.go + bitbucket.go.

- `GetHEADState()` lives in `internal/git/branch.go` (not status.go) despite
  being called by the status workflow. It shares the file with CurrentBranch,
  Checkout, LocalBranchExists, and ShortHeadHash.

### Config

- `.gx/config` is an untracked file. `git add .` commits it to the branch,
  causing it to vanish on checkout. Always `git add <specific_file>` in
  conflict-setup tests.

- `setup_base_repo` always writes `.gx/config`. Delete it with
  `rm -rf "$TEST_DIR/.gx"` for tests that need no-config state.

- Use `gx config <key> <value>` (not `write_config` raw JSON) when testing
  the config→workflow chain — this exercises the full pipeline
  (`config.Set()` → file → `config.Load()` → workflow consumption).

- `gx config list` is a positional subcommand, NOT a `--list` flag.
  There is no --list flag defined anywhere in the codebase.

- Config has five keys not four: remote, defaultBranch, syncStrategy,
  sensitivePatterns, and protectedBranches. The protectedBranches key
  supports Get/Set/CLI --overwrite and is consumed by gx ship.

### Test scripts

- `run_test()` wraps the test function in a **subshell** — `$FAIL`/`$PASS` assertion counters
  do NOT propagate to the parent. Use `$TEST_FAIL`/`$TEST_PASS` (test-level counters set by
  `run_test()`) for exit decisions. `run_test()` detects failure by parsing `✗` from subshell
  output.

- In `run-all.sh`, **capture `PIPESTATUS` immediately** after the pipeline — any intervening
  command resets it. Always: `bash "$suite" 2>&1 | tee "$out"; rc=${PIPESTATUS[0]}` (one line).

- `while read` inside a **pipe** runs in a subshell — variable assignments don't propagate
  to the parent. Use `<<<` (here-string) instead: `while read x; do arr+=("$x"); done <<< "$lines"`.

- `run_gx` provides no stdin. Tests for interactive prompts (bufio.Scanner-based)
  need a separate helper that pipes input, since `os.Stdin` returns EOF immediately.

### File list filtering

- After `--exclude` filtering removes all explicit file args, `hasSpecificFiles` stays
  `true` but the file list is empty — causes `git.Add()` with no args to fail. Reset
  `hasSpecificFiles = false` when `len(files) == 0` after filtering, so the caller
  falls through to the empty-commit guard instead.

### Doc maintenance

- `docs/domains/*.md` routinely fall behind `internal/git/*.go` — new
  functions get added to the code without updating the domain doc. After
  adding or modifying a git function, always cross-reference the matching
  domain doc for missing entries.

- The doc cross-reference pattern: read `cmd/<cmd>.go` for flags/config,
  `internal/workflow/<cmd>.go` for errors/behavior, `internal/git/<file>.go`
  for function signatures, then diff against `docs/commands/<cmd>.md` and
  `docs/domains/<domain>.md`.

- "Currently implemented (code)" goes stale the moment a planned command
  ships. Update that line whenever `resolve`/`start`/`tag`/`undo`/`pr`/
  `clean`/`log`/`stash` moves from planned to implemented.
