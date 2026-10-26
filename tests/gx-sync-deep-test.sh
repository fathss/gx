#!/bin/bash
# gx sync — deep correctness/reliability suite (mutation + recovery paths)
# Usage: bash tests/gx-sync-deep-test.sh
# Re-runnable: cleans up and recreates everything each run.
#
# Focus: mutations the local repository, so every test asserts either exact
# state preservation (failure paths), exact completion (success paths), or
# stash integrity. Complements tests/gx-sync-test.sh.
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib.sh"
source "$SCRIPT_DIR/lib-invariants.sh"

# ======================================================================
# BEHAVIORAL CONTRACT (derived from docs, before reading implementation)
#
# ID   Class             Precondition                         Action               Expected outcome                                              Spec ref
# S01  SPEC              feature behind base, 2 commits       gx sync              all local commits kept, base ancestor, exit 0                commands/sync.md#normal-flow
# S02  SPEC              feature already based on base        gx sync              no-op, HEAD unchanged, exit 0                               commands/sync.md#normal-flow
# S03  SPEC              dirty tracked change                 gx sync              dirty change preserved exactly, exit 0                      commands/sync.md#normal-success-with-stash
# S04  INVARIANT         staged uncommitted change            gx sync              staged content preserved, no data loss                      (safety)
# S05  CHARACTERIZATION  untracked file                       gx sync              untracked file preserved                                    (spec silent)
# S06  SPEC              untracked file collides with base    gx sync              exit non-zero, original branch/HEAD/worktree intact         commands/sync.md#edge-cases
# S07  SPEC              dirty tree + fetch failure           gx sync              exit non-zero, dirty change restored, no stash left         commands/sync.md#edge-cases
# S08  SPEC              on base, dirty, remote advanced      gx sync              base fast-forwarded, dirty change restored, exit 0          commands/sync.md#normal-flow
# S09  SPEC              local base diverged from remote      gx sync              exit non-zero, local base untouched, dirty restored         commands/sync.md#edge-cases
# S10  SPEC              remote base deleted                  gx sync              exit non-zero, original branch restored, dirty restored     commands/sync.md#edge-cases
# S11  SPEC              conflict (dirty), resolved           gx sync --continue   rebase completes, stash restored/dropped, exit 0            commands/sync.md#on-re-run-after-resolving-conflicts
# S12  SPEC              conflict (dirty)                     gx sync --abort      original HEAD/branch/worktree restored, stash dropped       commands/sync.md#aborting-with---abort
# S13  SPEC              conflict (dirty)                     gx sync --skip       conflicting commit dropped, dirty restored, exit 0          commands/sync.md#edge-cases
# S14  EXPECTED-FAIL     clean tree, rebase conflict          gx sync --continue   must continue gx's own rebase (hint says so)                commands/sync.md#on-conflict-first-run
# S15  EXPECTED-FAIL     clean tree, merge conflict           gx sync --continue   must continue gx's own merge (hint says so)                 commands/sync.md#on-conflict-first-run
# S16  EXPECTED-FAIL     clean tree, rebase conflict          gx sync --abort      must abort gx's own rebase                                  commands/sync.md#aborting-with---abort
# S17  EXPECTED-FAIL     clean tree, merge conflict           gx sync --abort      must abort gx's own merge                                   commands/sync.md#aborting-with---abort
# S18  INVARIANT         user stash exists                    gx sync              user stash untouched, gx stash cleaned up                   (safety)
# S19  SPEC              orphaned same-branch gx stash        gx sync              stash restored, sync proceeds, no gx stash left             commands/sync.md#orphaned-gx-stash-q2
# S20  EXPECTED-FAIL     orphaned gx stash from branch A      gx sync on B         A's changes must not be applied to B; stash preserved       (data integrity)
# S21  SPEC              rebase succeeds, stash pop conflicts gx sync --continue   exit non-zero, stash kept, conflicts shown                  commands/sync.md#edge-cases
# S22  SPEC              merge conflict, gx stash             gx sync --skip       rejected, merge stays paused, stash kept                    commands/sync.md#edge-cases
# S23  SPEC              idle                                 conflicting flags    exit non-zero, no mutation                                  commands/sync.md#flags
# S24  INVARIANT         dirty tree                           gx sync twice        second run is a no-op, dirty change preserved               (safety)
# S25  SPEC              another live sync holds the lock     gx sync              exit non-zero, clear lock message, no mutation               (concurrency safety)
# S26  SPEC              stale marker+stash from branch A      gx sync on B         A's stash must stay on A; B unchanged                        (data integrity)
# ======================================================================

# ======================================================================
# Setup helpers
# ======================================================================

