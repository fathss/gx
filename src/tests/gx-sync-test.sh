#!/bin/bash
# gx sync — comprehensive test suite
# Usage: bash tests/gx-sync-test.sh
# Re-runnable: cleans up and recreates everything each run.
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

# ======================================================================
# TEST 01 — success: feature branch (full rebase flow)
# ======================================================================
test_01_success_feature() {
    echo ""
    echo "=== Test 01: success — feature branch (full rebase flow) ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git_checkout feature/test
    old_head=$(git_head)
    run_gx sync

    assert_exit_code "exits 0" 0
    assert_output_not_contains "no error indicators" "✗"
    assert_branch "still on feature/test" "feature/test"
    assert_clean "working tree clean"
    assert_no_rebase_in_progress "no rebase lingering"
    assert_merge_base_equals "develop is ancestor of feature" "develop" "feature/test"
    assert_head_changed "HEAD changed after rebase" "$old_head"
    assert_git_valid "repository intact"
}

# ======================================================================
# TEST 02 — outside repo
# ======================================================================
test_02_outside_repo() {
    echo ""
    echo "=== Test 02: outside repo ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    GX_RUN_DIR=/tmp run_gx sync

    assert_exit_code "exits non-zero" 1
    assert_output_contains "message: Not inside a Git repository" "Git repository"
    assert_output_contains "hint given" "cloned repository"
}

# ======================================================================
# TEST 03 — on base branch (pull-only flow)
# ======================================================================
test_03_base_branch() {
    echo ""
    echo "=== Test 03: base branch (pull only) ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git_checkout develop
    run_gx sync

    assert_exit_code "exits 0" 0
    assert_output_not_contains "no rebase" "Rebasing"
    assert_branch "still on develop" "develop"
    assert_clean "working tree clean"
    assert_no_rebase_in_progress "no rebase lingering"
    assert_git_valid "repository intact"
}

# ======================================================================
# TEST 04 — remote offline
# ======================================================================
test_04_remote_offline() {
    echo ""
    echo "=== Test 04: remote offline ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git remote set-url origin /nonexistent/path
    run_gx sync

    assert_exit_code "exits non-zero" 1
    assert_output_contains "friendly fetch error" "fetch remote"
    assert_output_contains "hint about network" "Check network"
}

# ======================================================================
# TEST 05 — merge conflict (rebase left in progress)
# ======================================================================
test_05_merge_conflict() {
    echo ""
    echo "=== Test 05: merge conflict (rebase left in progress) ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"

    # Create a conflict: both develop and feature touch the same file
    git_checkout develop
    git_pull --ff-only origin develop
    echo "develop content" > conflict.txt
    git add conflict.txt && git_commit -m "develop conflict"
    git_push origin develop

    git_checkout feature/test
    git_fetch origin feature/test
    git rebase origin/feature/test >/dev/null 2>&1
    echo "feature content" > conflict.txt
    git add conflict.txt && git_commit -m "feature conflict"

    run_gx sync

    assert_exit_code "exits non-zero" 1
    assert_output_contains "conflict message" "conflicts"
    assert_output_contains "conflicted files" "Conflicted files"
    assert_rebase_in_progress "rebase left in progress"
}

# ======================================================================
# TEST 06 — base branch missing (checkout error)
# ======================================================================
test_06_branch_missing() {
    echo ""
    echo "=== Test 06: branch missing ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    write_config "nonexistent" "origin" "rebase"
    run_gx sync

    assert_exit_code "exits non-zero" 1
    assert_output_contains "branch not found" "does not exist"
    assert_output_contains "create branch hint" "git checkout -b"
}

# ======================================================================
# TEST 07 — dirty working tree (stash-transparent success)
# ======================================================================
test_07_dirty_working_tree() {
    echo ""
    echo "=== Test 07: dirty working tree (stash-transparent success) ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git_checkout feature/test
    echo "dirty change" >> readme.md   # uncommitted

    run_gx sync

    assert_exit_code "exits 0" 0
    assert_output_contains "stash push" "Stashing"
    assert_output_contains "stash pop" "stashed"
    assert_output_not_contains "no error message" "Error"
    assert_branch "still on feature/test" "feature/test"
    assert_dirty "dirty change restored after stashing"
}

# ======================================================================
# TEST 08 — detached HEAD
# ======================================================================
test_08_detached_head() {
    echo ""
    echo "=== Test 08: detached HEAD ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git checkout --detach >/dev/null 2>&1
    run_gx sync

    assert_exit_code "exits non-zero" 1
    assert_output_contains "detached HEAD error" "Detached HEAD"
    assert_output_contains "hint" "Checkout"
}

# ======================================================================
# TEST 09 — base branch deleted from remote (ff fails)
# ======================================================================
test_09_base_branch_deleted() {
    echo ""
    echo "=== Test 09: base branch deleted from remote ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git_push origin --delete develop 2>/dev/null || true
    run_gx sync

    assert_exit_code "exits non-zero" 1
    assert_output_contains "branch not found error" "not found"
    assert_output_contains "deleted hint" "deleted"
    assert_branch "restored to feature/test" "feature/test"
}

