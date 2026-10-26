#!/bin/bash
# gx clean — deep correctness/reliability suite
# Usage: bash tests/gx-clean-deep-test.sh
# Re-runnable: cleans up and recreates everything each run.
#
# Focus: the mutation boundary — exactly which refs may be deleted, against
# which base, with which answers — plus safety when deletion is impossible.
# Complements tests/gx-clean-test.sh.
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib.sh"
source "$SCRIPT_DIR/lib-invariants.sh"

# ======================================================================
# BEHAVIORAL CONTRACT (derived from docs, before reading implementation)
#
# ID   Class             Precondition                           Action                    Expected outcome                                             Spec ref
# C01  SPEC              custom protectedBranches config        gx clean --yes            protected branch kept, other merged pruned                  commands/clean.md#candidate-selection
# C02  SPEC              current branch merged, tracking exists gx clean --yes            local current kept, origin/current tracking pruned          commands/clean.md#candidate-selection
# C03  SPEC              local branch ahead of merged tracking  gx clean --yes            local (unmerged) kept, merged tracking pruned               commands/clean.md#candidate-selection
# C04  SPEC              merged local branch                    gx clean -y               prunes without prompting                                    commands/clean.md#flags
# C05  SPEC              merged local branch                    answer "YES"              case-insensitive yes prunes                                 commands/clean.md#candidate-selection
# C06  SPEC              merged branch, --remote, one answer    gx clean --remote         EOT at second prompt declines remote, exit 0                commands/clean.md#candidate-selection
# C07  INVARIANT         remote advanced since last fetch       gx clean --yes --remote   fetch first, discover+prune new candidates                  commands/clean.md#usage
# C08  INVARIANT         merged branch                          gx clean --yes            remote refs untouched without --remote                      commands/clean.md#what-it-deletes
# C09  SPEC              local defaultBranch missing            gx clean --yes            exit non-zero, nothing deleted                              commands/clean.md#candidate-selection
# C10  SPEC              remote base missing                    gx clean --yes            exit non-zero, nothing deleted                              commands/clean.md#candidate-selection
# C11  SPEC              dirty working tree                     gx clean --yes            prunes, dirty content preserved exactly                     commands/clean.md#pre-flight
# C12  CHARACTERIZATION  branch checked out in a worktree       gx clean --yes            branch kept + warning, tracking pruned, exit 0             commands/clean.md#edge-cases
# C13  SPEC              remote branch already deleted remotely  gx clean --yes --remote   prune stale tracking ref via fetch, exit 0, no failure   commands/clean.md#edge-cases
# C14  SPEC              unmerged remote branch                 gx clean --yes --remote   remote branch kept, exit 0                                  commands/clean.md#candidate-selection
# C15  INVARIANT         repeat after a successful clean        gx clean --yes twice      second run stays safe; may re-prune re-created tracking ref (safety)
# C16  SPEC              nothing merged                         gx clean --yes            "Pruned 0", no prompt, exit 0                               commands/clean.md#edge-cases
# ======================================================================

# ======================================================================
# Setup and assertions
# ======================================================================

