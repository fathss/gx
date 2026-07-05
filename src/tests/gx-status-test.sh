#!/bin/bash
# gx status — comprehensive test suite
# Usage: bash tests/gx-status-test.sh
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

# ======================================================================
# Helpers
# ======================================================================

setup_status_repo() {
    setup_tempdir
    mkdir -p "$ORIGIN_DIR"
    cd "$ORIGIN_DIR"
    git init --bare 2>/dev/null

    mkdir -p "$TEST_DIR"
    git clone "$ORIGIN_DIR" "$TEST_DIR" 2>/dev/null
    cd "$TEST_DIR"
    configure_git
    git checkout -b develop
    echo "initial" > readme.md
    git add . && git_commit -m "init"
    git_push -u origin develop

    git checkout -b feature/test
    echo "feature work" >> readme.md && git add . && git_commit -m "feature work"
    git_push -u origin feature/test

    write_config "develop" "origin" "rebase"
    echo ".gx/" >> "$TEST_DIR/.git/info/exclude"
}

# ======================================================================
# TEST 01 — normal: on branch with staged + unstaged + stashes
# ======================================================================
test_01_normal() {
    echo ""
    echo "=== Test 01: normal — on branch with staged + unstaged + stashes ==="
    cleanup_tempdir
    setup_status_repo

    cd "$TEST_DIR"
    git_checkout feature/test

    # Create a stash first
    echo "stash content" >> readme.md
    git stash push -m "test stash" 2>/dev/null || true

    # Then create staged and unstaged changes
    echo "staged change" > staged.txt
    git add staged.txt >/dev/null 2>&1
    echo "unstaged change" >> readme.md

    run_gx status

    assert_exit_code "exits 0" 0
    assert_output_contains "branch name shown" "On branch feature/test"
    assert_output_contains "staged section" "Staged:"
    assert_output_contains "staged file listed" "staged.txt"
    assert_output_contains "unstaged section shown" "Unstaged:"
    assert_output_contains "stash count shows 1" "Stashes: 1"
}

# ======================================================================
# TEST 02 — clean tree: nothing staged, nothing dirty
# ======================================================================
test_02_clean_tree() {
    echo ""
    echo "=== Test 02: clean tree ==="
    cleanup_tempdir
    setup_status_repo

    cd "$TEST_DIR"
    git_checkout feature/test

    run_gx status

    assert_exit_code "exits 0" 0
    assert_output_contains "staged section" "Staged:"
    assert_output_contains "staged none" "(none)"
    assert_output_not_contains "no Unstaged section" "Unstaged:"
    assert_output_contains "stash none" "Stashes: (none)"
    assert_output_contains "recent commits" "Recent commits:"
}

# ======================================================================
# TEST 03 — detached HEAD
# ======================================================================
test_03_detached_head() {
    echo ""
    echo "=== Test 03: detached HEAD ==="
    cleanup_tempdir
    setup_status_repo

    cd "$TEST_DIR"
    git checkout --detach 2>/dev/null

    run_gx status

    assert_exit_code "exits 0" 0
    assert_output_contains "detached head message" "HEAD detached at"
}

# ======================================================================
# TEST 04 — rebase in progress
# ======================================================================
test_04_rebase_in_progress() {
    echo ""
    echo "=== Test 04: rebase in progress ==="
    cleanup_tempdir
    setup_status_repo

    cd "$TEST_DIR"
    git_checkout feature/test

    # Create a conflict scenario
    git_checkout develop
    echo "develop content" > conflict.txt
    git add . && git_commit -m "develop conflict base"
    git_push origin develop 2>/dev/null || true

    git_checkout feature/test
    echo "feature content" > conflict.txt
    git add . && git_commit -m "feature conflict"

    # Rebase will conflict
    git rebase develop 2>/dev/null || true

    run_gx status

    assert_exit_code "exits 0" 0
    assert_output_contains "rebase indicator" "(rebase in progress)"

    cd "$TEST_DIR" && git rebase --abort 2>/dev/null || true
}

# ======================================================================
# TEST 05 — merge in progress
# ======================================================================
test_05_merge_in_progress() {
    echo ""
    echo "=== Test 05: merge in progress ==="
    cleanup_tempdir
    setup_status_repo

    cd "$TEST_DIR"
    git_checkout feature/test

    # Create a conflict
    git_checkout develop
    echo "develop content" > conflict.txt
    git add . && git_commit -m "develop conflict base"
    git_push origin develop 2>/dev/null || true

    git_checkout feature/test
    echo "feature content" > conflict.txt
    git add . && git_commit -m "feature conflict"

    # Merge will conflict
    git merge develop 2>/dev/null || true

    run_gx status

    assert_exit_code "exits 0" 0
    assert_output_contains "merge indicator" "(merge in progress)"

    cd "$TEST_DIR" && git merge --abort 2>/dev/null || true
}