# ======================================================================
# TEST 10 — merge strategy
# ======================================================================
test_10_merge_strategy() {
    echo ""
    echo "=== Test 10: merge strategy ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git_checkout feature/test
    write_config "develop" "origin" "merge"

    run_gx sync

    assert_exit_code "exits 0" 0
    assert_output_not_contains "no rebase message" "Rebasing"
    assert_branch "still on feature/test" "feature/test"
    assert_clean "working tree clean"
    assert_git_valid "repository intact"
}

# ======================================================================
# TEST 11 — rebase already in progress (guard check)
# ======================================================================
test_11_rebase_in_progress() {
    echo ""
    echo "=== Test 11: rebase already in progress (guard) ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git_checkout feature/test

    # Create a conflict scenario to leave a rebase mid-way
    git_checkout develop
    git_pull --ff-only origin develop
    echo "guard-conflict-content" > conflict.txt
    git add conflict.txt && git_commit -m "develop conflict for guard"
    git_push origin develop

    git_checkout feature/test
    git_fetch origin feature/test
    git rebase origin/feature/test >/dev/null 2>&1
    echo "feature guard content" > conflict.txt
    git add conflict.txt && git_commit -m "feature conflict for guard"

    # Start rebase manually — this will conflict and leave rebase in progress
    git rebase develop >/dev/null 2>&1 || true

    run_gx sync

    assert_exit_code "exits non-zero" 1
    assert_output_contains "guard message" "already in progress"
    assert_output_contains "hint" "gx sync only manages"
    assert_output_not_contains "no detached message" "Detached HEAD"
    assert_rebase_in_progress "rebase still in progress (not aborted)"

    # Cleanup for next test
    cd "$TEST_DIR" && git rebase --abort >/dev/null 2>&1 || true
}

# ======================================================================
# TEST 12 — auto-continue after conflict resolution
# ======================================================================
test_12_auto_continue() {
    echo ""
    echo "=== Test 12: auto-continue after conflict resolution ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git_checkout feature/test

    # Create a conflict: both develop and feature touch the same file
    git_checkout develop
    git_pull --ff-only origin develop
    echo "develop content" > conflict.txt
    git add conflict.txt && git_commit -m "develop conflict"
    git_push origin develop

    git_checkout feature/test
    git_fetch origin feature/test
    git rebase origin/feature/test >/dev/null 2>&1
    echo "feature content" > conflict.txt
    git add conflict.txt && git_commit -m "feature conflict"

    # Add a dirty change so gx creates a stash entry
    echo "dirty" >> readme.md

    # First run — should hit conflict and leave rebase in progress
    run_gx sync

    assert_exit_code "first run exits non-zero" 1
    assert_output_contains "conflict message" "conflicts"
    assert_output_contains "stashed hint" "stashed"
    assert_output_contains "conflicted files" "Conflicted files"
    assert_rebase_in_progress "rebase left in progress after first run"

    # Resolve the conflict
    echo "resolved" > conflict.txt
    git add conflict.txt

    # Second run — should auto-continue, pop stash, succeed
    run_gx sync --continue

    assert_exit_code "second run exits 0" 0
    assert_output_contains "continuing" "Continuing"
    assert_output_contains "restoring stash" "stashed"
    assert_no_rebase_in_progress "rebase completed"
    assert_branch "back on feature/test" "feature/test"
    assert_dirty "dirty change restored after auto-continue"
}

# ======================================================================
# TEST 13 — auto-continue with still-unresolved conflicts
# ======================================================================
test_13_auto_continue_still_conflicted() {
    echo ""
    echo "=== Test 13: auto-continue with still-unresolved conflicts ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git_checkout feature/test

    # Create a conflict: both develop and feature touch the same file
    git_checkout develop
    git_pull --ff-only origin develop
    echo "develop content" > conflict.txt
    git add conflict.txt && git_commit -m "develop conflict"
    git_push origin develop

    git_checkout feature/test
    git_fetch origin feature/test
    git rebase origin/feature/test >/dev/null 2>&1
    echo "feature content" > conflict.txt
    git add conflict.txt && git_commit -m "feature conflict"

    # Add a dirty change so gx creates a stash entry
    echo "dirty" >> readme.md

    # First run — hits conflict, stash pushed, rebase left in progress
    run_gx sync

    assert_exit_code "first run exits non-zero" 1
    assert_rebase_in_progress "rebase left in progress"

    # Second run — DON'T resolve, auto-continue should fail
    run_gx sync --continue

    assert_exit_code "second run still exits non-zero" 1
    assert_output_contains "still unresolved" "unresolved"
    assert_output_contains "conflicted files" "Conflicted files"
    assert_rebase_in_progress "rebase still in progress"

    # Cleanup for next test
    cd "$TEST_DIR" && git rebase --abort >/dev/null 2>&1 || true
}

