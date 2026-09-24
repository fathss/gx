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

## Core principle: the spec is the oracle, the code is not

A test that asserts whatever the implementation currently does can never fail
for the right reason. If the code has a bug, a code-derived test enshrines it.

Therefore:

1. **Expected behavior comes from the spec** (`docs/commands/*.md`, `gx.md`,
   `docs/domains/*.md`) and from git's own semantics.
2. **The implementation is read only for wiring**: exact flag names, config
   keys, how to provoke a state, and error-message keywords.
3. **When the code and the spec disagree, the test asserts the spec.** A test
   that fails on day one is a feature: it is a suspected bug, and it must be
   reported, not silently "fixed" by adjusting the assertion to match the code.
4. **A test must be able to name the bug it would catch.** If it cannot, it is
   a code-restating test and should be deleted or merged.

## Constraints

- **Do NOT modify source code.** This skill creates a test file only.
- **Do NOT run the generated script.** That is the job of gx-test-analyst.
- **Do NOT adjust an assertion to make it match observed code behavior.**
  Assertions come from the contract (Phase 1a), never from reading the code.
- Write the script to `tests/gx-<command>-test.sh`.
- The script must be **re-runnable** (cleans up and recreates state each run).
- The script must source `tests/lib.sh` for shared helpers.

## Workflow

The workflow is ordered on purpose. Do not reorder it. Reading the
implementation before writing the contract anchors you on the code and defeats
the purpose of this skill.

### Phase 0. Identify the command

Extract the gx command name from the user's request (e.g. `sync`, `save`,
`ship`, `status`, `resolve`, `start`, `tag`). If ambiguous, ask.

### Phase 1a. Build the behavioral contract (spec only, NO implementation)

Read ONLY:

- `docs/commands/<command>.md` (command specification and user-visible output)
- `gx.md` (project README, command descriptions)
- `docs/domains/*.md` files relevant to the git operations involved
- `tests/lib.sh` (the helper library)
- `tests/gx-sync-test.sh` (reference for structure and style ONLY, not for
  scenarios)
  **Do NOT open** `cmd/<command>.go`, `internal/workflow/<command>.go`, or
  `internal/git/*.go` yet.

Write the contract as a comment block at the top of the test file:

```
# BEHAVIORAL CONTRACT (derived from docs, before reading implementation)
#
# ID   Class        Precondition            Action     Expected outcome                 Spec ref
# C01  SPEC         feature branch, behind  gx sync    rebased onto develop, exit 0     commands/sync.md#happy-path
# C02  SPEC         outside a repo          gx sync    exit 1, "not a repository"      commands/sync.md#errors
# C03  INVARIANT    any                     any        no commits lost                  (safety)
# C04  UNSPECIFIED  detached HEAD           gx sync    <spec is silent>                 -
```

Rules for the contract:

- Every `SPEC` row MUST cite a doc file and section. No citation, no SPEC row.
- Every "must / should / never / always / only if" in the docs becomes at least
  one contract row.
- If the spec is silent on a behavior, mark the row `UNSPECIFIED`. Do NOT fill
  it in from the code.
- Use git's own semantics as a legitimate oracle where the docs defer to git
  (e.g. after a successful rebase, the base is an ancestor of the branch).

### Phase 1b. Read the implementation (wiring only)

Now read:

- `cmd/<command>.go` (cobra wiring, flags)
- `internal/workflow/<command>.go` (workflow logic)
- `internal/git/` files used by the workflow
- `internal/config/config.go` (config struct and defaults)
- `internal/cli/error.go` (`cli.Error` with `Message` and `Hint`)
  Use these ONLY to learn:

- flag names and config keys,
- how to provoke a given state in a repo,
- error-message **keywords** (see "Assertions on messages" below).
  Do NOT use them to decide what the correct behavior is.

If any source file does not exist yet (the command is not implemented), note
which parts are missing and generate tests only for the behavior that can be
exercised. Contract rows for missing behavior stay in the contract, marked
`# NOT-YET-IMPLEMENTED`, and produce no test function.

### Phase 1c. Reconcile contract against implementation

For each contract row, compare against the implementation and classify:

