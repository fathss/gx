---
name: gx-test-creator
description: >
  Create a self-contained bash test script for a gx CLI command, following the
  project's established testing pattern.
  Trigger when the user says "create a test script for <command>", "make a test for
  <command>", "write tests for <command>", "generate a test script", or
  "add a test suite for <command>". Do NOT use for running tests, debugging code,
  or unrelated shell scripts.
---

# gx-test-creator

Generate a comprehensive, self-contained bash test script for a gx CLI command,
using the shared test library in `tests/lib.sh`.

## Constraints

- **Do NOT modify source code.** This skill creates a test file only.
- **Do NOT run the generated script.** That is the job of gx-test-analyst.
- Write the script to `tests/gx-<command>-test.sh`.
- The script must be **re-runnable** (cleans up and recreates state each run).
- The script must source `tests/lib.sh` for shared helpers.

## Workflow

### 1. Understand the command

Extract the gx command name from the user's request (e.g. `sync`, `save`, `ship`, `status`, `resolve`, `start`, `tag`). If ambiguous, ask.

Then gather context:

- Read `docs/commands/<command>.md` — command specification and user-visible output.
- Read `cmd/<command>.go` — cobra wiring, flags.
- Read `internal/workflow/<command>.go` — full workflow logic, error messages, hints.
- Read `internal/git/` files relevant to the command (check git operations used in workflow).
- Read `docs/domains/*.md` files relevant to the git operations.
- Read `internal/config/config.go` — config struct and defaults.
- Read `internal/cli/error.go` — `cli.Error` type with `Message` and `Hint`.
- Read `gx.md` (the project README) for command descriptions.
- Read `tests/lib.sh` — the shared helper library to use.
- Read `tests/gx-sync-test.sh` — the reference test file to follow.

If any source files don't exist yet (e.g. the command hasn't been implemented), note which parts are missing and generate tests only for what exists.

### 2. Generate the test script

Write a complete bash script at `tests/gx-<command>-test.sh` that sources `lib.sh` and uses its helpers.

#### Directory structure

```
tests/
├── lib.sh                # shared helpers (already exists)
└── gx-<command>-test.sh  # you create this
```

#### Pattern to replicate

| Element | Pattern |
|---------|---------|
| Shebang | `#!/bin/bash` |
| Header | `# gx <command> — comprehensive test suite` |
| Usage comment | `# Usage: bash tests/gx-<command>-test.sh` |
| Re-runnable note | `# Re-runnable: cleans up and recreates everything each run.` |
| Strict mode | `set -e` |
| Source lib | `SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"` + `source "$SCRIPT_DIR/lib.sh"` |
| Build | Call `build_gx` once in the runner block (always rebuilds, ~1s) |
| Temp dirs | Use `setup_tempdir` / `cleanup_tempdir` (uses `mktemp -d`) |
| Git identity | `configure_git` is called inside `setup_base_repo` — no manual action needed |
| Run command | Use `run_gx <subcommand>` — executes in `$TEST_DIR` (override via `GX_RUN_DIR=/path run_gx <cmd>`); captures `$GX_STATUS` and `$GX_OUTPUT` |
| Test functions | `test_NN_description() { ... }` |
| Runner block | `cleanup_tempdir`, `build_gx`, `trap cleanup_tempdir EXIT`, banner, `run_test` each test, `print_test_summary`, `cleanup_tempdir`, `[ "$TEST_FAIL" -eq 0 ]` |

#### Available helpers in lib.sh

**Setup:**

| Helper | Purpose |
|--------|---------|
| `build_gx` | Rebuilds binary from source |
| `setup_tempdir` | Creates `$TEST_ROOT` via `mktemp -d`; sets `$TEST_DIR`, `$ORIGIN_DIR`, `$OTHER_DIR`, `$CONFIG_FILE` |
| `cleanup_tempdir` | Removes temp dir (safe to call multiple times) |
| `configure_git` | Sets `user.name` and `user.email` locally (required in CI) |
| `setup_base_repo` | Creates bare origin + test clone with develop + feature/test branches |
| `write_config <base> <remote> <strategy>` | Writes `.gx/config` with given values |

**Git wrappers** (suppress noisy output; tracing goes here if needed later):

| Wrapper | Expands to | Redirects |
|---------|-----------|-----------|
| `git_checkout <args>` | `git checkout <args>` | stdout+stderr → `/dev/null` |
| `git_commit <args>` | `git commit <args>` | stdout → `/dev/null` |
| `git_push <args>` | `git push <args>` | stdout+stderr → `/dev/null` |
| `git_pull <args>` | `git pull <args>` | stdout+stderr → `/dev/null` |
| `git_fetch <args>` | `git fetch <args>` | stdout+stderr → `/dev/null` |
| `git_head` | `git rev-parse HEAD` | (none — need the hash) |