# ======================================================================
# TEST 14 — orphaned gx stash (no rebase)
# ======================================================================
test_14_orphaned_stash() {
    echo ""
    echo "=== Test 14: orphaned gx stash ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git_checkout feature/test

    # Create an orphaned gx stash entry
    echo "orphaned" >> readme.md
    git stash push -m "gx-sync/feature/test/99999" >/dev/null 2>&1

    run_gx sync

    assert_exit_code "exits 0 despite orphaned stash" 0
    assert_output_contains "restored message" "Restoring stashed"
    assert_branch "still on feature/test" "feature/test"
    assert_dirty "stash restored to working tree"
    assert_no_rebase_in_progress "no rebase"
}

# ======================================================================
# TEST 15 — merge conflict → resolve → auto-detect (Q6)
# ======================================================================
test_15_merge_conflict_auto_continue() {
    echo ""
    echo "=== Test 15: merge conflict → auto-continue ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git_checkout feature/test
    write_config "develop" "origin" "merge"

    # Create a conflict: develop and feature both create same file
    git_checkout develop
    git_pull --ff-only origin develop
    echo "develop line" > conflict.txt
    git add conflict.txt && git_commit -m "develop add conflict.txt"
    git_push origin develop

    git_checkout feature/test
    git_fetch origin feature/test
    git rebase origin/feature/test >/dev/null 2>&1
    echo "feature line" > conflict.txt
    git add conflict.txt && git_commit -m "feature add conflict.txt"

    # Add dirty change so gx creates a stash entry
    echo "dirty" >> readme.md

    # First run — should hit merge conflict and leave merge in progress
    run_gx sync

    assert_exit_code "first run exits non-zero" 1
    assert_output_contains "conflict message" "conflicts"
    assert_output_contains "conflicted files" "Conflicted files"
    assert_output_contains "stashed hint" "stashed"
    assert_merge_in_progress "merge left in progress"

    # Resolve the conflict
    echo "resolved" > conflict.txt
    git add conflict.txt

    # Second run — should auto-continue, pop stash, succeed
    run_gx sync --continue

    assert_exit_code "second run exits 0" 0
    assert_output_contains "continuing" "Continuing"
    assert_output_contains "restoring stash" "stashed"
    assert_no_merge_in_progress "merge completed"
    assert_branch "back on feature/test" "feature/test"
    assert_dirty "dirty change restored"
}

# ======================================================================
# TEST 16 — merge conflict → resolve → --continue
# ======================================================================
test_16_merge_conflict_continue_flag() {
    echo ""
    echo "=== Test 16: merge conflict → --continue flag ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git_checkout feature/test
    write_config "develop" "origin" "merge"

    # Create a conflict: develop and feature both create same file
    git_checkout develop
    git_pull --ff-only origin develop
    echo "develop line" > conflict.txt
    git add conflict.txt && git_commit -m "develop add conflict.txt"
    git_push origin develop

    git_checkout feature/test
    git_fetch origin feature/test
    git rebase origin/feature/test >/dev/null 2>&1
    echo "feature line" > conflict.txt
    git add conflict.txt && git_commit -m "feature add conflict.txt"

    # Add a dirty change so gx creates a stash entry (makes it Q6)
    echo "dirty" >> readme.md

    # First run — hits merge conflict, stash pushed
    run_gx sync

    assert_exit_code "first run exits non-zero" 1
    assert_merge_in_progress "merge left in progress"
    assert_output_contains "stashed hint" "stashed"

    # Resolve conflict and stage
    echo "resolved" > conflict.txt
    git add conflict.txt

    # Second run with --continue flag
    run_gx sync --continue

    assert_exit_code "second run exits 0" 0
    assert_output_contains "continuing" "Continuing"
    assert_output_contains "restoring stash" "stashed"
    assert_no_merge_in_progress "merge completed"
    assert_branch "back on feature/test" "feature/test"
    assert_dirty "dirty change restored"
}

# ======================================================================
# TEST 17 — merge conflict → don't resolve → auto-detect fails
# ======================================================================
test_17_merge_conflict_still_conflicted() {
    echo ""
    echo "=== Test 17: merge conflict still unresolved ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git_checkout feature/test
    write_config "develop" "origin" "merge"

    # Create a conflict
    git_checkout develop
    git_pull --ff-only origin develop
    echo "develop line" > conflict.txt
    git add conflict.txt && git_commit -m "develop add conflict.txt"
    git_push origin develop

    git_checkout feature/test
    git_fetch origin feature/test
    git rebase origin/feature/test >/dev/null 2>&1
    echo "feature line" > conflict.txt
    git add conflict.txt && git_commit -m "feature add conflict.txt"

    # Add a dirty change so gx creates a stash entry (triggers Q6 auto-detect)
    echo "dirty" >> readme.md

    # First run — hits merge conflict, gx stashed our dirty change
    run_gx sync

    assert_exit_code "first run exits non-zero" 1
    assert_merge_in_progress "merge left in progress"

    # Second run — DON'T resolve, auto-continue should fail
    run_gx sync --continue

    assert_exit_code "second run still exits non-zero" 1
    assert_output_contains "still unresolved" "unresolved"
    assert_output_contains "conflicted files" "Conflicted files"
    assert_merge_in_progress "merge still in progress"

    # Cleanup
    cd "$TEST_DIR" && git merge --abort >/dev/null 2>&1 || true
}