# Bare origin + clone with develop initialized; OTHER_DIR is a collaborator
# clone used to advance develop.
setup_sync_repo() {
    cleanup_tempdir
    setup_tempdir
    mkdir -p "$ORIGIN_DIR"
    cd "$ORIGIN_DIR"
    git init --bare 2>/dev/null
    git symbolic-ref HEAD refs/heads/develop 2>/dev/null || true
    # Allow tests to simulate a remote whose default branch was deleted.
    git config receive.denyDeleteCurrent ignore

    mkdir -p "$TEST_DIR"
    git clone "$ORIGIN_DIR" "$TEST_DIR" 2>/dev/null
    cd "$TEST_DIR"
    configure_git
    git checkout -b develop 2>/dev/null || git_checkout develop
    echo "base" > base.txt
    git add base.txt
    git_commit -m "init"
    git_push -u origin develop
    git --git-dir="$ORIGIN_DIR" symbolic-ref HEAD refs/heads/develop 2>/dev/null || true

    write_config "develop" "origin" "rebase"
    echo ".gx/" >> "$TEST_DIR/.git/info/exclude"

    git clone "$ORIGIN_DIR" "$OTHER_DIR" 2>/dev/null
    cd "$OTHER_DIR"
    configure_git
    git checkout -q -B develop origin/develop
    cd "$TEST_DIR"
}

# create_feature <branch> [extra commit message...]
# Creates the branch from develop, commits one file, pushes it.
create_feature() {
    local branch="$1"
    shift
    local fname="${branch//\//_}"
    cd "$TEST_DIR"
    git checkout -b "$branch" develop 2>/dev/null || git_checkout "$branch"
    echo "$branch work" > "${fname}.txt"
    git add "${fname}.txt"
    git_commit -m "first commit on $branch"
    local msg
    for msg in "$@"; do
        echo "$msg" > "${fname}_$(echo "$msg" | tr ' ' '_').txt"
        git add .
        git_commit -m "$msg"
    done
    git_push -u origin "$branch"
}

# advance_base <name> [file] [content] — collaborator commits on develop.
advance_base() {
    local name="$1"
    local file="${2:-upstream.txt}"
    local content="${3:-upstream}"
    cd "$OTHER_DIR"
    git checkout -q develop
    git pull -q --ff-only origin develop >/dev/null 2>&1 || true
    printf '%s\n' "$content" > "$file"
    git add "$file"
    git_commit -m "upstream: $name"
    git_push origin develop
    cd "$TEST_DIR"
}

# setup_conflict <rebase|merge> <none|tracked|samefile>
# Creates feature/x changing f.txt and an upstream commit changing f.txt.
# Leaves the test clone on feature/x with the requested dirty state.
setup_conflict() {
    local strategy="$1"
    local dirty="${2:-none}"
    setup_sync_repo
    cd "$TEST_DIR"
    git checkout -b feature/x develop 2>/dev/null || git_checkout feature/x
    echo "feature" > f.txt
    git add f.txt
    git_commit -m "feature change"
    git_push -u origin feature/x

    write_config "develop" "origin" "$strategy"

    cd "$OTHER_DIR"
    git pull -q --ff-only origin develop >/dev/null 2>&1 || true
    echo "upstream" > f.txt
    git add f.txt
    git_commit -m "upstream change"
    git_push origin develop

    cd "$TEST_DIR"
    git_checkout feature/x
    case "$dirty" in
        tracked)  echo "dirty" >> base.txt ;;
        samefile) echo "dirty" > f.txt ;;
    esac
}

assert_branch_equals_remote() {
    local label="$1" branch="$2" remote_ref="$3"
    local a b
    a=$(cd "$TEST_DIR" && git rev-parse "$branch" 2>/dev/null || echo "")
    b=$(cd "$TEST_DIR" && git rev-parse "$remote_ref" 2>/dev/null || echo "")
    if [ -n "$a" ] && [ "$a" = "$b" ]; then
        pass "$label"
    else
        fail "$label — $branch ($a) != $remote_ref ($b)"
    fi
}

# ======================================================================
# S01 — multi-commit rebase keeps every local commit
# ======================================================================
# Contract: S01
# Class: SPEC
# Catches: sync dropping or reordering local commits while rebasing
test_s01_multi_commit() {
    echo ""
    echo "=== S01: multi-commit rebase keeps every commit ==="
    setup_sync_repo
    cd "$TEST_DIR"
    create_feature "feature/multi" "local one" "local two"
    advance_base "base moved"
    git_checkout feature/multi

    snapshot_state
    snapshot_origin_refs
    run_gx sync

    assert_exit_code "exits 0" 0
    assert_branch "still on feature/multi" "feature/multi"
    assert_clean "working tree clean"
    assert_merge_base_equals "develop is ancestor" "develop" "feature/multi"
    assert_file_equals "first local file intact" "feature_multi.txt" "feature/multi work"
    assert_file_equals "second local commit applied" \
        "feature_multi_local_one.txt" "local one"
    assert_file_equals "third local commit applied" \
        "feature_multi_local_two.txt" "local two"
    assert_no_gx_stash "no gx stash left"
    assert_stash_count "no stash entries" 0
    assert_origin_refs_unchanged "sync does not push"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
    assert_idempotent "second sync is a no-op" sync
}

