#!/bin/bash
# gx clean — comprehensive test suite
# Usage: bash tests/gx-clean-test.sh
# Re-runnable: cleans up and recreates everything each run.
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

# ======================================================================
# Helpers
# ======================================================================

# run_gx_stdin runs gx with a given stdin string and captures output.
run_gx_stdin() {
    local stdin_input="$1"
    shift
    local saved_opts
    saved_opts=$(set +o)
    set +e
    (
        cd "${GX_RUN_DIR:-$TEST_DIR}" >/dev/null 2>&1 || exit 1
        echo "$stdin_input" | "$GX_BIN" "$@"
    ) >"$TEST_ROOT/gx-out.txt" 2>&1
    GX_STATUS=$?
    eval "$saved_opts"
    GX_OUTPUT=$(cat "$TEST_ROOT/gx-out.txt")
}

setup_clean_repo() {
    setup_tempdir
    mkdir -p "$ORIGIN_DIR"
    cd "$ORIGIN_DIR"
    git init --bare 2>/dev/null
    git symbolic-ref HEAD refs/heads/develop 2>/dev/null || true

    mkdir -p "$TEST_DIR"
    git clone "$ORIGIN_DIR" "$TEST_DIR" 2>/dev/null
    cd "$TEST_DIR"
    configure_git
    git checkout -b develop
    echo "initial" > readme.md
    git add readme.md && git_commit -m "init"
    git_push -u origin develop

    # Ensure origin HEAD points to develop
    git --git-dir="$ORIGIN_DIR" symbolic-ref HEAD refs/heads/develop 2>/dev/null || true

    write_config "develop" "origin" "rebase"
    echo ".gx/" >> "$TEST_DIR/.git/info/exclude"

    # Create OTHER_DIR as second clone (collaborator)
    git clone "$ORIGIN_DIR" "$OTHER_DIR" 2>/dev/null
    cd "$OTHER_DIR"
    configure_git
    git checkout develop >/dev/null 2>&1
    cd "$TEST_DIR"
}

# Helper: create a feature branch, push it, then merge into develop and push develop
# Usage: create_merged_branch <branch_name> [workdir]
create_merged_branch() {
    local branch="$1"
    local wdir="${2:-$TEST_DIR}"
    cd "$wdir"
    git_checkout -b "$branch" 2>/dev/null || git checkout -b "$branch" >/dev/null 2>&1
    echo "$branch work" > "${branch//\//_}.txt"
    git add "${branch//\//_}.txt" && git_commit -m "$branch work"
    git_push -u origin "$branch" 2>/dev/null || git_push origin "$branch" >/dev/null 2>&1
    git_checkout develop >/dev/null 2>&1
    git merge --no-ff "$branch" -m "merge $branch" >/dev/null 2>&1
    git_push origin develop >/dev/null 2>&1
}

# Helper: create a branch and push but do NOT merge (unmerged)
create_unmerged_branch() {
    local branch="$1"
    local wdir="${2:-$TEST_DIR}"
    cd "$wdir"
    git_checkout -b "$branch" 2>/dev/null || git checkout -b "$branch" >/dev/null 2>&1
    echo "$branch work" > "${branch//\//_}_un.txt"
    git add "${branch//\//_}_un.txt" && git_commit -m "$branch work unmerged"
    git_push -u origin "$branch" >/dev/null 2>&1
    git_checkout develop >/dev/null 2>&1
}

# Assertions for branch existence
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
    local remote="${3:-origin}"
    if (cd "$TEST_DIR" && git rev-parse --verify --quiet "refs/remotes/$remote/$branch" >/dev/null 2>&1); then
        pass "$label"
    else
        fail "$label — expected remote-tracking ref '$remote/$branch' to exist"
    fi
}

assert_remote_tracking_not_exists() {
    local label="$1" branch="$2"
    local remote="${3:-origin}"
    if (cd "$TEST_DIR" && git rev-parse --verify --quiet "refs/remotes/$remote/$branch" >/dev/null 2>&1); then
        fail "$label — expected remote-tracking ref '$remote/$branch' NOT to exist"
    else
        pass "$label"
    fi
}