# ======================================================================
# TEST 18 — --continue on clean state (nothing to continue)
# ======================================================================
test_18_continue_nothing() {
    echo ""
    echo "=== Test 18: --continue on clean state ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git_checkout feature/test

    run_gx sync --continue

    assert_exit_code "exits non-zero" 1
    assert_output_contains "nothing to continue" "Nothing"
}

# ======================================================================
# TEST 19 — manual rebase + --continue (proxy)
# ======================================================================
test_19_manual_rebase_continue() {
    echo ""
    echo "=== Test 19: manual rebase + --continue ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git_checkout feature/test

    # Create a conflict
    git_checkout develop
    git_pull --ff-only origin develop
    echo "develop line" > conflict.txt
    git add conflict.txt && git_commit -m "develop add conflict.txt"
    git_push origin develop

    git_checkout feature/test
    git_fetch origin feature/test
    git rebase origin/feature/test >/dev/null 2>&1
    echo "feature line" > conflict.txt
    git add conflict.txt && git_commit -m "feature add conflict.txt"

    # Start rebase manually — this will conflict
    git rebase develop >/dev/null 2>&1 || true

    # gx sync (no --continue) should block with Q3
    run_gx sync

    assert_exit_code "blocked by guard" 1
    assert_output_contains "guard message" "already in progress"
    assert_rebase_in_progress "rebase still in progress"

    # Resolve and stage
    echo "resolved" > conflict.txt
    git add conflict.txt

    # gx sync --continue should block — it's a manual rebase, not gx-orchestrated
    run_gx sync --continue

    assert_exit_code "blocked by guard" 1
    assert_output_contains "guard message" "gx sync only manages"

    # Clean up the manual rebase
    cd "$TEST_DIR" && git rebase --abort >/dev/null 2>&1 || true
}

# ======================================================================
# TEST 20 — manual merge + --continue (proxy)
# ======================================================================
test_20_manual_merge_continue() {
    echo ""
    echo "=== Test 20: manual merge + --continue ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git_checkout feature/test

    # Create a conflicting commit on develop
    git_checkout develop
    git_pull --ff-only origin develop
    echo "develop line" > conflict.txt
    git add . && git_commit -m "develop add conflict.txt"
    git_push origin develop

    git_checkout feature/test
    # Fast-forward to include develop's latest
    git_fetch origin feature/test >/dev/null 2>&1

    # Create a conflicting commit on feature
    echo "feature line" > conflict.txt
    git add . && git_commit -m "feature add conflict.txt"

    # Start merge manually — this will conflict
    git merge develop >/dev/null 2>&1 || true

    # gx sync (no --continue) should block with Q5
    run_gx sync

    assert_exit_code "blocked by guard" 1
    assert_output_contains "guard message" "already in progress"
    assert_merge_in_progress "merge still in progress"

    # Resolve and stage
    echo "resolved" > conflict.txt
    git add conflict.txt

    # gx sync --continue should block — it's a manual merge, not gx-orchestrated
    run_gx sync --continue

    assert_exit_code "blocked by guard" 1
    assert_output_contains "guard message" "gx sync only manages"

    # Clean up the manual merge
    cd "$TEST_DIR" && git merge --abort >/dev/null 2>&1 || true
}

# ======================================================================
# TEST 21 — --rebase flag (overrides config merge to rebase)
# ======================================================================
test_21_flag_rebase() {
    echo ""
    echo "=== Test 21: --rebase flag overrides config ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git_checkout feature/test
    write_config "develop" "origin" "merge"   # config says merge

    run_gx sync --rebase

    assert_exit_code "exits 0" 0
    assert_output_not_contains "no merging" "Merging"
    assert_branch "still on feature/test" "feature/test"
    assert_clean "working tree clean"
    assert_no_rebase_in_progress "no rebase lingering"
    assert_merge_base_equals "develop is ancestor after rebase" "develop" "feature/test"
    local merge_count
    merge_count=$(cd "$TEST_DIR" && git rev-list --count --merges HEAD 2>/dev/null)
    [ "$merge_count" -eq 0 ] && pass "no merge commits (rebase)" || fail "expected no merge commits, got $merge_count"
    assert_git_valid "repository intact"
}