# ======================================================================
# TEST 06 — no stashes
# ======================================================================
test_06_no_stashes() {
    echo ""
    echo "=== Test 06: no stashes ==="
    cleanup_tempdir
    setup_status_repo

    cd "$TEST_DIR"
    git_checkout feature/test

    run_gx status

    assert_exit_code "exits 0" 0
    assert_output_contains "stash none" "Stashes: (none)"
}

# ======================================================================
# TEST 07 — no upstream
# ======================================================================
test_07_no_upstream() {
    echo ""
    echo "=== Test 07: no upstream ==="
    cleanup_tempdir
    setup_status_repo

    cd "$TEST_DIR"
    git_checkout feature/test
    git branch --unset-upstream 2>/dev/null

    run_gx status

    assert_exit_code "exits 0" 0
    assert_output_contains "no upstream message" "no upstream configured"
}

# ======================================================================
# TEST 08 — staged changes grouped by status type
# ======================================================================
test_08_staged_grouped() {
    echo ""
    echo "=== Test 08: staged changes grouped by status type ==="
    cleanup_tempdir
    setup_status_repo

    cd "$TEST_DIR"
    git_checkout feature/test

    # First commit a file so we can stage a deletion
    mkdir -p subdir
    echo "deletable" > subdir/to-delete.txt
    git add subdir/to-delete.txt && git_commit -m "add deletable"

    # Now stage changes of all three types
    echo "new content" > newfile.txt
    git add newfile.txt >/dev/null 2>&1

    echo "modified content" >> readme.md
    git add readme.md >/dev/null 2>&1

    git rm subdir/to-delete.txt >/dev/null 2>&1

    run_gx status

    assert_exit_code "exits 0" 0
    assert_output_contains "new file label" "new file:"
    assert_output_contains "modified label" "modified:"
    assert_output_contains "deleted label" "deleted:"
}

# ======================================================================
# TEST 09 — untracked files shown
# ======================================================================
test_09_untracked_files() {
    echo ""
    echo "=== Test 09: untracked files shown ==="
    cleanup_tempdir
    setup_status_repo

    cd "$TEST_DIR"
    git_checkout feature/test

    # Create an untracked file (no git add)
    echo "new untracked" > notes.md

    run_gx status

    assert_exit_code "exits 0" 0
    assert_output_contains "untracked section" "Untracked:"
    assert_output_contains "untracked file" "notes.md"
}

# ======================================================================
# TEST 10 — empty repo (no commits)
# ======================================================================
test_10_empty_repo() {
    echo ""
    echo "=== Test 10: empty repo (no commits) ==="
    cleanup_tempdir
    setup_tempdir
    mkdir -p "$TEST_DIR"
    cd "$TEST_DIR"
    git init 2>/dev/null
    configure_git
    write_config "develop" "origin" "rebase"

    run_gx status

    assert_exit_code "exits 0" 0
    assert_output_contains "no commits message" "no commits yet"
}

# ======================================================================
# TEST 11 — fewer than 5 commits (no padding)
# ======================================================================
test_11_fewer_commits() {
    echo ""
    echo "=== Test 11: fewer than 5 commits ==="
    cleanup_tempdir
    setup_status_repo

    cd "$TEST_DIR"

    run_gx status

    assert_exit_code "exits 0" 0
    assert_output_contains "recent commits section" "Recent commits:"
    assert_output_contains "init commit shown" "init"
    assert_output_contains "feature work shown" "feature work"
    assert_output_not_contains "no padding" "no commits yet"
}

# ======================================================================
# TEST 12 — conflicted files (mid-conflict)
# ======================================================================
test_12_conflicted() {
    echo ""
    echo "=== Test 12: conflicted files ==="
    cleanup_tempdir
    setup_status_repo

    cd "$TEST_DIR"
    git_checkout feature/test

    # Create a conflicted merge
    git_checkout develop
    echo "base conflict" > merge-conflict.txt
    git add . && git_commit -m "base conflict on develop"
    git_push origin develop 2>/dev/null || true

    git_checkout feature/test
    echo "feature conflict" > merge-conflict.txt
    git add . && git_commit -m "feature conflict change"

    # Merge with conflict
    git merge develop 2>/dev/null || true

    run_gx status

    assert_exit_code "exits 0" 0
    assert_output_contains "conflicted section" "Conflicted:"
    assert_output_contains "both modified" "both modified:"

    cd "$TEST_DIR" && git merge --abort 2>/dev/null || true
}

# ======================================================================
# TEST 13 — up to date with upstream
# ======================================================================
test_13_up_to_date() {
    echo ""
    echo "=== Test 13: up to date with upstream ==="
    cleanup_tempdir
    setup_status_repo

    cd "$TEST_DIR"
    git_checkout develop
    # Fetch and fast-forward to be up to date
    git fetch origin 2>/dev/null
    git merge --ff-only origin/develop 2>/dev/null || true

    run_gx status

    assert_exit_code "exits 0" 0
    assert_output_contains "up to date" "up to date with"
}