assert_remote_branch_exists() {
    local label="$1" branch="$2"
    if git --git-dir="$ORIGIN_DIR" show-ref --verify --quiet "refs/heads/$branch" 2>/dev/null; then
        pass "$label"
    else
        fail "$label — expected remote branch '$branch' to exist on origin"
    fi
}

assert_remote_branch_not_exists() {
    local label="$1" branch="$2"
    if git --git-dir="$ORIGIN_DIR" show-ref --verify --quiet "refs/heads/$branch" 2>/dev/null; then
        fail "$label — expected remote branch '$branch' NOT to exist on origin"
    else
        pass "$label"
    fi
}

# ======================================================================
# TEST 01 — Prune one merged local branch
# ======================================================================
test_01_prune_merged_local() {
    echo ""
    echo "=== Test 01: prune one merged local branch ==="
    setup_clean_repo
    cd "$TEST_DIR"
    create_merged_branch "feature/merged-local"
    # Return to develop so we are not on the branch to be pruned (also tests current-branch filter not matching)
    git_checkout develop >/dev/null 2>&1

    # Verify branch exists before clean
    assert_local_branch_exists "pre: local branch exists" "feature/merged-local"

    run_gx clean

    assert_exit_code "exits 0" 0
    assert_output_contains "Pruned" "Pruned"
    assert_local_branch_not_exists "local merged branch pruned" "feature/merged-local"
    assert_git_valid "repo integrity"
}

# ======================================================================
# TEST 02 — Protected branch not pruned
# ======================================================================
test_02_protected_not_pruned() {
    echo ""
    echo "=== Test 02: protected branch not pruned ==="
    setup_clean_repo
    cd "$TEST_DIR"
    # Create a branch named main that is merged into develop (edge: main is protected but we will try to prune)
    # We need to create main locally as a feature branch merged into develop (not the real main branch flow)
    # Instead, use a protected name like "feature/protected-test" and set protectedBranches to include it, but simpler:
    # Create a branch named "main" from develop, merge it into develop, then clean should not delete main
    git_checkout -b main 2>/dev/null || git checkout -b main >/dev/null 2>&1
    echo "main work" > main.txt
    git add main.txt && git_commit -m "main work"
    git_push origin main >/dev/null 2>&1 || true
    git_checkout develop >/dev/null 2>&1
    git merge --no-ff main -m "merge main" >/dev/null 2>&1
    git_push origin develop >/dev/null 2>&1

    # Ensure main still exists before clean
    assert_local_branch_exists "pre: protected branch exists" "main"

    run_gx clean

    assert_exit_code "exits 0" 0
    assert_local_branch_exists "protected branch still exists" "main"
    assert_git_valid "repo integrity"
}

# ======================================================================
# TEST 03 — Current branch not pruned
# ======================================================================
test_03_current_not_pruned() {
    echo ""
    echo "=== Test 03: current branch not pruned ==="
    setup_clean_repo
    cd "$TEST_DIR"
    create_merged_branch "feature/current-merged"
    # Checkout the merged branch itself (current)
    git_checkout feature/current-merged >/dev/null 2>&1

    run_gx clean

    assert_exit_code "exits 0" 0
    assert_local_branch_exists "current branch not pruned" "feature/current-merged"
    assert_branch "still on current branch" "feature/current-merged"
    assert_git_valid "repo integrity"
}

# ======================================================================
# TEST 04 — Default branch not pruned
# ======================================================================
test_04_default_not_pruned() {
    echo ""
    echo "=== Test 04: default branch not pruned ==="
    setup_clean_repo
    cd "$TEST_DIR"
    # develop is default; ensure it exists
    git_checkout develop >/dev/null 2>&1
    assert_local_branch_exists "pre: default branch exists" "develop"

    run_gx clean

    assert_exit_code "exits 0" 0
    assert_local_branch_exists "default branch still exists" "develop"
    assert_branch "still on develop" "develop"
}