# ======================================================================
# TEST 22 — --merge flag (overrides config rebase to merge)
# ======================================================================
test_22_flag_merge() {
    echo ""
    echo "=== Test 22: --merge flag overrides config ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git_checkout feature/test
    # config default is rebase — use --merge to override

    run_gx sync --merge

    assert_exit_code "exits 0" 0
    assert_output_not_contains "no rebasing" "Rebasing"
    assert_branch "still on feature/test" "feature/test"
    assert_clean "working tree clean"
    assert_no_merge_in_progress "no merge lingering"
    local merge_count
    merge_count=$(cd "$TEST_DIR" && git rev-list --count --merges HEAD 2>/dev/null)
    [ "$merge_count" -ge 1 ] && pass "merge commit created" || fail "expected merge commit, got none"
    assert_git_valid "repository intact"
}

# ======================================================================
# TEST 23 — --rebase and --merge together (mutual exclusion error)
# ======================================================================
test_23_flag_both_exclusive() {
    echo ""
    echo "=== Test 23: --rebase + --merge (mutual exclusion) ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git_checkout feature/test

    run_gx sync --rebase --merge

    assert_exit_code "exits non-zero" 1
    assert_output_contains "mutual exclusion error" "specify both"
    assert_output_contains "hint" "one strategy"
}

# ======================================================================
# TEST 24 — --rebase with conflict (flag overrides config merge)
# ======================================================================
test_24_flag_rebase_conflict() {
    echo ""
    echo "=== Test 24: --rebase with conflict ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git_checkout feature/test
    write_config "develop" "origin" "merge"   # config says merge

    # Create a conflict
    git_checkout develop
    git_pull --ff-only origin develop
    echo "develop content" > conflict.txt
    git add conflict.txt && git_commit -m "develop conflict"
    git_push origin develop

    git_checkout feature/test
    git_fetch origin feature/test
    git rebase origin/feature/test >/dev/null 2>&1
    echo "feature content" > conflict.txt
    git add conflict.txt && git_commit -m "feature conflict"

    run_gx sync --rebase

    assert_exit_code "exits non-zero" 1
    assert_output_contains "conflict via rebase flag" "conflicts"
    assert_output_contains "conflicted files" "Conflicted files"
    assert_output_not_contains "no merge message" "Merge stopped"
    assert_rebase_in_progress "rebase left in progress"

    cd "$TEST_DIR" && git rebase --abort >/dev/null 2>&1 || true
}

# ======================================================================
# TEST 25 — --merge with conflict (flag overrides config rebase)
# ======================================================================
test_25_flag_merge_conflict() {
    echo ""
    echo "=== Test 25: --merge with conflict ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git_checkout feature/test
    # config default is rebase — use --merge to override

    # Create a conflict
    git_checkout develop
    git_pull --ff-only origin develop
    echo "develop content" > conflict.txt
    git add conflict.txt && git_commit -m "develop conflict"
    git_push origin develop

    git_checkout feature/test
    git_fetch origin feature/test
    git rebase origin/feature/test >/dev/null 2>&1
    echo "feature content" > conflict.txt
    git add conflict.txt && git_commit -m "feature conflict"

    run_gx sync --merge

    assert_exit_code "exits non-zero" 1
    assert_output_contains "conflict via merge flag" "conflicts"
    assert_output_contains "conflicted files" "Conflicted files"
    assert_output_not_contains "no rebase message" "Rebase"
    assert_merge_in_progress "merge left in progress"

    cd "$TEST_DIR" && git merge --abort >/dev/null 2>&1 || true
}

# ======================================================================
# TEST 26 — no config file, no args → "No configuration found."
# ======================================================================
test_26_no_config_file() {
    echo ""
    echo "=== Test 26: no config file, no args ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    rm -rf "$TEST_DIR/.gx"

    run_gx sync

    assert_exit_code "exits non-zero" 1
    assert_output_contains "no config error" "No existing"
    assert_output_contains "hint shows gx init" "gx init"
    assert_branch "still on feature/test" "feature/test"
}

# ======================================================================
# TEST 27 — custom remote name via config
# ======================================================================
test_27_custom_remote_config() {
    echo ""
    echo "=== Test 27: custom remote via config ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    # Add a second remote pointing to the same origin
    git remote add upstream "$ORIGIN_DIR"

    run_gx config remote upstream
    run_gx sync

    assert_exit_code "exits 0" 0
    assert_output_not_contains "no errors" "✗"
    assert_branch "still on feature/test" "feature/test"
    assert_clean "working tree clean"
    assert_no_rebase_in_progress "no rebase lingering"
    assert_merge_base_equals "develop is ancestor of feature" "develop" "feature/test"
    assert_git_valid "repository intact"
}