| Finding                                | Action                                                                                                                                         |
| -------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| Implementation matches the spec        | Normal test.                                                                                                                                   |
| Implementation contradicts the spec    | Write the test to assert the **spec**. Tag it `# EXPECTED-FAIL: code diverges from spec (<how>)`. Report it as a suspected bug in the summary. |
| Spec is silent, code has a behavior    | Either write a `CHARACTERIZATION` test or skip. List it as "undocumented behavior" in the summary. Never present it as verified-correct.       |
| Code has a branch with no contract row | Flag it as "undocumented behavior". Do not silently test it as if it were specified.                                                           |
| Spec requires something the code lacks | Write the test, tag it `# EXPECTED-FAIL: not implemented`.                                                                                     |

### Phase 2. Choose scenarios (priority order)

Cover these in order. Do not start with error-path enumeration.

1. **Spec requirements**: every contract `SPEC` row.
2. **User journeys**: realistic multi-step sequences, not isolated calls.
   Examples: sync, commit, sync again; sync after a teammate pushed to base;
   sync then continue after resolving a conflict.
3. **Boundary and idempotency**: run twice in a row (the second run must be a
   no-op); already up to date; exactly 0, 1, and many commits ahead/behind.
4. **Adversarial states** (what real repos look like): untracked files;
   staged-but-uncommitted changes; stashes; branch names with slashes,
   unicode, or leading dashes; detached HEAD; shallow clone; remote with
   rewritten history; diverged local and remote; empty commit; binary files.
5. **Invariants** (see below): applied to every test.
6. **Error paths**: last. One per distinct user-visible failure the spec
   describes, plus any additional ones found in Phase 1c.
   Sanity check before writing: if more than roughly half of your scenarios are
   in category 6, you are mirroring the code. Go back to 1-4.

### Phase 3. Write the test script

Write a complete bash script at `tests/gx-<command>-test.sh`.

#### Directory structure

```
tests/
├── lib.sh                # shared helpers (already exists; see lib additions)
└── gx-<command>-test.sh  # you create this
```

#### Pattern to replicate

| Element          | Pattern                                                                                                                                                     |
| ---------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Shebang          | `#!/bin/bash`                                                                                                                                               |
| Header           | `# gx <command> — comprehensive test suite`                                                                                                                 |
| Usage comment    | `# Usage: bash tests/gx-<command>-test.sh`                                                                                                                  |
| Re-runnable note | `# Re-runnable: cleans up and recreates everything each run.`                                                                                               |
| Contract block   | The Phase 1a contract table, as a comment block                                                                                                             |
| Strict mode      | `set -e`                                                                                                                                                    |
| Source lib       | `SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"` + `source "$SCRIPT_DIR/lib.sh"`                                                                               |
| Build            | Call `build_gx` once in the runner block (always rebuilds, ~1s)                                                                                             |
| Temp dirs        | Use `setup_tempdir` / `cleanup_tempdir` (uses `mktemp -d`)                                                                                                  |
| Git identity     | `configure_git` is called inside `setup_base_repo`                                                                                                          |
| Run command      | `run_gx <subcommand>` runs in `$TEST_DIR` (override with `GX_RUN_DIR=/path run_gx <cmd>`); sets `$GX_STATUS` and `$GX_OUTPUT`                               |
| Test functions   | `test_NN_description() { ... }`                                                                                                                             |
| Runner block     | `cleanup_tempdir`, `build_gx`, `trap cleanup_tempdir EXIT`, banner, `run_test` each test, `print_test_summary`, `cleanup_tempdir`, `[ "$TEST_FAIL" -eq 0 ]` |

#### Available helpers in lib.sh

**Setup:**

| Helper                                    | Purpose                                                                                             |
| ----------------------------------------- | --------------------------------------------------------------------------------------------------- |
| `build_gx`                                | Rebuilds binary from source                                                                         |
| `setup_tempdir`                           | Creates `$TEST_ROOT` via `mktemp -d`; sets `$TEST_DIR`, `$ORIGIN_DIR`, `$OTHER_DIR`, `$CONFIG_FILE` |
| `cleanup_tempdir`                         | Removes temp dir (safe to call multiple times)                                                      |
| `configure_git`                           | Sets `user.name` and `user.email` locally (required in CI)                                          |
| `setup_base_repo`                         | Creates bare origin + test clone with develop + feature/test branches                               |
| `write_config <base> <remote> <strategy>` | Writes `.gx/config` with given values                                                               |