# ======================================================================
# TEST 05 — Unmerged branch not pruned
# ======================================================================
test_05_unmerged_not_pruned() {
    echo ""
    echo "=== Test 05: unmerged branch not pruned ==="
    setup_clean_repo
    cd "$TEST_DIR"
    create_unmerged_branch "feature/unmerged"

    assert_local_branch_exists "pre: unmerged branch exists" "feature/unmerged"

    run_gx clean

    assert_exit_code "exits 0" 0
    assert_local_branch_exists "unmerged branch still exists" "feature/unmerged"
}

# ======================================================================
# TEST 06 — Remote-tracking ref pruned (no --remote)
# ======================================================================
test_06_remote_tracking_pruned() {
    echo ""
    echo "=== Test 06: remote-tracking ref pruned ==="
    setup_clean_repo
    cd "$TEST_DIR"
    create_merged_branch "feature/remote-track"
    git_checkout develop >/dev/null 2>&1

    assert_remote_tracking_exists "pre: remote-tracking exists" "feature/remote-track"

    run_gx clean

    assert_exit_code "exits 0" 0
    assert_output_contains "Pruned" "Pruned"
    assert_remote_tracking_not_exists "remote-tracking ref pruned" "feature/remote-track"
    # Remote branch still exists (no --remote)
    assert_remote_branch_exists "remote branch still exists (no --remote)" "feature/remote-track"
}

# ======================================================================
# TEST 07 — --remote without confirming (decline)
# ======================================================================
test_07_remote_decline() {
    echo ""
    echo "=== Test 07: --remote decline (n) ==="
    setup_clean_repo
    cd "$TEST_DIR"
    create_merged_branch "feature/remote-decline"
    git_checkout develop >/dev/null 2>&1

    assert_remote_branch_exists "pre: remote branch exists" "feature/remote-decline"

    run_gx_stdin "n" clean --remote

    assert_exit_code "exits 0" 0
    assert_output_contains "Pruned" "Pruned"
    assert_remote_branch_exists "remote branch still intact after decline" "feature/remote-decline"
    # Local and remote-tracking should still be pruned even when remote declined
    assert_local_branch_not_exists "local branch pruned even on decline" "feature/remote-decline"
    assert_remote_tracking_not_exists "remote-tracking pruned even on decline" "feature/remote-decline"
}

# ======================================================================
# TEST 08 — --remote with confirming (yes)
# ======================================================================
test_08_remote_confirm() {
    echo ""
    echo "=== Test 08: --remote confirm (y) ==="
    setup_clean_repo
    cd "$TEST_DIR"
    create_merged_branch "feature/remote-confirm"
    git_checkout develop >/dev/null 2>&1

    assert_remote_branch_exists "pre: remote branch exists" "feature/remote-confirm"

    run_gx_stdin "y" clean --remote

    assert_exit_code "exits 0" 0
    assert_output_contains "Pruned" "Pruned"
    assert_remote_branch_not_exists "remote branch deleted after confirm" "feature/remote-confirm"
    assert_local_branch_not_exists "local branch pruned" "feature/remote-confirm"
    assert_remote_tracking_not_exists "remote-tracking pruned" "feature/remote-confirm"
}

# ======================================================================
# TEST 09 — Collaborator with no local branch still prunes remote
# ======================================================================
test_09_no_local_still_prunes_remote() {
    echo ""
    echo "=== Test 09: no local branch still prunes remote ==="
    setup_clean_repo

    # Create merged branch in OTHER_DIR (collaborator), not in TEST_DIR
    cd "$OTHER_DIR"
    git_checkout develop >/dev/null 2>&1
    create_merged_branch "feature/no-local" "$OTHER_DIR"

    # Fetch in TEST_DIR so we have remote-tracking ref but no local branch
    cd "$TEST_DIR"
    git_fetch origin >/dev/null 2>&1

    # Verify TEST_DIR has remote-tracking but no local branch
    assert_local_branch_not_exists "pre: no local branch" "feature/no-local"
    assert_remote_tracking_exists "pre: remote-tracking exists" "feature/no-local"
    assert_remote_branch_exists "pre: remote branch exists" "feature/no-local"

    run_gx_stdin "y" clean --remote

    assert_exit_code "exits 0" 0
    assert_remote_branch_not_exists "remote branch deleted" "feature/no-local"
    assert_remote_tracking_not_exists "remote-tracking pruned" "feature/no-local"
}