# ======================================================================
# TEST 28 — valid custom defaultBranch via config
# ======================================================================
test_28_custom_defaultBranch_valid() {
    echo ""
    echo "=== Test 28: custom defaultBranch (valid) ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    # Create a main branch on origin at the same position as origin/develop
    git fetch origin develop 2>/dev/null
    git push origin origin/develop:refs/heads/main 2>/dev/null

    # Create local main tracking origin/main
    git branch main origin/main 2>/dev/null

    run_gx config defaultBranch main
    run_gx sync

    assert_exit_code "exits 0" 0
    assert_branch "still on feature/test" "feature/test"
    assert_clean "working tree clean"
    assert_no_rebase_in_progress "no rebase lingering"
    assert_merge_base_equals "main is ancestor of feature" "main" "feature/test"
    assert_git_valid "repository intact"
}

# ======================================================================
# TEST 29 — invalid custom defaultBranch via config → checkout error
# ======================================================================
test_29_custom_defaultBranch_invalid() {
    echo ""
    echo "=== Test 29: custom defaultBranch (invalid) ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"

    run_gx config defaultBranch nonexistent
    run_gx sync

    assert_exit_code "exits non-zero" 1
    assert_output_contains "checkout error" "checkout"
    assert_output_contains "shows branch name" "nonexistent"
    assert_branch "back on feature/test" "feature/test"
}

# ======================================================================
# TEST 30 — syncStrategy set via config (merge)
# ======================================================================
test_30_config_strategy_merge() {
    echo ""
    echo "=== Test 30: syncStrategy via config (merge) ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"

    run_gx config syncStrategy merge
    run_gx sync

    assert_exit_code "exits 0" 0
    assert_output_not_contains "no rebasing" "Rebasing"
    assert_branch "still on feature/test" "feature/test"
    assert_clean "working tree clean"
    assert_no_merge_in_progress "no merge lingering"
    local merge_count
    merge_count=$(cd "$TEST_DIR" && git rev-list --count --merges HEAD 2>/dev/null)
    [ "$merge_count" -ge 1 ] && pass "merge commit created from config" || fail "expected merge commit, got none"
    assert_git_valid "repository intact"
}

# ======================================================================
# TEST 31 — config remote name that doesn't exist as a git remote
# ======================================================================
test_31_config_remote_bogus() {
    echo ""
    echo "=== Test 31: config remote bogus (remote name doesn't exist) ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"

    run_gx config remote bogus
    run_gx sync

    assert_exit_code "exits non-zero" 1
    assert_output_contains "remote not found" "bogus"
    assert_output_contains "hint" "git remote -v"
    assert_branch "still on feature/test" "feature/test"
}

test_32_sync_positional_args() {
    echo ""
    echo "=== Test 32: sync positional args (remote + branch) ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"

    git_checkout feature/test
    old_head=$(git_head)

    run_gx sync origin develop

    assert_exit_code "exits 0" 0
    assert_branch "still on feature/test" "feature/test"
    assert_clean "working tree clean"
    assert_no_rebase_in_progress "no rebase lingering"
    assert_merge_base_equals "develop is ancestor" "develop" "feature/test"
    assert_head_changed "HEAD changed" "$old_head"
    assert_git_valid "repository intact"
}

test_33_sync_positional_remote_only() {
    echo ""
    echo "=== Test 33: sync positional remote only ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"

    git_checkout feature/test

    run_gx sync origin

    assert_exit_code "exits 0" 0
    assert_branch "still on feature/test" "feature/test"
    assert_clean "working tree clean"
    assert_no_rebase_in_progress "no rebase lingering"
    assert_git_valid "repository intact"
}

test_34_sync_verbose() {
    echo ""
    echo "=== Test 34: sync with --verbose flag ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"

    git_checkout feature/test

    run_gx sync --verbose

    assert_exit_code "exits 0" 0
    assert_branch "still on feature/test" "feature/test"
    assert_clean "working tree clean"
    assert_no_rebase_in_progress "no rebase lingering"
    assert_git_valid "repository intact"
}

# ======================================================================
# TEST 35 — abort rebase in progress (Q3, no stash)
# ======================================================================
test_35_abort_rebase_no_stash() {
    echo ""
    echo "=== Test 35: abort rebase in progress (Q3, no stash) ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git_checkout feature/test

    # Create a conflict: develop and feature both modify conflict.txt
    git_checkout develop
    git_pull --ff-only origin develop
    echo "develop content" > conflict.txt
    git add conflict.txt && git_commit -m "develop conflict for abort"
    git_push origin develop

    git_checkout feature/test
    git_fetch origin feature/test
    git rebase origin/feature/test >/dev/null 2>&1
    echo "feature content" > conflict.txt
    git add conflict.txt && git_commit -m "feature conflict for abort"

    # Start rebase manually — will conflict, leave rebase in progress
    git rebase develop >/dev/null 2>&1 || true

    run_gx sync --abort

    assert_exit_code "exits non-zero" 1
    assert_output_contains "gx only manages" "gx sync only manages"

    # Cleanup the manual rebase
    cd "$TEST_DIR" && git rebase --abort >/dev/null 2>&1 || true
}