# ======================================================================
# S02 — already up to date is a no-op
# ======================================================================
# Contract: S02
# Class: SPEC
# Catches: sync rewriting HEAD or stashing when there is nothing to do
test_s02_up_to_date() {
    echo ""
    echo "=== S02: already up to date is a no-op ==="
    setup_sync_repo
    cd "$TEST_DIR"
    git checkout -b feature/uptodate develop
    git_push -u origin feature/uptodate
    old_head=$(git_head)

    snapshot_state
    run_gx sync

    assert_exit_code "exits 0" 0
    assert_branch "still on feature/uptodate" "feature/uptodate"
    assert_head_unchanged "HEAD unchanged" "$old_head"
    assert_clean "working tree clean"
    assert_stash_count "no stash entries" 0
    assert_no_rebase_in_progress "no rebase lingering"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
    assert_idempotent "second sync is a no-op" sync
}

# ======================================================================
# S03 — dirty tracked change is preserved exactly
# ======================================================================
# Contract: S03
# Class: SPEC
# Catches: stash push/pop losing or altering uncommitted content
test_s03_dirty_preserved() {
    echo ""
    echo "=== S03: dirty tracked change is preserved exactly ==="
    setup_sync_repo
    cd "$TEST_DIR"
    create_feature "feature/dirty" "local one"
    advance_base "base moved"
    git_checkout feature/dirty
    echo "dirty content" >> base.txt

    snapshot_state
    run_gx sync

    assert_exit_code "exits 0" 0
    assert_branch "still on feature/dirty" "feature/dirty"
    assert_worktree_preserved "dirty change preserved exactly"
    assert_merge_base_equals "develop is ancestor" "develop" "feature/dirty"
    assert_stash_count "gx stash cleaned up" 0
    assert_no_gx_stash "no gx stash left"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# S04 — staged content survives sync
# ======================================================================
# Contract: S04
# Class: INVARIANT
# Catches: sync discarding staged-but-uncommitted content during stash/restore
test_s04_staged_preserved() {
    echo ""
    echo "=== S04: staged content survives sync ==="
    setup_sync_repo
    cd "$TEST_DIR"
    create_feature "feature/staged" "local one"
    advance_base "base moved"
    git_checkout feature/staged
    echo "staged content" >> base.txt
    git add base.txt

    snapshot_state
    run_gx sync

    assert_exit_code "exits 0" 0
    assert_branch "still on feature/staged" "feature/staged"
    assert_worktree_preserved "staged content preserved"
    assert_file_contains "content still present" "base.txt" "staged content"
    assert_stash_count "gx stash cleaned up" 0
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# S05 — untracked files survive sync
# ======================================================================
# Contract: S05
# Class: CHARACTERIZATION
# Catches: sync removing or overwriting untracked files
test_s05_untracked_preserved() {
    echo ""
    echo "=== S05: untracked files survive sync ==="
    setup_sync_repo
    cd "$TEST_DIR"
    create_feature "feature/untracked" "local one"
    advance_base "base moved"
    git_checkout feature/untracked
    echo "scratch" > scratch.txt

    snapshot_state
    run_gx sync

    assert_exit_code "exits 0" 0
    assert_branch "still on feature/untracked" "feature/untracked"
    assert_file_equals "untracked file preserved" "scratch.txt" "scratch"
    assert_stash_count "untracked files are not stashed" 0
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# S06 — untracked collision aborts without mutating
# ======================================================================
# Contract: S06
# Class: SPEC
# Catches: sync overwriting an untracked file or leaving the wrong branch
test_s06_untracked_collision() {
    echo ""
    echo "=== S06: untracked collision aborts without mutating ==="
    setup_sync_repo
    cd "$TEST_DIR"
    create_feature "feature/collide" "local one"
    advance_base "collide" "collide.txt" "upstream version"
    git_checkout feature/collide
    echo "my local file" > collide.txt

    snapshot_state
    run_gx sync

    assert_exit_nonzero "exits non-zero"
    assert_branch "original branch restored" "feature/collide"
    assert_head_unchanged "HEAD unchanged" "$SNAP_HEAD"
    assert_worktree_preserved "untracked file untouched"
    assert_stash_count "no stash left" 0
    assert_recoverable "repository recoverable"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# S07 — fetch failure restores a dirty tree
# ======================================================================
# Contract: S07
# Class: SPEC
# Catches: sync losing the stashed change when fetch fails
test_s07_fetch_failure_restores() {
    echo ""
    echo "=== S07: fetch failure restores a dirty tree ==="
    setup_sync_repo
    cd "$TEST_DIR"
    create_feature "feature/fetchfail" "local one"
    git_checkout feature/fetchfail
    echo "dirty content" >> base.txt
    git remote set-url origin "/nonexistent/path/to/origin"

    snapshot_state
    run_gx sync

    assert_exit_nonzero "exits non-zero"
    assert_output_contains "fetch failure surfaced" "fetch"
    assert_branch "still on feature/fetchfail" "feature/fetchfail"
    assert_head_unchanged "HEAD unchanged" "$SNAP_HEAD"
    assert_worktree_preserved "dirty change restored"
    assert_stash_count "no stash left" 0
    assert_no_gx_stash "no gx stash left"
    assert_recoverable "repository recoverable"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# S08 — on base branch: fast-forward with dirty tree
# ======================================================================
# Contract: S08
# Class: SPEC
# Catches: sync cutting a rebase on the base branch instead of fast-forwarding
test_s08_base_dirty() {
    echo ""
    echo "=== S08: base branch fast-forwards with dirty tree ==="
    setup_sync_repo
    cd "$TEST_DIR"
    advance_base "base moved"
    git_checkout develop
    echo "dirty on base" >> base.txt

    snapshot_state
    run_gx sync

    assert_exit_code "exits 0" 0
    assert_branch "still on develop" "develop"
    assert_branch_equals_remote "develop == origin/develop" "develop" "origin/develop"
    assert_worktree_preserved "dirty change preserved"
    assert_stash_count "gx stash cleaned up" 0
    assert_no_rebase_in_progress "no rebase on base branch"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# S09 — diverged local base is never overwritten
# ======================================================================
# Contract: S09
# Class: SPEC
# Catches: sync force-moving a base branch that has local commits
test_s09_base_diverged() {
    echo ""
    echo "=== S09: diverged local base is never overwritten ==="
    setup_sync_repo
    cd "$TEST_DIR"
    advance_base "base moved"
    git_checkout develop
    echo "local-only" > local_only.txt
    git add local_only.txt
    git_commit -m "local base commit"
    echo "dirty on base" >> base.txt

    snapshot_state
    run_gx sync

    assert_exit_nonzero "exits non-zero"
    assert_output_contains "divergence surfaced" "diverg"
    assert_branch "still on develop" "develop"
    assert_head_unchanged "local base commit kept" "$SNAP_HEAD"
    assert_worktree_preserved "dirty change restored"
    assert_stash_count "no stash left" 0
    assert_recoverable "repository recoverable"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# S10 — remote base deleted: restore and report
# ======================================================================
# Contract: S10
# Class: SPEC
# Catches: sync leaving the user on the base branch or losing the stash
test_s10_remote_base_deleted() {
    echo ""
    echo "=== S10: remote base deleted restores original state ==="
    setup_sync_repo
    cd "$TEST_DIR"
    create_feature "feature/deleted" "local one"
    git_push origin --delete develop
    git_checkout feature/deleted
    echo "dirty content" >> base.txt

    snapshot_state
    run_gx sync

    assert_exit_nonzero "exits non-zero"
    assert_output_contains "deleted base surfaced" "deleted"
    assert_branch "original branch restored" "feature/deleted"
    assert_head_unchanged "HEAD unchanged" "$SNAP_HEAD"
    assert_worktree_preserved "dirty change restored"
    assert_stash_count "no stash left" 0
    assert_recoverable "repository recoverable"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# S11 — conflict, resolve, --continue completes and restores stash
# ======================================================================
# Contract: S11
# Class: SPEC
# Catches: continue not finishing the rebase or dropping the stashed change
test_s11_conflict_continue() {
    echo ""
    echo "=== S11: conflict -> resolve -> --continue ==="
    setup_conflict rebase tracked
    cd "$TEST_DIR"

    snapshot_state
    run_gx sync

    assert_exit_nonzero "first run stops on conflict"
    assert_rebase_in_progress "rebase left in progress"
    assert_stash_count "gx stash preserved" 1

    echo "resolved" > f.txt
    git add f.txt

    run_gx sync --continue

    assert_exit_code "continue exits 0" 0
    assert_no_rebase_in_progress "rebase completed"
    assert_branch "still on feature/x" "feature/x"
    assert_merge_base_equals "develop is ancestor" "develop" "feature/x"
    assert_worktree_preserved "dirty change restored"
    assert_stash_count "gx stash cleaned up" 0
    assert_no_gx_stash "no gx stash left"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# S12 — conflict + --abort restores the exact pre-sync state
# ======================================================================
# Contract: S12
# Class: SPEC
# Catches: abort leaving HEAD, branch, or the dirty tree altered
test_s12_conflict_abort() {
    echo ""
    echo "=== S12: conflict -> --abort restores pre-sync state ==="
    setup_conflict rebase tracked
    cd "$TEST_DIR"

    snapshot_state
    run_gx sync

    assert_exit_nonzero "first run stops on conflict"
    assert_rebase_in_progress "rebase left in progress"
    assert_stash_count "gx stash preserved" 1

    run_gx sync --abort

    assert_exit_code "abort exits 0" 0
    assert_no_rebase_in_progress "rebase aborted"
    assert_branch "original branch restored" "feature/x"
    assert_head_unchanged "original HEAD restored" "$SNAP_HEAD"
    assert_worktree_preserved "dirty change restored"
    assert_stash_count "gx stash cleaned up" 0
    assert_no_gx_stash "no gx stash left"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# S13 — conflict + --skip drops the commit and finishes
# ======================================================================
# Contract: S13
# Class: SPEC
# Catches: skip leaving the rebase paused or losing the stashed change
test_s13_conflict_skip() {
    echo ""
    echo "=== S13: conflict -> --skip finishes ==="
    setup_conflict rebase tracked
    cd "$TEST_DIR"

    snapshot_state
    run_gx sync

    assert_exit_nonzero "first run stops on conflict"
    assert_rebase_in_progress "rebase left in progress"

    run_gx sync --skip

    assert_exit_code "skip exits 0" 0
    assert_no_rebase_in_progress "rebase completed"
    assert_branch "still on feature/x" "feature/x"
    assert_merge_base_equals "develop is ancestor" "develop" "feature/x"
    assert_file_equals "conflicting commit skipped" "f.txt" "upstream"
    assert_worktree_preserved "dirty change restored"
    assert_stash_count "gx stash cleaned up" 0
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# S14 — clean-tree rebase conflict: gx must continue its own rebase
# ======================================================================
# Contract: S14
# Class: EXPECTED-FAIL
# Catches: Q-table classifies a gx rebase as manual when no stash was created,
#          so --continue refuses the operation gx itself started.
#          Spec: first-run conflict hint says "run gx sync --continue".
test_s14_clean_rebase_continue() {
    echo ""
    echo "=== S14: clean-tree rebase conflict -> --continue (EXPECTED-FAIL) ==="
    setup_conflict rebase none
    cd "$TEST_DIR"

    snapshot_state
    run_gx sync

    assert_exit_nonzero "first run stops on conflict"
    assert_rebase_in_progress "rebase left in progress"
    assert_stash_count "no stash was needed" 0
    assert_output_contains "hint promises --continue" "--continue"

    echo "resolved" > f.txt
    git add f.txt

    run_gx sync --continue

    assert_exit_code "continue exits 0 (spec)" 0
    assert_no_rebase_in_progress "rebase completed (spec)"
    assert_branch "still on feature/x (spec)" "feature/x"
    assert_merge_base_equals "develop is ancestor (spec)" "develop" "feature/x"
    assert_file_equals "resolved content kept (spec)" "f.txt" "resolved"
    assert_stash_count "no stash left (spec)" 0
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"

    cd "$TEST_DIR" && git rebase --abort >/dev/null 2>&1 || true
}

# ======================================================================
# S15 — clean-tree merge conflict: gx must continue its own merge
# ======================================================================
# Contract: S15
# Class: EXPECTED-FAIL
# Catches: same Q-table gap for merge: no stash means Q5 (manual) instead of Q6
test_s15_clean_merge_continue() {
    echo ""
    echo "=== S15: clean-tree merge conflict -> --continue (EXPECTED-FAIL) ==="
    setup_conflict merge none
    cd "$TEST_DIR"

    snapshot_state
    run_gx sync

    assert_exit_nonzero "first run stops on conflict"
    assert_merge_in_progress "merge left in progress"
    assert_stash_count "no stash was needed" 0
    assert_output_contains "hint promises --continue" "--continue"

    echo "resolved" > f.txt
    git add f.txt

    run_gx sync --continue

    assert_exit_code "continue exits 0 (spec)" 0
    assert_no_merge_in_progress "merge completed (spec)"
    assert_branch "still on feature/x (spec)" "feature/x"
    assert_merge_base_equals "develop is ancestor (spec)" "develop" "feature/x"
    assert_file_equals "resolved content kept (spec)" "f.txt" "resolved"
    assert_stash_count "no stash left (spec)" 0
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"

    cd "$TEST_DIR" && git merge --abort >/dev/null 2>&1 || true
}

# ======================================================================
# S16 — clean-tree rebase conflict: gx must abort its own rebase
# ======================================================================
# Contract: S16
# Class: EXPECTED-FAIL
# Catches: --abort refuses a gx-started rebase when no stash exists
test_s16_clean_rebase_abort() {
    echo ""
    echo "=== S16: clean-tree rebase conflict -> --abort (EXPECTED-FAIL) ==="
    setup_conflict rebase none
    cd "$TEST_DIR"

    snapshot_state
    run_gx sync

    assert_exit_nonzero "first run stops on conflict"
    assert_rebase_in_progress "rebase left in progress"

    run_gx sync --abort

    assert_exit_code "abort exits 0 (spec)" 0
    assert_no_rebase_in_progress "rebase aborted (spec)"
    assert_branch "original branch restored (spec)" "feature/x"
    assert_head_unchanged "original HEAD restored (spec)" "$SNAP_HEAD"
    assert_worktree_preserved "worktree restored (spec)"
    assert_stash_count "no stash left (spec)" 0
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"

    cd "$TEST_DIR" && git rebase --abort >/dev/null 2>&1 || true
}

# ======================================================================
# S17 — clean-tree merge conflict: gx must abort its own merge
# ======================================================================
# Contract: S17
# Class: EXPECTED-FAIL
# Catches: --abort refuses a gx-started merge when no stash exists
test_s17_clean_merge_abort() {
    echo ""
    echo "=== S17: clean-tree merge conflict -> --abort (EXPECTED-FAIL) ==="
    setup_conflict merge none
    cd "$TEST_DIR"

    snapshot_state
    run_gx sync

    assert_exit_nonzero "first run stops on conflict"
    assert_merge_in_progress "merge left in progress"

    run_gx sync --abort

    assert_exit_code "abort exits 0 (spec)" 0
    assert_no_merge_in_progress "merge aborted (spec)"
    assert_branch "original branch restored (spec)" "feature/x"
    assert_head_unchanged "original HEAD restored (spec)" "$SNAP_HEAD"
    assert_worktree_preserved "worktree restored (spec)"
    assert_stash_count "no stash left (spec)" 0
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"

    cd "$TEST_DIR" && git merge --abort >/dev/null 2>&1 || true
}

# ======================================================================
# S18 — the user's own stash is never touched
# ======================================================================
# Contract: S18
# Class: INVARIANT
# Catches: popping/dropping a user stash instead of the gx-owned stash
test_s18_user_stash_survives() {
    echo ""
    echo "=== S18: user stash survives sync ==="
    setup_sync_repo
    cd "$TEST_DIR"
    create_feature "feature/userstash" "local one"
    advance_base "base moved"
    git_checkout feature/userstash

    echo "user work" >> base.txt
    git stash push -m "user-wip" >/dev/null 2>&1
    echo "gx dirty" >> feature_userstash.txt

    snapshot_state
    run_gx sync

    assert_exit_code "exits 0" 0
    assert_stash_count "user stash still present" 1
    if (cd "$TEST_DIR" && git stash list | grep -Fq "user-wip"); then
        pass "user stash label intact"
    else
        fail "user stash label intact — user-wip missing"
    fi
    if (cd "$TEST_DIR" && git stash show -p "stash@{0}" 2>/dev/null | grep -Fq "user work"); then
        pass "user stash content intact"
    else
        fail "user stash content intact — 'user work' missing"
    fi
    assert_file_contains "gx dirty change restored" "feature_userstash.txt" "gx dirty"
    assert_no_gx_stash "no gx stash left"
    assert_merge_base_equals "develop is ancestor" "develop" "feature/userstash"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# S19 — orphaned same-branch gx stash is restored, then cleaned up
# ======================================================================
# Contract: S19
# Class: SPEC
# Catches: Q2 not restoring the orphan, or leaving it behind after success
test_s19_orphaned_same_branch() {
    echo ""
    echo "=== S19: orphaned same-branch gx stash ==="
    setup_sync_repo
    cd "$TEST_DIR"
    create_feature "feature/orphan" "local one"
    advance_base "base moved"
    git_checkout feature/orphan
    echo "orphan work" >> feature_orphan.txt
    git stash push -m "gx-sync/feature/orphan/424242" >/dev/null 2>&1
    assert_stash_count "orphan stash created" 1

    snapshot_state
    run_gx sync

    assert_exit_code "exits 0" 0
    assert_output_contains "orphan restored" "Restoring"
    assert_file_contains "orphan content restored" "feature_orphan.txt" "orphan work"
    assert_stash_count "orphan stash cleaned up" 0
    assert_no_gx_stash "no gx stash left"
    assert_merge_base_equals "develop is ancestor" "develop" "feature/orphan"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# S20 — another branch's gx stash must not leak into this branch
# ======================================================================
# Contract: S20
# Class: EXPECTED-FAIL
# Catches: gx identifies stashes by prefix only (not branch), so feature/a's
#          stashed work is applied and dropped while syncing feature/b.
test_s20_cross_branch_stash() {
    echo ""
    echo "=== S20: cross-branch gx stash must not leak (EXPECTED-FAIL) ==="
    setup_sync_repo
    cd "$TEST_DIR"
    create_feature "feature/a" "a work"
    create_feature "feature/b" "b work"

    git_checkout feature/a
    echo "secret-a" > base.txt
    git stash push -m "gx-sync/feature/a/424242" >/dev/null 2>&1
    git_checkout feature/b

    assert_stash_count "feature/a stash present" 1

    snapshot_state
    run_gx sync

    assert_exit_code "exits 0" 0
    assert_branch "still on feature/b" "feature/b"
    assert_file_equals "feature/a change not applied (spec)" "base.txt" "base"
    assert_stash_count "feature/a stash preserved (spec)" 1
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
}

# ======================================================================
# S21 — stash pop conflict after a successful rebase keeps the stash
# ======================================================================
# Contract: S21
# Class: SPEC
# Catches: a conflict while restoring the stash being treated as success or
#          dropping the stash, which would lose the user's uncommitted work.
test_s21_stash_pop_conflict() {
    echo ""
    echo "=== S21: stash pop conflict after rebase keeps stash ==="
    setup_conflict rebase samefile
    cd "$TEST_DIR"

    snapshot_state
    run_gx sync

    assert_exit_nonzero "first run stops on rebase conflict"
    assert_rebase_in_progress "rebase left in progress"
    assert_stash_count "gx stash preserved" 1

    echo "resolved" > f.txt
    git add f.txt

    run_gx sync --continue

    assert_exit_nonzero "pop conflict exits non-zero"
    assert_output_contains "pop conflict surfaced" "Stash pop"
    assert_no_rebase_in_progress "rebase itself completed"
    assert_stash_count "stash kept on stack" 1
    assert_file_contains "conflict markers visible" "f.txt" "<<<<<<<"
    assert_branch "still on feature/x" "feature/x"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"

    cd "$TEST_DIR" && git checkout -- . >/dev/null 2>&1 || true
}

# ======================================================================
# S22 — --skip is rejected during a gx merge and keeps it paused
# ======================================================================
# Contract: S22
# Class: SPEC
# Catches: --skip aborting or completing a merge (it is rebase-only)
test_s22_skip_during_merge() {
    echo ""
    echo "=== S22: --skip rejected during merge ==="
    setup_conflict merge tracked
    cd "$TEST_DIR"

    snapshot_state
    run_gx sync

    assert_exit_nonzero "first run stops on merge conflict"
    assert_merge_in_progress "merge left in progress"
    assert_stash_count "gx stash preserved" 1

    run_gx sync --skip

    assert_exit_nonzero "skip rejected"
    assert_output_contains "skip/merge reason shown" "merge"
    assert_merge_in_progress "merge still in progress"
    assert_stash_count "gx stash still preserved" 1
    assert_head_unchanged "HEAD unchanged" "$SNAP_HEAD"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"

    run_gx sync --abort
    assert_exit_code "abort after rejection exits 0" 0
    assert_no_merge_in_progress "merge aborted"
    assert_stash_count "stash restored/dropped" 0
}

# ======================================================================
# S23 — conflicting flag combinations never mutate the repository
# ======================================================================
# Contract: S23
# Class: SPEC
# Catches: a flag-conflict check running after the repository was mutated
test_s23_flag_matrix() {
    echo ""
    echo "=== S23: conflicting flag combinations ==="
    setup_sync_repo
    cd "$TEST_DIR"
    create_feature "feature/flags" "local one"
    git_checkout feature/flags

    snapshot_state
    local combo
    for combo in \
        "--abort --continue" \
        "--skip --abort" \
        "--skip --continue" \
        "--abort --rebase" \
        "--abort --merge" \
        "--continue --rebase" \
        "--continue --merge" \
        "--continue" \
        "--skip"; do
        run_gx sync $combo
        assert_exit_nonzero "rejects: $combo"
        assert_state_unchanged "no mutation: $combo"
    done
    assert_no_rebase_in_progress "no rebase started"
    assert_no_merge_in_progress "no merge started"
    assert_stash_count "no stash created" 0
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# S24 — sync is idempotent while the tree stays dirty
# ======================================================================
# Contract: S24
# Class: INVARIANT
# Catches: a second sync rewriting HEAD or losing the dirty change
test_s24_dirty_idempotent() {
    echo ""
    echo "=== S24: sync is idempotent with a dirty tree ==="
    setup_sync_repo
    cd "$TEST_DIR"
    create_feature "feature/idem" "local one"
    advance_base "base moved"
    git_checkout feature/idem
    echo "dirty content" >> base.txt

    snapshot_state
    run_gx sync

    assert_exit_code "first sync exits 0" 0
    assert_worktree_preserved "dirty change preserved after first sync"
    local head_after
    head_after=$(git_head)

    run_gx sync

    assert_exit_code "second sync exits 0" 0
    assert_head_unchanged "HEAD unchanged on second sync" "$head_after"
    assert_worktree_preserved "dirty change preserved after second sync"
    assert_stash_count "no stash left" 0
    assert_no_gx_stash "no gx stash left"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# S25 — a second sync is blocked by the repository lock
# ======================================================================
# Contract: S25
# Class: SPEC
# Catches: concurrent syncs racing on index.lock and reporting a misleading
#          "commit or discard your changes" hint instead of a lock message
test_s25_sync_lock() {
    echo ""
    echo "=== S25: repository lock blocks a concurrent sync ==="
    setup_sync_repo
    cd "$TEST_DIR"
    create_feature "feature/locked" "local one"
    advance_base "base moved"
    git_checkout feature/locked

    # Simulate a live gx sync: the test shell's PID is guaranteed alive.
    printf '%s\n' "$$" > "$TEST_DIR/.git/gx-sync.lock"

    snapshot_state
    run_gx sync

    assert_exit_nonzero "second sync exits non-zero"
    assert_output_contains "lock contention explained" "Another gx sync"
    assert_state_unchanged "repository untouched while locked"
    assert_no_rebase_in_progress "no rebase started"
    assert_stash_count "no stash created" 0

    rm -f "$TEST_DIR/.git/gx-sync.lock"
    run_gx sync

    assert_exit_code "sync works after lock released" 0
    assert_branch "still on feature/locked" "feature/locked"
    assert_merge_base_equals "develop is ancestor" "develop" "feature/locked"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# S26 — a stale marker from another branch never restores its stash here
# ======================================================================
# Contract: S26
# Class: SPEC
# Catches: Q2 using a stale marker's branch to pop branch A's stash while the
#          user is on branch B (the path left by `git rebase --abort` + switch)
test_s26_foreign_marker() {
    echo ""
    echo "=== S26: stale foreign marker does not leak stash ==="
    setup_sync_repo
    cd "$TEST_DIR"
    create_feature "feature/a" "a work"
    create_feature "feature/b" "b work"

    git_checkout feature/a
    echo "secret-a" > base.txt
    git stash push -m "gx-sync/feature/a/424242" >/dev/null 2>&1
    printf 'feature/a\n' > "$TEST_DIR/.git/gx-sync-state"
    git_checkout feature/b

    assert_stash_count "feature/a stash present" 1

    snapshot_state
    run_gx sync

    assert_exit_code "exits 0" 0
    assert_branch "still on feature/b" "feature/b"
    assert_file_equals "feature/a change not applied" "base.txt" "base"
    assert_stash_count "feature/a stash preserved" 1
    if (cd "$TEST_DIR" && git stash list | grep -Fq "gx-sync/feature/a/"); then
        pass "feature/a stash label intact"
    else
        fail "feature/a stash label intact — label missing"
    fi
    if [ ! -f "$TEST_DIR/.git/gx-sync-state" ]; then
        pass "stale marker cleared"
    else
        fail "stale marker cleared — marker still present"
    fi
    assert_merge_base_equals "develop is ancestor of b" "develop" "feature/b"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# RUN ALL
# ======================================================================
cleanup_tempdir
build_gx
trap cleanup_tempdir EXIT

echo "============================================"
echo "  gx sync — deep reliability suite"
echo "============================================"
echo ""

# `|| true` keeps `set -e` from aborting the suite at the first failure, so
# every suspected divergence is reported in one run.
run_test test_s01_multi_commit "S01: multi-commit rebase keeps every commit" || true
run_test test_s02_up_to_date "S02: already up to date is a no-op" || true
run_test test_s03_dirty_preserved "S03: dirty tracked change preserved" || true
run_test test_s04_staged_preserved "S04: staged content survives sync" || true
run_test test_s05_untracked_preserved "S05: untracked files survive sync" || true
run_test test_s06_untracked_collision "S06: untracked collision aborts safely" || true
run_test test_s07_fetch_failure_restores "S07: fetch failure restores dirty tree" || true
run_test test_s08_base_dirty "S08: base branch fast-forwards with dirty tree" || true
run_test test_s09_base_diverged "S09: diverged local base never overwritten" || true
run_test test_s10_remote_base_deleted "S10: remote base deleted restores state" || true
run_test test_s11_conflict_continue "S11: conflict -> resolve -> --continue" || true
run_test test_s12_conflict_abort "S12: conflict -> --abort restores state" || true
run_test test_s13_conflict_skip "S13: conflict -> --skip finishes" || true
run_test test_s14_clean_rebase_continue "S14: clean-tree rebase --continue (EXPECTED-FAIL)" || true
run_test test_s15_clean_merge_continue "S15: clean-tree merge --continue (EXPECTED-FAIL)" || true
run_test test_s16_clean_rebase_abort "S16: clean-tree rebase --abort (EXPECTED-FAIL)" || true
run_test test_s17_clean_merge_abort "S17: clean-tree merge --abort (EXPECTED-FAIL)" || true
run_test test_s18_user_stash_survives "S18: user stash survives sync" || true
run_test test_s19_orphaned_same_branch "S19: orphaned same-branch gx stash" || true
run_test test_s20_cross_branch_stash "S20: cross-branch gx stash must not leak (EXPECTED-FAIL)" || true
run_test test_s21_stash_pop_conflict "S21: stash pop conflict keeps stash" || true
run_test test_s22_skip_during_merge "S22: --skip rejected during merge" || true
run_test test_s23_flag_matrix "S23: conflicting flags never mutate" || true
run_test test_s24_dirty_idempotent "S24: sync idempotent with dirty tree" || true
run_test test_s25_sync_lock "S25: repository lock blocks concurrent sync" || true
run_test test_s26_foreign_marker "S26: stale foreign marker does not leak stash" || true

echo ""
print_test_summary

cleanup_tempdir

[ "$TEST_FAIL" -eq 0 ]