**Git wrappers** (suppress noisy output):

| Wrapper               | Expands to            | Redirects                    |
| --------------------- | --------------------- | ---------------------------- |
| `git_checkout <args>` | `git checkout <args>` | stdout+stderr to `/dev/null` |
| `git_commit <args>`   | `git commit <args>`   | stdout to `/dev/null`        |
| `git_push <args>`     | `git push <args>`     | stdout+stderr to `/dev/null` |
| `git_pull <args>`     | `git pull <args>`     | stdout+stderr to `/dev/null` |
| `git_fetch <args>`    | `git fetch <args>`    | stdout+stderr to `/dev/null` |
| `git_head`            | `git rev-parse HEAD`  | (none, need the hash)        |

**Running gx:**

| Helper          | Sets                                                                                         |
| --------------- | -------------------------------------------------------------------------------------------- |
| `run_gx <args>` | `$GX_STATUS` (exit code), `$GX_OUTPUT` (captured text); executes in `$TEST_DIR` via subshell |

**Assertions** (all accept a short label as first arg):

| Assertion                                                  | What it checks                                                       |
| ---------------------------------------------------------- | -------------------------------------------------------------------- |
| `assert_exit_code <label> <expected>`                      | `$GX_STATUS == expected`                                             |
| `assert_exit_nonzero <label>`                              | `$GX_STATUS != 0`                                                    |
| `assert_output_contains <label> <needle>`                  | `$GX_OUTPUT` contains needle (grep -F)                               |
| `assert_output_not_contains <label> <needle>`              | `$GX_OUTPUT` does NOT contain needle                                 |
| `assert_branch <label> <expected>`                         | `git branch --show-current == expected`                              |
| `assert_clean <label>`                                     | no tracked-file modifications                                        |
| `assert_dirty <label>`                                     | working tree has tracked-file modifications                          |
| `assert_merge_base_equals <label> <ancestor> <descendant>` | ancestor is an ancestor of descendant                                |
| `assert_head_changed <label> <old_hash>`                   | HEAD hash changed from old value                                     |
| `assert_head_unchanged <label> <old_hash>`                 | HEAD hash equals old value                                           |
| `assert_no_rebase_in_progress <label>`                     | no `.git/rebase-merge` or `.git/rebase-apply`                        |
| `assert_rebase_in_progress <label>`                        | `.git/rebase-merge` or `.git/rebase-apply` exists                    |
| `assert_git_valid <label>`                                 | `git fsck --no-dangling` passes                                      |
| `run_test <func> "<label>"`                                | Wraps test in subshell, emits `##TEST_PASS:` / `##TEST_FAIL:` marker |
| `print_test_summary`                                       | Prints test-level pass/fail counts                                   |

**Invariant helpers** (see `lib-additions.sh`; add to `tests/lib.sh` if missing):

| Helper                                   | Purpose                                                                                                     |
| ---------------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| `snapshot_state`                         | Records HEAD, current branch, all refs, all reachable commits, and tracked working-tree diff into `$SNAP_*` |
| `assert_no_commits_lost <label>`         | Every commit reachable at snapshot time is still reachable now (via refs or reflog)                         |
| `assert_worktree_preserved <label>`      | Tracked uncommitted changes present at snapshot are still present                                           |
| `assert_state_unchanged <label>`         | HEAD, branch, and working tree identical to snapshot                                                        |
| `assert_recoverable <label>`             | Either state is unchanged, or an operation is in progress and `--abort` restores the snapshot               |
| `assert_idempotent <label> <subcommand>` | Re-running the subcommand exits 0 and leaves HEAD unchanged                                                 |
| `assert_exit_output_consistent <label>`  | exit 0 has no error markers; non-zero has non-empty output                                                  |

#### Mandatory invariants

Every test function MUST call `snapshot_state` immediately before `run_gx` and
finish with the applicable invariants:

**On every path (success or failure):**

- `assert_git_valid`
- `assert_no_commits_lost`
- `assert_exit_output_consistent`
  **On every failure path (non-zero exit):**

- `assert_recoverable` (repo untouched, OR an operation is in progress that
  `--abort` cleanly reverses)
- `assert_worktree_preserved`
  **On every success path:**