# ======================================================================
# TEST 36 — abort rebase with stash (Q4)
# ======================================================================
test_36_abort_rebase_with_stash() {
    echo ""
    echo "=== Test 36: abort rebase with stash (Q4) ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git_checkout feature/test

    # Create a conflict: develop and feature both modify conflict.txt
    git_checkout develop
    git_pull --ff-only origin develop
    echo "develop content" > conflict.txt
    git add conflict.txt && git_commit -m "develop conflict for abort"
    git_push origin develop

    git_checkout feature/test
    git_fetch origin feature/test
    git rebase origin/feature/test >/dev/null 2>&1
    echo "feature content" > conflict.txt
    git add conflict.txt && git_commit -m "feature conflict for abort"

    # Add a dirty change so gx creates a stash entry
    echo "dirty" >> readme.md

    # First run — should hit conflict and leave rebase + stash
    run_gx sync

    assert_exit_code "first run exits non-zero" 1
    assert_rebase_in_progress "rebase left in progress after first run"

    # Second run with --abort
    run_gx sync --abort

    assert_exit_code "exits 0" 0
    assert_output_contains "abort rebase message" "Aborting rebase"
    assert_output_contains "restoring stash" "Restoring"
    assert_no_rebase_in_progress "rebase aborted"
    assert_branch "back on feature/test" "feature/test"
    assert_dirty "dirty change restored after abort"
    assert_git_valid "repository intact"

    # Cleanup in case abort failed
    cd "$TEST_DIR" && git rebase --abort >/dev/null 2>&1 || true
}

# ======================================================================
# TEST 37 — abort merge in progress (Q5, no stash)
# ======================================================================
test_37_abort_merge_no_stash() {
    echo ""
    echo "=== Test 37: abort merge in progress (Q5, no stash) ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git_checkout feature/test
    write_config "develop" "origin" "merge"

    # Create a conflict: develop and feature both modify conflict.txt
    git_checkout develop
    git_pull --ff-only origin develop
    echo "develop merge content" > conflict.txt
    git add conflict.txt && git_commit -m "develop merge conflict for abort"
    git_push origin develop

    git_checkout feature/test
    git_fetch origin feature/test
    git rebase origin/feature/test >/dev/null 2>&1
    echo "feature merge content" > conflict.txt
    git add conflict.txt && git_commit -m "feature merge conflict for abort"

    # Start merge manually — will conflict, leave merge in progress
    git merge develop >/dev/null 2>&1 || true

    run_gx sync --abort

    assert_exit_code "exits non-zero" 1
    assert_output_contains "gx only manages" "gx sync only manages"

    # Cleanup the manual merge
    cd "$TEST_DIR" && git merge --abort >/dev/null 2>&1 || true
}

# ======================================================================
# TEST 38 — abort merge with stash (Q6)
# ======================================================================
test_38_abort_merge_with_stash() {
    echo ""
    echo "=== Test 38: abort merge with stash (Q6) ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git_checkout feature/test
    write_config "develop" "origin" "merge"

    # Create a conflict: develop and feature both modify conflict.txt
    git_checkout develop
    git_pull --ff-only origin develop
    echo "develop merge content" > conflict.txt
    git add conflict.txt && git_commit -m "develop merge conflict for abort"
    git_push origin develop

    git_checkout feature/test
    git_fetch origin feature/test
    git rebase origin/feature/test >/dev/null 2>&1
    echo "feature merge content" > conflict.txt
    git add conflict.txt && git_commit -m "feature merge conflict for abort"

    # Add a dirty change so gx creates a stash entry
    echo "dirty" >> readme.md

    # First run — should hit merge conflict and leave merge + stash
    run_gx sync

    assert_exit_code "first run exits non-zero" 1
    assert_merge_in_progress "merge left in progress after first run"

    # Second run with --abort
    run_gx sync --abort

    assert_exit_code "exits 0" 0
    assert_output_contains "abort merge message" "Aborting merge"
    assert_output_contains "restoring stash" "Restoring"
    assert_no_merge_in_progress "merge aborted"
    assert_branch "back on feature/test" "feature/test"
    assert_dirty "dirty change restored after abort"
    assert_git_valid "repository intact"

    # Cleanup in case abort failed
    cd "$TEST_DIR" && git merge --abort >/dev/null 2>&1 || true
}

# ======================================================================
# TEST 39 — nothing to abort (Q1)
# ======================================================================
test_39_nothing_to_abort() {
    echo ""
    echo "=== Test 39: nothing to abort (Q1) ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git_checkout feature/test

    run_gx sync --abort

    assert_exit_code "exits non-zero" 1
    assert_output_contains "nothing to abort message" "Nothing to abort"
    assert_branch "still on feature/test" "feature/test"
    assert_clean "working tree clean"
    assert_no_rebase_in_progress "no rebase"
    assert_no_merge_in_progress "no merge"
}