**Running gx:**

| Helper | Sets |
|--------|------|
| `run_gx <args>` | `$GX_STATUS` (exit code), `$GX_OUTPUT` (captured text); executes in `$TEST_DIR` via subshell |

**Assertions** (all accept a short label as first arg):

| Assertion | What it checks |
|-----------|----------------|
| `assert_exit_code <label> <expected>` | `$GX_STATUS == expected` |
| `assert_output_contains <label> <needle>` | `$GX_OUTPUT` contains needle (grep -F) |
| `assert_output_not_contains <label> <needle>` | `$GX_OUTPUT` does NOT contain needle |
| `assert_branch <label> <expected>` | `git branch --show-current == expected` |
| `assert_clean <label>` | `git status --porcelain --untracked-files=no` is empty |
| `assert_dirty <label>` | working tree has tracked-file modifications |
| `assert_merge_base_equals <label> <ancestor> <descendant>` | ancestor is an ancestor of descendant |
| `assert_head_changed <label> <old_hash>` | HEAD hash changed from old value |
| `assert_no_rebase_in_progress <label>` | no `.git/rebase-merge` or `.git/rebase-apply` directory |
| `assert_rebase_in_progress <label>` | `.git/rebase-merge` or `.git/rebase-apply` exists |
| `assert_git_valid <label>` | `git fsck --no-dangling` passes |
| `run_test <func> "<label>"` | Wraps test in subshell, emits `##TEST_PASS:` / `##TEST_FAIL:` marker |
| `print_test_summary` | Prints test-level pass/fail counts at suite end |

#### Derive test scenarios from the code — not a template

Do **not** use a fixed scenario list. Analyze the command's source files and derive each test scenario from actual code structure:

**Source A: every `cli.Error` return in the workflow**

Each `cli.Error{...}` is an error path the user can hit. Each one becomes a test scenario. For each:

- Extract the exact `Message` and `Hint` strings for output assertions.
- Figure out what git/fs state triggers that return path.
- `setup_base_repo` produces a base repo; each test then customizes it for the scenario.

**Source B: every `if err != nil` block in the workflow**

Each git operation can fail. For each distinct failure, there should be a test that provokes it. Do not test every possible git failure — test those the workflow explicitly handles with distinct `cli.Error` messages.

**Source C: every config-dependent branch**

If the workflow reads `cfg.X` and branches on it, each distinct value is a scenario. Use `write_config` to set non-default values.

**Source D: pre-condition checks**

The first few lines of the workflow are typically guard checks (`IsRepository`, `CurrentBranch`, `IsRebaseInProgress`). Each guard that can return early is a scenario.

**Source E: the happy path**

One test for the basic success flow — no errors, feature branch, default config.

**Putting it together — build the test list by inspecting the workflow source top-to-bottom:**

1. **Guard tests** — one per pre-condition check that can return early.
2. **Happy path test** — the full success flow.
3. **Error path tests** — one per unique `cli.Error` return.
4. **Config variant tests** — one per config-induced branch.

#### Which assertions to use for each scenario

**Prefer state assertions over output assertions.** Git state matters more than wording:

```bash
# Strong — verifies what actually happened
old_head=$(git_head)
run_gx <command>

assert_exit_code "exits 0" 0
assert_branch "still on feature/test" "feature/test"
assert_clean "working tree clean"
assert_merge_base_equals "develop is ancestor" "develop" "feature/test"
assert_head_changed "HEAD changed" "$old_head"
assert_no_rebase_in_progress "no rebase lingering"
assert_git_valid "repository intact"

# Keep a few output assertions for confidence, but don't make every
# test depend on exact wording:
assert_output_contains "shows branch" "Current branch: feature/test"
```

Note: `run_gx` internally executes in `$TEST_DIR`. For tests that need to run outside a repo (guard tests for `IsRepository`), use `GX_RUN_DIR` to override:

```bash
GX_RUN_DIR=/tmp run_gx <command>
```

For error-path tests, assert both:
- Exit code is non-zero
- Output contains the `cli.Error.Message` (and optionally the `Hint`)
- For checkout errors: branch didn't change (`assert_branch`), dirty changes preserved (`assert_dirty`)
- For conflict: rebase actually stopped (`assert_rebase_in_progress`), then `git rebase --abort` to clean up