- `assert_idempotent` where the command is naturally repeatable (a second run
  is a no-op)
- `assert_no_rebase_in_progress` unless the spec says the command may stop
  mid-operation
  A test without a `snapshot_state` before `run_gx` is incomplete.

#### Assertions on messages

Exact-string matching against the implementation's own strings is the most
tautological assertion available: the code says X, the test checks for X.

- Assert messages by **intent**: match one short, stable keyword
  (`"conflict"`, `"remote"`, `"repository"`), not the full sentence.
- Prefer keywords the **spec/docs** use over strings copied from the code.
- **Every error test must also assert a state outcome** (branch unchanged,
  HEAD unchanged, changes preserved, rebase state). A test whose only
  assertions are exit code plus message text is incomplete.
- Keep a few output assertions for user-facing confidence, but never make a
  test depend solely on wording.

#### Prefer state assertions over output assertions

```bash
# Strong: verifies what actually happened
snapshot_state
old_head=$(git_head)
run_gx <command>

assert_exit_code "exits 0" 0
assert_branch "still on feature/test" "feature/test"
assert_clean "working tree clean"
assert_merge_base_equals "develop is ancestor" "develop" "feature/test"
assert_head_changed "HEAD changed" "$old_head"
assert_no_rebase_in_progress "no rebase lingering"
assert_git_valid "repository intact"
assert_no_commits_lost "no commits lost"
assert_idempotent "second run is a no-op" <command>
```

For tests that must run outside a repo, use `GX_RUN_DIR`:

```bash
GX_RUN_DIR=/tmp run_gx <command>
```

Never `cd` out of the repo.

#### Write style rules

- Source lib.sh, do not duplicate helpers.
- Use `run_gx`, not direct invocation.
- Use git wrappers (`git_checkout`, `git_commit`, etc.), not raw `git`.
- Assert exit code in every test via `assert_exit_code` (or
  `assert_exit_nonzero` when the spec only requires "fails").
- Use `setup_tempdir` / `cleanup_tempdir` (never hardcoded `/tmp/` paths).
- Install `trap cleanup_tempdir EXIT` in the runner block after `build_gx`.
- Call `build_gx` once in the runner block, not inside each test.
- Call `cleanup_tempdir` at the start of each test AND at the end of the runner
  block.
- Wrap each test with `run_test` in the runner block.
- Use `print_test_summary`; decide the exit code from `$TEST_FAIL` / `$TEST_PASS`
  (assertion counters do not propagate out of `run_test` subshells).
- Abort any in-progress rebase at the end of conflict tests
  (`git rebase --abort`).
- Prefix each test with a banner comment:
  `# ======================================================================`
- Every test function has a doc comment with three lines:
  `# Contract: <ID>`, `# Class: SPEC | INVARIANT | CHARACTERIZATION | EXPECTED-FAIL`,
  `# Catches: <the plausible bug this test would detect>`.
- Keep assertion labels short and descriptive.

#### Test classes

| Class              | Meaning                                                                | If it fails                                           |
| ------------------ | ---------------------------------------------------------------------- | ----------------------------------------------------- |
| `SPEC`             | Derived from documented behavior. Authoritative.                       | Code is presumed wrong.                               |
| `INVARIANT`        | Cross-cutting safety property (no data loss, recoverable, idempotent). | Code is wrong. Serious.                               |
| `EXPECTED-FAIL`    | Spec and code diverge; test asserts the spec.                          | Confirms a suspected bug.                             |
| `CHARACTERIZATION` | Spec is silent; pins current behavior. Lowest confidence.              | Behavior changed; a human decides whether it matters. |

A suite that is mostly `CHARACTERIZATION` is a warning sign. Report the mix.

### Phase 4. Adversarial self-review (mandatory)

Before finalizing, attack your own tests.

**Per-test check.** Every test's `# Catches:` line must name a concrete bug. If
you cannot write one, delete the test or merge it into another.

**Mutation pass.** Mentally mutate the workflow and confirm at least one test
would fail for each mutation:

- The fetch step is removed.
- The rebase and merge branches are swapped.
- The dirty-working-tree guard is deleted.
- The command does nothing and returns 0.
- The command returns non-zero but leaves the repo half-modified.
- The command force-updates or deletes a branch it should not touch.
- The base branch is checked out but the original branch is not restored.
- An error is swallowed and the exit code is 0.
- A conflict leaves a rebase in progress with no hint shown.
  List every mutation that no test would catch. Add a test for each, or record it
  as an accepted gap in the summary.