# ======================================================================
# TEST 10 — Dirty working tree still cleans
# ======================================================================
test_10_dirty_tree() {
    echo ""
    echo "=== Test 10: dirty working tree ==="
    setup_clean_repo
    cd "$TEST_DIR"
    create_merged_branch "feature/dirty-test"
    git_checkout develop >/dev/null 2>&1

    # Make dirty
    echo "dirty content" >> readme.md

    run_gx clean

    assert_exit_code "exits 0" 0
    assert_output_contains "Pruned" "Pruned"
    assert_local_branch_not_exists "merged branch pruned despite dirty" "feature/dirty-test"
    # Dirty file intact
    cd "$TEST_DIR"
    grep -q "dirty content" readme.md && pass "dirty file intact" || fail "dirty file was lost"
}

# ======================================================================
# TEST 11 — Outside git repo
# ======================================================================
test_11_outside_repo() {
    echo ""
    echo "=== Test 11: outside git repo ==="
    setup_clean_repo

    GX_RUN_DIR=/tmp run_gx clean

    assert_exit_code "exits 1" 1
    assert_output_contains "Git repository" "Git repository"
}

# ======================================================================
# TEST 12 — No config
# ======================================================================
test_12_no_config() {
    echo ""
    echo "=== Test 12: no config ==="
    setup_clean_repo

    cd "$TEST_DIR"
    rm -rf "$TEST_DIR/.gx"

    run_gx clean

    assert_exit_code "exits 1" 1
    assert_output_contains "gx init" "gx init"
}

# ======================================================================
# TEST 13 — Nothing to prune
# ======================================================================
test_13_nothing_to_prune() {
    echo ""
    echo "=== Test 13: nothing to prune ==="
    setup_clean_repo
    cd "$TEST_DIR"
    git_checkout develop >/dev/null 2>&1

    run_gx clean

    assert_exit_code "exits 0" 0
    assert_output_contains "Pruned 0" "Pruned 0"
}

# ======================================================================
# TEST 14 — Protected branch remote-tracking not pruned
# ======================================================================
test_14_protected_remote_tracking() {
    echo ""
    echo "=== Test 14: protected remote-tracking not pruned ==="
    setup_clean_repo
    cd "$TEST_DIR"
    # Create a branch named main merged into develop — its remote-tracking origin/main should be protected
    git_checkout -b main-temp 2>/dev/null || git checkout -b main-temp >/dev/null 2>&1
    echo "main temp" > main-temp.txt
    git add main-temp.txt && git_commit -m "main temp"
    # Create protected-like: push a branch named main
    git checkout -b main 2>/dev/null || git branch main main-temp >/dev/null 2>&1
    # Ensure origin/main exists by pushing
    git_checkout main >/dev/null 2>&1
    git_push origin main >/dev/null 2>&1 || true
    git_checkout develop >/dev/null 2>&1
    # Merge main into develop to make it appear merged
    git merge --no-ff main -m "merge main" >/dev/null 2>&1 || true
    git_push origin develop >/dev/null 2>&1 || true
    git_fetch origin >/dev/null 2>&1

    assert_remote_tracking_exists "pre: origin/main exists" "main"

    run_gx clean

    assert_exit_code "exits 0" 0
    assert_remote_tracking_exists "protected remote-tracking not pruned" "main"
}

# ======================================================================
# TEST 15 — --remote empty input defaults to decline
# ======================================================================
test_15_remote_empty_defaults_decline() {
    echo ""
    echo "=== Test 15: --remote empty input defaults to decline ==="
    setup_clean_repo
    cd "$TEST_DIR"
    create_merged_branch "feature/empty-decline"
    git_checkout develop >/dev/null 2>&1

    assert_remote_branch_exists "pre: remote branch exists" "feature/empty-decline"

    run_gx_stdin "" clean --remote

    assert_exit_code "exits 0" 0
    assert_remote_branch_exists "remote branch still intact (empty = decline)" "feature/empty-decline"
}