setup_clean_repo() {
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
    echo "initial" > readme.md
    git add readme.md
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

write_config_protected() {
    local protected_json="$1"
    printf '{"remote":"origin","defaultBranch":"develop","syncStrategy":"rebase","protectedBranches":%s}\n' \
        "$protected_json" > "$CONFIG_FILE"
}

# create_merged_branch <branch> [workdir]
# Creates, pushes, merges into develop (--no-ff), and pushes develop.
create_merged_branch() {
    local branch="$1"
    local wdir="${2:-$TEST_DIR}"
    local fname="${branch//\//_}"
    cd "$wdir"
    git checkout -b "$branch" develop 2>/dev/null || git_checkout "$branch"
    echo "$branch work" > "${fname}.txt"
    git add "${fname}.txt"
    git_commit -m "$branch work"
    git_push -u origin "$branch" 2>/dev/null || git_push origin "$branch"
    git_checkout develop
    git merge --no-ff "$branch" -m "merge $branch" >/dev/null 2>&1
    git_push origin develop
    cd "$TEST_DIR"
}

create_unmerged_branch() {
    local branch="$1"
    local fname="${branch//\//_}"
    cd "$TEST_DIR"
    git checkout -b "$branch" develop 2>/dev/null || git_checkout "$branch"
    echo "$branch unmerged" > "${fname}_unmerged.txt"
    git add "${fname}_unmerged.txt"
    git_commit -m "$branch unmerged work"
    git_push -u origin "$branch"
    git_checkout develop
}

assert_local_branch_exists() {
    local label="$1" branch="$2"
    if (cd "$TEST_DIR" && git show-ref --verify --quiet "refs/heads/$branch" 2>/dev/null); then
        pass "$label"
    else
        fail "$label — expected local branch '$branch' to exist"
    fi
}

assert_local_branch_not_exists() {
    local label="$1" branch="$2"
    if (cd "$TEST_DIR" && git show-ref --verify --quiet "refs/heads/$branch" 2>/dev/null); then
        fail "$label — expected local branch '$branch' NOT to exist"
    else
        pass "$label"
    fi
}

assert_remote_tracking_exists() {
    local label="$1" branch="$2"
    if (cd "$TEST_DIR" && git rev-parse --verify --quiet "refs/remotes/origin/$branch" >/dev/null 2>&1); then
        pass "$label"
    else
        fail "$label — expected origin/$branch tracking ref to exist"
    fi
}

assert_remote_tracking_not_exists() {
    local label="$1" branch="$2"
    if (cd "$TEST_DIR" && git rev-parse --verify --quiet "refs/remotes/origin/$branch" >/dev/null 2>&1); then
        fail "$label — expected origin/$branch tracking ref NOT to exist"
    else
        pass "$label"
    fi
}

assert_remote_branch_exists() {
    local label="$1" branch="$2"
    if git --git-dir="$ORIGIN_DIR" show-ref --verify --quiet "refs/heads/$branch" 2>/dev/null; then
        pass "$label"
    else
        fail "$label — expected remote branch '$branch' to exist"
    fi
}

assert_remote_branch_not_exists() {
    local label="$1" branch="$2"
    if git --git-dir="$ORIGIN_DIR" show-ref --verify --quiet "refs/heads/$branch" 2>/dev/null; then
        fail "$label — expected remote branch '$branch' NOT to exist"
    else
        pass "$label"
    fi
}

# ======================================================================
# C01 — custom protectedBranches are honored
# ======================================================================
# Contract: C01
# Class: SPEC
# Catches: clean ignoring the configured protectedBranches list
test_c01_custom_protected() {
    echo ""
    echo "=== C01: custom protectedBranches honored ==="
    setup_clean_repo
    create_merged_branch "keepme"
    create_merged_branch "goaway"
    write_config_protected '["keepme"]'
    git_checkout develop

    snapshot_state
    run_gx clean --yes

    assert_exit_code "exits 0" 0
    assert_output_contains "pruned summary" "Pruned"
    assert_local_branch_exists "protected keepme kept" "keepme"
    assert_local_branch_not_exists "unprotected goaway pruned" "goaway"
    assert_remote_tracking_not_exists "goaway tracking pruned" "goaway"
    assert_remote_tracking_exists "keepme tracking kept" "keepme"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# C02 — current branch kept, its tracking ref still prunable
# ======================================================================
# Contract: C02
# Class: SPEC
# Catches: clean deleting the checked-out branch, or refusing to prune its
#          remote-tracking ref (docs explicitly allow origin/<current>)
test_c02_current_tracking() {
    echo ""
    echo "=== C02: current local kept, origin/current tracking pruned ==="
    setup_clean_repo
    create_merged_branch "feature/current"
    git_checkout feature/current

    snapshot_origin_refs
    run_gx clean --yes

    assert_exit_code "exits 0" 0
    assert_branch "still on current branch" "feature/current"
    assert_local_branch_exists "current local branch kept" "feature/current"
    assert_remote_tracking_not_exists "origin/current tracking pruned" "feature/current"
    assert_remote_branch_exists "remote branch kept without --remote" "feature/current"
    assert_origin_refs_unchanged "remote refs untouched"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# C03 — local branch with unmerged commits is never deleted
# ======================================================================
# Contract: C03
# Class: SPEC
# Catches: using the remote base to delete a local branch that still has
#          unpushed commits not merged anywhere
test_c03_local_ahead() {
    echo ""
    echo "=== C03: local ahead of merged tracking is kept ==="
    setup_clean_repo

    git checkout -b feature/ahead develop
    echo "pushed commit" > pushed.txt
    git add pushed.txt
    git_commit -m "pushed commit"
    git_push -u origin feature/ahead

    echo "unpushed commit" > unpushed.txt
    git add unpushed.txt
    git_commit -m "unpushed commit"

    git_checkout develop
    git_fetch origin
    git merge --no-ff origin/feature/ahead -m "merge pushed tip" >/dev/null 2>&1
    git_push origin develop

    run_gx clean --yes

    assert_exit_code "exits 0" 0
    assert_local_branch_exists "local unmerged work kept" "feature/ahead"
    assert_remote_tracking_not_exists "merged tracking pruned" "feature/ahead"
    assert_remote_branch_exists "remote branch kept without --remote" "feature/ahead"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# C04 — -y short flag prunes without prompting
# ======================================================================
# Contract: C04
# Class: SPEC
# Catches: the short form being ignored (prompting or not pruning)
test_c04_short_yes() {
    echo ""
    echo "=== C04: -y short flag ==="
    setup_clean_repo
    create_merged_branch "feature/short-y"

    run_gx clean -y

    assert_exit_code "exits 0" 0
    assert_output_not_contains "no prompt" "[y/N]"
    assert_local_branch_not_exists "branch pruned" "feature/short-y"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# C05 — answers are case-insensitive
# ======================================================================
# Contract: C05
# Class: SPEC
# Catches: "YES" being treated as a decline
test_c05_yes_case_insensitive() {
    echo ""
    echo "=== C05: YES is accepted ==="
    setup_clean_repo
    create_merged_branch "feature/yes-case"

    run_gx_stdin "YES" clean

    assert_exit_code "exits 0" 0
    assert_local_branch_not_exists "branch pruned by YES" "feature/yes-case"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# C06 — EOF at the second prompt declines only the remote scope
# ======================================================================
# Contract: C06
# Class: SPEC
# Catches: hanging on EOF or treating EOF as yes for remote deletion
test_c06_eof_second_prompt() {
    echo ""
    echo "=== C06: EOF at second prompt declines remote ==="
    setup_clean_repo
    create_merged_branch "feature/eof"

    run_gx_stdin "y" clean --remote

    assert_exit_code "exits 0" 0
    assert_output_contains "remote prompt shown" "Delete 1 remote branch"
    assert_local_branch_not_exists "local pruned after first yes" "feature/eof"
    assert_remote_tracking_not_exists "tracking pruned" "feature/eof"
    assert_remote_branch_exists "remote kept on EOF" "feature/eof"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# C07 — fetch happens before candidate selection
# ======================================================================
# Contract: C07
# Class: INVARIANT
# Catches: clean deciding from stale local remote-tracking refs, missing a
#          branch merged on the remote since the last fetch
test_c07_fetch_before_decide() {
    echo ""
    echo "=== C07: fetch before deciding ==="
    setup_clean_repo
    old_develop=$(cd "$TEST_DIR" && git rev-parse develop)

    cd "$OTHER_DIR"
    git pull -q --ff-only origin develop >/dev/null 2>&1 || true
    git checkout -b feature/stale-view develop
    echo "remote branch" > remote_branch.txt
    git add remote_branch.txt
    git_commit -m "remote branch work"
    git push -u origin feature/stale-view
    git_checkout develop
    git merge --no-ff feature/stale-view -m "merge remote branch" >/dev/null 2>&1
    git push origin develop
    cd "$TEST_DIR"

    assert_remote_tracking_not_exists "pre: tracking not fetched yet" "feature/stale-view"

    run_gx clean --yes --remote

    assert_exit_code "exits 0" 0
    assert_output_contains "pruned summary" "Pruned"
    assert_remote_branch_not_exists "new remote branch discovered and deleted" "feature/stale-view"
    assert_remote_tracking_not_exists "new tracking ref pruned" "feature/stale-view"

    local_develop_after=$(cd "$TEST_DIR" && git rev-parse develop)
    if [ "$local_develop_after" = "$old_develop" ]; then
        pass "local base not advanced by clean"
    else
        fail "local base not advanced by clean — clean moved develop"
    fi
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# C08 — no remote mutation without --remote
# ======================================================================
# Contract: C08
# Class: INVARIANT
# Catches: clean pushing deletions to the shared remote by default
test_c08_no_remote_mutation() {
    echo ""
    echo "=== C08: no remote mutation without --remote ==="
    setup_clean_repo
    create_merged_branch "feature/local-only"

    snapshot_origin_refs
    run_gx clean --yes

    assert_exit_code "exits 0" 0
    assert_origin_refs_unchanged "remote refs untouched"
    assert_remote_branch_exists "remote branch still exists" "feature/local-only"
    assert_local_branch_not_exists "local branch pruned" "feature/local-only"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# C09 — missing local base refuses before deleting anything
# ======================================================================
# Contract: C09
# Class: SPEC
# Catches: clean deleting candidates when it cannot establish the merged set
test_c09_local_base_missing() {
    echo ""
    echo "=== C09: missing local base refuses ==="
    setup_clean_repo
    create_merged_branch "feature/merged"
    git_checkout feature/merged
    git branch -D develop >/dev/null 2>&1

    snapshot_state
    run_gx clean --yes

    assert_exit_nonzero "exits non-zero"
    assert_output_contains "base error surfaced" "merged local"
    assert_local_branch_exists "candidate untouched" "feature/merged"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# C10 — missing remote base refuses before deleting anything
# ======================================================================
# Contract: C10
# Class: SPEC
# Catches: falling back to the local base for remote decisions when the
#          configured remote base is gone
test_c10_remote_base_missing() {
    echo ""
    echo "=== C10: missing remote base refuses ==="
    setup_clean_repo
    create_merged_branch "feature/merged"
    git_push origin --delete develop >/dev/null 2>&1 || true

    snapshot_state
    snapshot_origin_refs
    run_gx clean --yes

    assert_exit_nonzero "exits non-zero"
    assert_output_contains "remote base error surfaced" "remote-tracking"
    assert_local_branch_exists "candidate untouched" "feature/merged"
    assert_origin_refs_unchanged "remote refs untouched"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# C11 — dirty working tree is preserved exactly
# ======================================================================
# Contract: C11
# Class: SPEC
# Catches: clean stashing, resetting, or overwriting uncommitted changes
test_c11_dirty_preserved() {
    echo ""
    echo "=== C11: dirty working tree preserved ==="
    setup_clean_repo
    create_merged_branch "feature/dirty-clean"
    git_checkout develop
    echo "dirty edit" >> readme.md

    snapshot_state
    run_gx clean --yes

    assert_exit_code "exits 0" 0
    assert_local_branch_not_exists "merged branch pruned" "feature/dirty-clean"
    assert_worktree_preserved "dirty content preserved exactly"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# C12 — branch checked out in another worktree is never deleted
# ======================================================================
# Contract: C12
# Class: CHARACTERIZATION
# Catches: force-deleting a branch that git protects because a worktree uses it.
# Worktree branches are excluded explicitly by id and reported via a warning;
# their remote-tracking refs remain prunable by design.
test_c12_worktree_branch() {
    echo ""
    echo "=== C12: worktree branch is never deleted ==="
    setup_clean_repo
    create_merged_branch "feature/worktree"
    git checkout -f develop >/dev/null 2>&1 || true
    git worktree add "$TEST_ROOT/wt" feature/worktree >/dev/null 2>&1

    run_gx clean --yes

    assert_exit_code "exits 0" 0
    assert_local_branch_exists "worktree branch kept" "feature/worktree"
    assert_remote_tracking_not_exists "tracking ref still pruned" "feature/worktree"
    assert_remote_branch_exists "remote branch kept without --remote" "feature/worktree"
    assert_output_contains "worktree skip explained" "worktree"
    assert_output_contains "reports the tracking ref it pruned" "Pruned 1"
    assert_output_not_contains "local branch not deleted" "Deleted branch feature/worktree"
    if [ -d "$TEST_ROOT/wt" ]; then
        pass "worktree still present"
    else
        fail "worktree still present — worktree directory removed"
    fi
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"

    git worktree remove --force "$TEST_ROOT/wt" >/dev/null 2>&1 || true
}

# ======================================================================
# C13 — a branch already deleted remotely is pruned without a failure
# ======================================================================
# Contract: C13
# Class: SPEC
# Catches: stale remote-tracking refs surviving fetch --prune, then failing
# `push --delete` with a misleading partial-failure summary
test_c13_stale_remote_ref() {
    echo ""
    echo "=== C13: branch already deleted remotely ==="
    setup_clean_repo
    create_merged_branch "feature/stale-remote"

    cd "$OTHER_DIR"
    git push origin --delete feature/stale-remote >/dev/null 2>&1
    cd "$TEST_DIR"

    # The stale tracking ref exists before clean; fetch --prune must remove it
    # so it never becomes a remote deletion candidate.
    assert_remote_tracking_exists "pre: stale tracking ref present" "feature/stale-remote"

    run_gx clean --yes --remote

    assert_exit_code "exits 0" 0
    assert_output_not_contains "no spurious failure" "Failed to prune"
    assert_local_branch_not_exists "local merged branch pruned" "feature/stale-remote"
    assert_remote_tracking_not_exists "stale tracking ref pruned" "feature/stale-remote"
    assert_remote_branch_not_exists "remote branch absent" "feature/stale-remote"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# C14 — unmerged remote branch is never deleted
# ======================================================================
# Contract: C14
# Class: SPEC
# Catches: --yes deleting remote branches that still carry unmerged work
test_c14_unmerged_remote_kept() {
    echo ""
    echo "=== C14: unmerged remote branch kept ==="
    setup_clean_repo
    create_unmerged_branch "feature/unmerged"

    run_gx clean --yes --remote

    assert_exit_code "exits 0" 0
    assert_output_contains "nothing pruned" "Pruned 0"
    assert_local_branch_exists "local unmerged kept" "feature/unmerged"
    assert_remote_tracking_exists "tracking kept" "feature/unmerged"
    assert_remote_branch_exists "remote unmerged kept" "feature/unmerged"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# C15 — a second clean stays safe (fetch re-creates the tracking ref)
# ======================================================================
# Contract: C15
# Class: INVARIANT
# Catches: a second clean failing, moving HEAD, or deleting the remote branch
# Note: without --remote the remote branch survives, so the next fetch
# re-creates its tracking ref and clean prunes it again. The count is not zero.
test_c15_idempotent() {
    echo ""
    echo "=== C15: repeated clean stays safe ==="
    setup_clean_repo
    create_merged_branch "feature/idem-clean"
    head_before=$(git_head)

    run_gx clean --yes
    assert_exit_code "first clean exits 0" 0
    assert_local_branch_not_exists "branch pruned" "feature/idem-clean"

    run_gx clean --yes
    assert_exit_code "second clean exits 0" 0
    assert_local_branch_not_exists "branch stays pruned" "feature/idem-clean"
    assert_remote_branch_exists "remote branch still untouched" "feature/idem-clean"
    assert_remote_tracking_not_exists "re-created tracking ref pruned again" "feature/idem-clean"
    assert_head_unchanged "HEAD unchanged" "$head_before"
    assert_git_valid "repository intact"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit/output consistent"
}

# ======================================================================
# C16 — nothing to prune: no prompt, success
# ======================================================================
# Contract: C16
# Class: SPEC
# Catches: prompting or failing when there are no candidates
test_c16_nothing_to_prune() {
    echo ""
    echo "=== C16: nothing to prune ==="
    setup_clean_repo
    git_checkout develop

    snapshot_state
    run_gx clean --yes

    assert_exit_code "exits 0" 0
    assert_output_contains "reports zero" "Pruned 0"
    assert_output_not_contains "no prompt" "[y/N]"
    assert_state_unchanged "state unchanged"
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
echo "  gx clean — deep reliability suite"
echo "============================================"
echo ""

# `|| true` keeps `set -e` from aborting the suite at the first failure.
run_test test_c01_custom_protected "C01: custom protectedBranches honored" || true
run_test test_c02_current_tracking "C02: current local kept, tracking pruned" || true
run_test test_c03_local_ahead "C03: unmerged local branch kept" || true
run_test test_c04_short_yes "C04: -y short flag" || true
run_test test_c05_yes_case_insensitive "C05: YES accepted" || true
run_test test_c06_eof_second_prompt "C06: EOF at second prompt declines remote" || true
run_test test_c07_fetch_before_decide "C07: fetch before deciding" || true
run_test test_c08_no_remote_mutation "C08: no remote mutation without --remote" || true
run_test test_c09_local_base_missing "C09: missing local base refuses" || true
run_test test_c10_remote_base_missing "C10: missing remote base refuses" || true
run_test test_c11_dirty_preserved "C11: dirty working tree preserved" || true
run_test test_c12_worktree_branch "C12: worktree branch never deleted" || true
run_test test_c13_stale_remote_ref "C13: stale remote ref fails safely" || true
run_test test_c14_unmerged_remote_kept "C14: unmerged remote branch kept" || true
run_test test_c15_idempotent "C15: clean --yes is idempotent" || true
run_test test_c16_nothing_to_prune "C16: nothing to prune" || true

echo ""
print_test_summary

cleanup_tempdir

[ "$TEST_FAIL" -eq 0 ]