### Phase 5. Verify the file

After writing, check:

- The file exists at `tests/gx-<command>-test.sh`.
- It is at least ~50 lines.
- The contract comment block is present (grep for `BEHAVIORAL CONTRACT`).
- `lib.sh` is sourced (grep for `source.*lib.sh`).
- `run_gx` is used (grep for `run_gx`).
- `build_gx` is called in the runner block.
- `trap cleanup_tempdir EXIT` is in the runner block (grep for `trap`).
- `assert_exit_code` or `assert_exit_nonzero` is used in every test function.
- `snapshot_state` appears in every test function that calls `run_gx`.
- `run_test` is used for test function calls in the runner block.
- Every test function has `# Contract:`, `# Class:`, and `# Catches:` lines.
- Git wrappers are used in place of raw `git` commands.
- No assertion depends solely on a full message sentence copied from the code.

### Phase 6. Notify

Return this summary:

```
Generated: tests/gx-<command>-test.sh
Tests:     <N> total  (SPEC: a | INVARIANT: b | EXPECTED-FAIL: c | CHARACTERIZATION: d)

Suspected spec/code divergences (EXPECTED-FAIL):
  - <test> : spec says <X>, code does <Y>
  (or "none found")

Undocumented behavior in code (no spec coverage):
  - <description>
  (or "none")

Spec gaps (behavior unspecified, no test written):
  - <description>
  (or "none")

Mutations no test would catch:
  - <description>
  (or "none")

Not yet implemented (contract rows with no test):
  - <description>
  (or "none")

Run:       bash tests/gx-<command>-test.sh
Analyze:   use the gx-test-analyst skill to run and report results
```

A high `CHARACTERIZATION` count, an empty divergence list on a complex command,
or a long "spec gaps" list is a signal the docs need work, not that the code is
perfect.

## Example: deriving sync tests from the spec

User says: "create a test script for sync commands"

**Phase 1a** (docs only). Contract rows come from the documentation, for example:

| ID  | Class       | Precondition               | Expected (from docs)                                            |
| --- | ----------- | -------------------------- | --------------------------------------------------------------- |
| C01 | SPEC        | feature branch behind base | branch rebased onto latest base, exit 0                         |
| C02 | SPEC        | outside a repository       | exit non-zero, explains it is not a repo                        |
| C03 | SPEC        | on the base branch         | fast-forwards base, does not rebase it onto itself              |
| C04 | SPEC        | remote unreachable         | exit non-zero, nothing local changed                            |
| C05 | SPEC        | dirty working tree         | does not lose uncommitted changes                               |
| C06 | SPEC        | rebase conflict            | stops with rebase in progress, tells user how to continue/abort |
| C07 | SPEC        | `sync_strategy = merge`    | merges instead of rebasing                                      |
| C08 | INVARIANT   | any                        | no commits lost, repo valid                                     |
| C09 | SPEC        | already up to date         | no-op, exit 0                                                   |
| C10 | UNSPECIFIED | detached HEAD              | spec silent; decide later                                       |

**Phase 1b/1c** (read code, reconcile). Suppose the docs say a dirty working
tree must be preserved, but `workflow/sync.go` runs `git checkout` without
checking for local changes and can fail mid-way, leaving the user on the wrong
branch. The test for C05 asserts the **spec** (branch restored, changes
preserved), is tagged `EXPECTED-FAIL`, and is reported as a suspected bug.
It is not softened to match the code.

**Phase 2** (scenarios). Beyond the rows above, add journeys and adversarial
states the code never mentions: sync twice (idempotency), sync after the
remote's base was force-pushed, sync with staged-but-uncommitted changes, a
branch named `feature/ünï-cödé`, a shallow clone.

**Phase 4** (mutation). Deleting the fetch step is caught by C01 (base
never advances). Making the command a silent no-op is caught by C01 and C06.
A checkout that never restores the original branch is caught by C04's
`assert_branch` plus `assert_state_unchanged`.

The result is a suite where a bug in the implementation produces a red test,
instead of a suite that mirrors the implementation and is always green.