# ======================================================================
# TEST 14 — ahead only
# ======================================================================
test_14_ahead_only() {
    echo ""
    echo "=== Test 14: ahead only ==="
    cleanup_tempdir
    setup_status_repo

    cd "$TEST_DIR"
    git_checkout feature/test

    # Make a local commit ahead of remote
    echo "ahead change" >> readme.md
    git add . && git_commit -m "ahead commit"

    run_gx status

    assert_exit_code "exits 0" 0
    assert_output_contains "shows ahead" "ahead"
    assert_output_not_contains "no behind count" "behind "
}

# ======================================================================
# TEST 15 — behind only
# ======================================================================
test_15_behind_only() {
    echo ""
    echo "=== Test 15: behind only ==="
    cleanup_tempdir
    setup_status_repo

    cd "$TEST_DIR"
    git_checkout feature/test

    # Need remote-tracking ref to be ahead of local
    # Set up: OTHER_DIR pushes extra commits, local fetches them
    # Create another dir that pushes ahead
    OTHER_REPO="$TEST_ROOT/other_repo"
    git clone "$ORIGIN_DIR" "$OTHER_REPO" 2>/dev/null
    cd "$OTHER_REPO"
    configure_git
    git_checkout feature/test
    echo "remote ahead work" >> readme.md
    git add . && git_commit -m "remote ahead commit"
    git_push

    # Now fetch in test dir — local will be behind
    cd "$TEST_DIR"
    git fetch origin 2>/dev/null

    run_gx status

    assert_exit_code "exits 0" 0
    assert_output_contains "shows behind" "behind "
    assert_output_not_contains "no ahead" "ahead"
}

# ======================================================================
# TEST 16 — --verbose flag shows plumbing
# ======================================================================
test_16_verbose() {
    echo ""
    echo "=== Test 16: --verbose shows plumbing ==="
    cleanup_tempdir
    setup_status_repo

    cd "$TEST_DIR"
    git_checkout feature/test

    run_gx status --verbose

    assert_exit_code "exits 0" 0
    assert_output_contains "verbose shows git diff" "▸ git diff"
}

# ======================================================================
# TEST 17 — without --verbose, no plumbing shown
# ======================================================================
test_17_no_verbose() {
    echo ""
    echo "=== Test 17: without --verbose, no plumbing ==="
    cleanup_tempdir
    setup_status_repo

    cd "$TEST_DIR"
    git_checkout feature/test

    run_gx status

    assert_exit_code "exits 0" 0
    assert_output_not_contains "no ▸ shown" "▸"
}

# ======================================================================
# TEST 18 — outside repo
# ======================================================================
test_18_outside_repo() {
    echo ""
    echo "=== Test 18: outside repo ==="
    cleanup_tempdir
    setup_status_repo

    GX_RUN_DIR=/tmp run_gx status

    assert_exit_code "exits non-zero" 1
    assert_output_contains "not inside git" "Git repository"
}

# ======================================================================
# TEST 19 — no config
# ======================================================================
test_19_no_config() {
    echo ""
    echo "=== Test 19: no config ==="
    cleanup_tempdir
    setup_status_repo

    cd "$TEST_DIR"
    rm -rf "$TEST_DIR/.gx"

    run_gx status

    assert_exit_code "exits non-zero" 1
    assert_output_contains "no config message" "No existing"
    assert_output_contains "hint shows gx init" "gx init"
}

# ======================================================================
# RUN ALL
# ======================================================================
cleanup_tempdir
build_gx
trap cleanup_tempdir EXIT

echo "============================================"
echo "  gx status — test suite"
echo "============================================"
echo ""

run_test test_01_normal "Test 01: normal — staged + unstaged + stashes"
run_test test_02_clean_tree "Test 02: clean tree"
run_test test_03_detached_head "Test 03: detached HEAD"
run_test test_04_rebase_in_progress "Test 04: rebase in progress"
run_test test_05_merge_in_progress "Test 05: merge in progress"
run_test test_06_no_stashes "Test 06: no stashes"
run_test test_07_no_upstream "Test 07: no upstream"
run_test test_08_staged_grouped "Test 08: staged changes grouped by status"
run_test test_09_untracked_files "Test 09: untracked files"
run_test test_10_empty_repo "Test 10: empty repo (no commits)"
run_test test_11_fewer_commits "Test 11: fewer than 5 commits"
run_test test_12_conflicted "Test 12: conflicted files"
run_test test_13_up_to_date "Test 13: up to date with upstream"
run_test test_14_ahead_only "Test 14: ahead only"
run_test test_15_behind_only "Test 15: behind only"
run_test test_16_verbose "Test 16: --verbose shows plumbing"
run_test test_17_no_verbose "Test 17: no --verbose, no plumbing"
run_test test_18_outside_repo "Test 18: outside repo"
run_test test_19_no_config "Test 19: no config"

echo ""
print_test_summary

cleanup_tempdir

[ "$TEST_FAIL" -eq 0 ]
