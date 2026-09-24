# gx — AGENTS.md

Compact, high-signal instructions for agents working on this repo.

## Project

Opinionated Git CLI for combining common multi-step Git workflows into single, safe commands. Go module: `github.com/fathss/gx`.

## Design boundary

`gx` is not a general-purpose Git wrapper. Its job is to combine multiple Git operations into safe, opinionated workflows such as `gx sync`, `gx save`, and `gx ship`.

Use `git` directly for operations that gx does not own.

Do not add:

- catch-all/pass-through commands such as `gx pull`
- a `gx git` subcommand
- thin wrappers for `rebase`, `merge`, `stash`, etc.
- gx commands whose only purpose is exposing a single raw Git command

When gx cannot handle an operation, user-facing hints should direct the user to `git <cmd>`.

Dependencies: `spf13/cobra` (CLI).

Tests: Bash scripts under `tests/`.

---

## Architecture

The code follows a strict layered architecture:

```
cmd/ → internal/workflow/ → internal/git/ → internal/runner/
```

Layer violations are bugs.

| Layer | Must | Must NOT |
|---|---|---|
| `cmd/` | cobra definitions, flags, config loading, invoke workflows | execute Git directly, contain workflow logic |
| `internal/workflow/` | orchestrate Git functions, convert errors to `cli.Error` | call `exec.Command`, depend on cobra internals |
| `internal/git/` | one function per Git operation, thin wrappers over runner | print messages, know workflows, create `cli.Error` |
| `internal/runner/` | execute Git, logging, stdio plumbing | understand Git semantics or workflow logic |
| `internal/config/` | load `.gx/config`, provide defaults | — |
| `internal/cli/` | define `Error{Message, Hint}` | print errors |

## Error handling

Only `main.go` prints errors. No other package should print a `cli.Error` or user-facing error.

Workflow errors should use:

```go
if err := git.SomeOp(...); err != nil {
    return &cli.Error{
        Message: "user-facing message",
        Hint:    "actionable hint",
    }
}
```

`cli.Error` has `Message` and `Hint`.

Only `Message` and `Hint` are shown to users. `Hint` may contain `\n`; `main.go` renders each hint line as a separate lowercase `hint:` line.

`cli.Error.Error()` only exists to satisfy Go's `error` interface. `main.go` extracts the concrete error with `errors.As` and reads `Message` directly.

---

## Safety contract

Every gx command that mutates the repository follows:

**check → warn → act → recover**

1. **check** — validate preconditions such as clean state, branch, config, or semver format.
2. **warn** — surface unusual or potentially destructive conditions before acting.
3. **act** — perform the Git operations.
4. **recover** — undo partial state after failure and report what the user should do next.

A command must not silently leave the repository in a broken or ambiguous state.

---

## Pre-flight guard

`cmd/root.go` uses `PersistentPreRunE` to validate repository and config state before commands execute.

Commands normally require both:

- a Git repository
- `.gx/config`

Annotations can bypass either check:

| Annotation | Effect |
|---|---|
| `no_repo` | skip repository check |
| `no_config` | skip config check |
| none | require both |

New commands should declare annotations in their cobra command when necessary:

```go
Annotations: map[string]string{"no_config": ""},
```

Built-in `help` and `completion` commands are annotated dynamically in `PersistentPreRunE`. Annotation must recurse into leaf commands such as `completion bash`; annotating only the parent command is insufficient.

---

## Adding a command

A normal new command requires exactly:

```
cmd/foo.go
internal/workflow/foo.go
docs/commands/foo.md
```

The command layer should define cobra behavior and invoke the workflow. Workflow code owns orchestration and user-facing errors.

Only add `internal/git/*.go` and `docs/domains/*.md` when the command introduces a genuinely new Git capability.

### Exception: `gx config`

`gx config` intentionally bypasses the normal workflow/Git layers. It reads and writes `.gx/config` directly through the config package.

This is the only intentional architecture exception.

---

## Config

`.gx/config` is the source of truth for gx configuration.

Default configuration is provided by the config package, but behavior that depends on persisted configuration must not silently introduce a second set of hardcoded defaults.

In particular:

- `gx init` seeds `sensitivePatterns` from `config.DefaultSensitivePatterns`.
- `MatchSensitivePatterns` must use the configured patterns and must not concatenate hidden hardcoded defaults.
- Missing `.gx/config` causes the normal pre-flight guard to require `gx init`.
- `gx init` and `gx config` bypass the config requirement via `no_config`.
- `config.Load()` can fall back to defaults when the file is absent; this path is intentionally used by `gx init` and `gx config`.
- Empty config fields fall back to defaults.

Current keys:

- `remote`
- `defaultBranch`
- `syncStrategy`
- `sensitivePatterns`
- `protectedBranches`

Example:

```json
{
  "remote": "origin",
  "defaultBranch": "develop",
  "syncStrategy": "rebase",
  "protectedBranches": ["main", "master", "develop"]
}
```

`sensitivePatterns` and `protectedBranches` accept multiple positional values through `gx config`. Other keys accept at most one value.

`gx config list` is a positional subcommand, not a `--list` flag.