# ======================================================================
# TEST 16 — --remote confirmation lists exact branches (Option A)
# ======================================================================
test_16_remote_confirm_lists_branches() {
    echo ""
    echo "=== Test 16: --remote confirm lists branches ==="
    setup_clean_repo
    cd "$TEST_DIR"
    create_merged_branch "feature/list-a"
    create_merged_branch "feature/list-b"
    git_checkout develop >/dev/null 2>&1

    run_gx_stdin "n" clean --remote

    assert_exit_code "exits 0" 0
    assert_output_contains "shows origin/list-a" "origin/feature/list-a"
    assert_output_contains "shows origin/list-b" "origin/feature/list-b"
    assert_output_contains "shows [y/N]" "[y/N]"
}

# ======================================================================
# TEST 17 — Remote candidates checked against <remote>/<defaultBranch> not local
# ======================================================================
test_17_remote_candidates_use_remote_base() {
    echo ""
    echo "=== Test 17: remote candidates use remote base ==="
    setup_clean_repo
    cd "$TEST_DIR"
    # Create a branch feature/only-local-merge that we merge locally but NOT push develop
    git_checkout -b feature/only-local-merge develop >/dev/null 2>&1
    echo "only local" > only-local.txt
    git add only-local.txt && git_commit -m "only local merge work"
    git_push -u origin feature/only-local-merge >/dev/null 2>&1
    git_checkout develop >/dev/null 2>&1
    git merge --no-ff feature/only-local-merge -m "merge only-local" >/dev/null 2>&1
    # Do NOT push develop — so remote develop does NOT contain the merge
    # The remote-tracking ref should NOT be considered merged into origin/develop
    # Therefore gx clean should NOT prune it (remote-tracking or remote)
    assert_remote_tracking_exists "pre: remote-tracking exists" "feature/only-local-merge"

    run_gx clean

    assert_exit_code "exits 0" 0
    # Remote-tracking should NOT be pruned because it's not merged into origin/develop
    assert_remote_tracking_exists "remote-tracking not pruned (not merged on remote base)" "feature/only-local-merge"
    # Local SHOULD be pruned (merged into local develop)
    assert_local_branch_not_exists "local branch pruned (merged into local base)" "feature/only-local-merge"

    # Now push develop and retry — this time remote-tracking SHOULD be pruned
    git_push origin develop >/dev/null 2>&1
    run_gx clean
    assert_exit_code "exits 0 on second clean" 0
    assert_remote_tracking_not_exists "remote-tracking pruned after pushing develop" "feature/only-local-merge"
}

# ======================================================================
# RUN ALL
# ======================================================================
build_gx
trap cleanup_tempdir EXIT

echo "============================================"
echo "  gx clean — test suite"
echo "============================================"
echo ""

run_test test_01_prune_merged_local "Test 01: prune one merged local branch"
run_test test_02_protected_not_pruned "Test 02: protected branch not pruned"
run_test test_03_current_not_pruned "Test 03: current branch not pruned"
run_test test_04_default_not_pruned "Test 04: default branch not pruned"
run_test test_05_unmerged_not_pruned "Test 05: unmerged branch not pruned"
run_test test_06_remote_tracking_pruned "Test 06: remote-tracking ref pruned"
run_test test_07_remote_decline "Test 07: --remote decline"
run_test test_08_remote_confirm "Test 08: --remote confirm"
run_test test_09_no_local_still_prunes_remote "Test 09: no local prunes remote"
run_test test_10_dirty_tree "Test 10: dirty working tree"
run_test test_11_outside_repo "Test 11: outside repo"
run_test test_12_no_config "Test 12: no config"
run_test test_13_nothing_to_prune "Test 13: nothing to prune"
run_test test_14_protected_remote_tracking "Test 14: protected remote-tracking not pruned"
run_test test_15_remote_empty_defaults_decline "Test 15: empty input defaults decline"
run_test test_16_remote_confirm_lists_branches "Test 16: confirm lists branches"
run_test test_17_remote_candidates_use_remote_base "Test 17: remote candidates use remote base"

echo ""
print_test_summary

cleanup_tempdir

[ "$TEST_FAIL" -eq 0 ]