#### Write style rules

- Source lib.sh, don't duplicate helpers.
- Use `run_gx` instead of direct invocation. `run_gx` executes in `$TEST_DIR` via subshell — no need to `cd` before calling it.
- For tests that must run outside a repo, use `GX_RUN_DIR=/somewhere run_gx <cmd>`. Do not `cd` out of the repo.
- Use git wrappers (`git_checkout`, `git_commit`, etc.) instead of raw `git` commands.
- Assert exit code in every test via `assert_exit_code`.
- Use `setup_tempdir` / `cleanup_tempdir` (never hardcoded `/tmp/` paths).
- Install the trap in the runner block: `trap cleanup_tempdir EXIT` after `build_gx`.
- Call `build_gx` once in the runner block, not inside each test.
- Call `cleanup_tempdir` at the start of each test AND at the end of the runner block.
- Use `run_test` to wrap each test function in the runner block (test-level tracking + markers for run-all.sh).
- Use `print_test_summary` instead of inline "Results:" echo.
- Use `$TEST_FAIL` / `$TEST_PASS` (not `$FAIL` / `$PASS`) for exit decision — assertion counters do not propagate from `run_test` subshells.
- Abort any in-progress rebase at the end of conflict tests (`git rebase --abort`).
- Prefix each test with a banner comment: `# ======================================================================`
- Add a doc comment per test function explaining what it tests.
- Keep assertion labels short and descriptive.

### 3. Verify the file

After writing, check:
- The file exists at `tests/gx-<command>-test.sh`.
- It's at least ~50 lines.
- `lib.sh` is sourced (grep for `source.*lib.sh`).
- `run_gx` is used (grep for `run_gx`).
- `build_gx` is called in the runner block.
- `trap cleanup_tempdir EXIT` is in the runner block (grep for `trap`).
- `assert_exit_code` is used in every test function.
- `run_test` is used for test function calls in the runner block (grep for `run_test`).
- Git wrappers are used in place of raw `git` commands.

### 4. Notify

Return a brief summary:

```
Generated: tests/gx-<command>-test.sh
Tests:     <N> test functions
Coverage:  happy path, guard tests, <error paths>, <config variants>

Run:       bash tests/gx-<command>-test.sh
Analyze:   use the gx-test-analyst skill to run and report results

[Any notes about unimplemented features that were skipped]
```

## Example — dynamic derivation from sync's code

User says: "create a test script for sync commands"

The agent reads `internal/workflow/sync.go` top-to-bottom and derives scenarios from the actual code:

| Code structure | Derived scenario | Key assertions |
|---|---|---|
| `!git.IsRepository(...)` guard | **Test 02** — outside repo (`GX_RUN_DIR=/tmp`) | exit 1, output `"Not inside a Git repository"` |
| `current == ""` guard | **Test 08** — detached HEAD | exit 1, output `"Detached HEAD is not supported"` |
| `git.IsRebaseInProgress(run)` guard | *(merged into confict test)* | — |
| Happy path (feature, rebase) | **Test 01** — success flow | exit 0, branch unchanged, clean, merge-base, head changed, fsck |
| `current == cfg.DefaultBranch` | **Test 03** — on base branch | exit 0, branch=develop, clean, no "Rebasing" |
| `git.Fetch` error → cli.Error | **Test 04** — remote offline | exit 1, output `"Failed to fetch remote"` |
| `git.Checkout` error (dirty) | **Test 07** — dirty working tree | exit 1, branch unchanged, dirty preserved |
| `git.Checkout` error (missing) | **Test 06** — branch missing | exit 1, output `"Failed to checkout"` |
| `git.FastForward` error | **Test 09** — base deleted | exit 1, restored to feature/test |
| `git.Rebase` error → IsRebaseInProgress | **Test 05** — merge conflict | exit 1, rebase in progress, abort |
| `cfg.SyncStrategy == "merge"` | **Test 10** — merge strategy | exit 0, output `"Merging"`, clean |

Each scenario maps to a distinct code path or error return. Tests use `run_gx`, `assert_exit_code`, git state assertions, and a minimal set of output assertions. Git wrappers (`git_checkout`, `git_commit`, etc.) are used instead of raw `git` commands.

## Output template

```
Generated: tests/gx-<command>-test.sh
Tests:     <N> test functions
Coverage:  happy path, guard tests, <error paths>, <config variants>

Run:       bash tests/gx-<command>-test.sh
Analyze:   use the gx-test-analyst skill to run and report results

[Any notes about unimplemented features that were skipped]
```