---

## Git invariants

These are cross-cutting rules that are easy to violate and expensive to debug.

### gx-owned stashes

gx-owned sync stashes are identified by their message prefix:

```
gx-sync/
```

Never assume `stash@{0}` is the gx stash. User stash operations can reorder the stash stack.

### Rebase/merge detection

Detect repository rebase/merge state from Git's state files, such as:

```
.git/rebase-merge
.git/rebase-apply
.git/MERGE_HEAD
```

Do not use `git rev-parse --show-current-patch` as the sole indication that a rebase is in progress; it can produce false negatives during paused rebases.

### Manual vs gx-orchestrated operations

A manually started rebase or merge must not be mistaken for one initiated by gx.

The presence of a gx-owned stash is the signal used by sync to distinguish gx-orchestrated state from manual Git state.

See `docs/domains/sync.md` for the complete sync state machine.

---

## Output philosophy

Git should explain Git operations.

`run.Run()` always prints a:

```
▸ git <args>
```

header before streaming Git's output.

gx should only print information that Git does not already communicate, such as:

- gx stash context
- continuation signals
- conflict summaries
- recovery instructions

Do not add redundant messages such as:

```
Done.
Fetching origin...
```

when Git already provides equivalent output.

Detailed runner behavior is documented with the runner implementation.

---

## Testing

Tests are Bash-based and share helpers through:

```
tests/lib.sh
```

There is normally one suite per command:

```
tests/gx-sync-test.sh
tests/gx-save-test.sh
tests/gx-ship-test.sh
...
```

Run one suite:

```bash
bash tests/gx-COMMAND-test.sh
```

Run everything:

```bash
bash tests/run-all.sh
```

`run-all.sh` discovers test suites using the:

```
tests/*-test.sh
```

glob, so new test files do not need to be registered manually.

### Test assertions

Tests should assert both:

1. process outcome with `assert_exit_code`
2. relevant output/state with an assertion such as `assert_output_contains`

Output assertions should use the minimum unique substring that proves the behavior. Do not assert complete sentences or full error messages.

**Good:**

- `"conflicts"`
- `"Git repository"`
- `"syncStrategy"`
- `"Merge stopped"`

**Bad:**

- `"Merge stopped because conflicts were detected in the repository."`

This keeps tests resilient to wording improvements.

### Test implementation details

Test setup, `run_test()` behavior, stdin handling, `PIPESTATUS`, test counters, and other Bash-specific conventions live in:

```
tests/AGENTS.md
```

Read that file when modifying or adding tests.

---

## Documentation maintenance

When changing a command, cross-check the relevant layers:

```
cmd/<cmd>.go
internal/workflow/<cmd>.go
internal/git/*.go
docs/commands/<cmd>.md
docs/domains/*.md
```

The implementation is the source of truth for behavior.

When adding or modifying a Git function, check the corresponding domain documentation for missing or stale entries.

When changing command behavior, update the command documentation.

Do not allow documentation to describe planned behavior as implemented or implemented behavior as planned.

---

## Current implementation status

**Currently implemented:**

- `gx sync`
- `gx save`
- `gx ship`
- `gx status`
- `gx init`
- `gx config`
- `gx clean`

**Planned but not yet implemented:**

- `resolve`
- `start`
- `tag`
- `undo`
- `pr`
- `log`
- `stash`

Update this section whenever a planned command becomes implemented.

---

## Agent workflow

### Issue tracker

Issues are local Markdown files under:

```
.scratch/<feature>/
```

See:

```
.agents/docs/issue-tracker.md
```

### Triage labels

The canonical labels are:

- `needs-triage`
- `needs-info`
- `ready-for-agent`
- `ready-for-human`
- `wontfix`

See:

```
.agents/docs/triage-labels.md
```

---

## Before finishing a change

For a code change:

- preserve the layer boundaries
- keep Git operations in `internal/git`
- keep orchestration in `internal/workflow`
- keep cobra concerns in `cmd`
- return `cli.Error` from workflows for user-facing failures
- ensure only `main.go` prints errors
- preserve the check → warn → act → recover safety model for mutations
- update affected command/domain documentation
- add or update tests for changed behavior

For a new command:

```
cmd/<cmd>.go
internal/workflow/<cmd>.go
docs/commands/<cmd>.md
```

and add Git/domain files only when introducing new Git capabilities.

For test changes, read:

```
tests/AGENTS.md
```

before changing shared test infrastructure.

---

## High-value traps

Keep this list short. If a trap is specific to a subsystem, document it in that subsystem instead of adding it here.

- `.gx/config` is untracked. Never use broad `git add .` in tests where the config file must remain outside the commit; stage specific files instead.
- Use `gx config` when testing config-to-workflow behavior so the test exercises the real `Set()` → file → `Load()` → workflow path.
- `cli.Error.Error()` text is not a user-facing output contract; `main.go` reads `Message` directly.
- gx-owned stashes must be located by their `gx-sync/` prefix, never by stash index.
- Manual Git operations must not be confused with gx-orchestrated operations.

Subsystem-specific traps belong in:

```
tests/AGENTS.md
docs/domains/*.md
```