# ======================================================================
# TEST 40 — orphaned stash nothing to abort (Q2)
# ======================================================================
test_40_orphaned_stash_abort() {
    echo ""
    echo "=== Test 40: orphaned stash (Q2) nothing to abort ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git_checkout feature/test

    # Create an orphaned gx stash entry with no rebase/merge in progress
    echo "orphaned" >> readme.md
    git stash push -m "gx-sync/feature/test/99999" >/dev/null 2>&1

    run_gx sync --abort

    assert_exit_code "exits non-zero" 1
    assert_output_contains "nothing to abort message" "Nothing to abort"
    assert_branch "still on feature/test" "feature/test"
    assert_clean "working tree clean (stash preserved)"
    assert_no_rebase_in_progress "no rebase"
    assert_no_merge_in_progress "no merge"
}

# ======================================================================
# TEST 41 — mutual exclusion (--abort + --continue)
# ======================================================================
test_41_abort_and_continue_exclusive() {
    echo ""
    echo "=== Test 41: mutual exclusion (--abort + --continue) ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    cd "$TEST_DIR"
    git_checkout feature/test

    run_gx sync --abort --continue

    assert_exit_code "exits non-zero" 1
    assert_output_contains "mutual exclusion error" "Cannot specify both"
    assert_output_contains "mentions abort" "abort"
    assert_branch "still on feature/test" "feature/test"
    assert_clean "working tree clean"
}

# ======================================================================
# RUN ALL
# ======================================================================
cleanup_tempdir
build_gx
trap cleanup_tempdir EXIT

echo "============================================"
echo "  gx sync — test suite"
echo "============================================"
echo ""

run_test test_01_success_feature "Test 01: success — feature branch"
run_test test_02_outside_repo "Test 02: outside repo"
run_test test_03_base_branch "Test 03: base branch (pull only)"
run_test test_04_remote_offline "Test 04: remote offline"
run_test test_05_merge_conflict "Test 05: merge conflict (rebase left in progress)"
run_test test_06_branch_missing "Test 06: branch missing"
run_test test_07_dirty_working_tree "Test 07: dirty working tree"
run_test test_08_detached_head "Test 08: detached HEAD"
run_test test_09_base_branch_deleted "Test 09: base branch deleted"
run_test test_10_merge_strategy "Test 10: merge strategy"
run_test test_11_rebase_in_progress "Test 11: rebase already in progress"
run_test test_12_auto_continue "Test 12: auto-continue after conflict"
run_test test_13_auto_continue_still_conflicted "Test 13: auto-continue still conflicted"
run_test test_14_orphaned_stash "Test 14: orphaned gx stash"
run_test test_15_merge_conflict_auto_continue "Test 15: merge conflict auto-continue"
run_test test_16_merge_conflict_continue_flag "Test 16: merge conflict --continue flag"
run_test test_17_merge_conflict_still_conflicted "Test 17: merge conflict still unresolved"
run_test test_18_continue_nothing "Test 18: --continue on clean state"
run_test test_19_manual_rebase_continue "Test 19: manual rebase --continue"
run_test test_20_manual_merge_continue "Test 20: manual merge --continue"
run_test test_21_flag_rebase "Test 21: --rebase flag override"
run_test test_22_flag_merge "Test 22: --merge flag override"
run_test test_23_flag_both_exclusive "Test 23: --rebase + --merge exclusive"
run_test test_24_flag_rebase_conflict "Test 24: --rebase with conflict"
run_test test_25_flag_merge_conflict "Test 25: --merge with conflict"
run_test test_26_no_config_file "Test 26: no config file"
run_test test_27_custom_remote_config "Test 27: custom remote via config"
run_test test_28_custom_defaultBranch_valid "Test 28: custom defaultBranch valid"
run_test test_29_custom_defaultBranch_invalid "Test 29: custom defaultBranch invalid"
run_test test_30_config_strategy_merge "Test 30: syncStrategy via config (merge)"
run_test test_31_config_remote_bogus "Test 31: config remote bogus"
run_test test_32_sync_positional_args "Test 32: sync positional args"
run_test test_33_sync_positional_remote_only "Test 33: sync positional remote only"
run_test test_34_sync_verbose "Test 34: sync verbose"
run_test test_35_abort_rebase_no_stash "Test 35: abort rebase in progress (Q3)"
run_test test_36_abort_rebase_with_stash "Test 36: abort rebase with stash (Q4)"
run_test test_37_abort_merge_no_stash "Test 37: abort merge in progress (Q5)"
run_test test_38_abort_merge_with_stash "Test 38: abort merge with stash (Q6)"
run_test test_39_nothing_to_abort "Test 39: nothing to abort (Q1)"
run_test test_40_orphaned_stash_abort "Test 40: orphaned stash nothing to abort (Q2)"
run_test test_41_abort_and_continue_exclusive "Test 41: --abort + --continue exclusion"

echo ""
print_test_summary

cleanup_tempdir

[ "$TEST_FAIL" -eq 0 ]
