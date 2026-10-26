#!/bin/bash
# gx ship — comprehensive test suite
# Usage: bash tests/gx-ship-test.sh
# Re-runnable: cleans up and recreates everything each run.
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

# ======================================================================
# Common setup — creates origin + test repo with develop and feature/test
# ======================================================================
setup_ship_repo() {
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

    # feature branch with one commit
    git checkout -b feature/test
    echo "feature work" >> readme.md && git add . && git_commit -m "feature work"
    git_push -u origin feature/test

    # second user: advance develop and feature/test on origin
    git clone "$ORIGIN_DIR" "$OTHER_DIR" 2>/dev/null
    cd "$OTHER_DIR"
    configure_git
    git_checkout develop
    echo "# gx project" > Makefile
    git add . && git_commit -m "add Makefile"
    git_push

    git_checkout feature/test
    echo "other work" >> readme.md && git add . && git_commit -m "other feat work"
    git_push

    # return to test repo and write config
    cd "$TEST_DIR"
    write_config "develop" "origin" "rebase"
    echo ".gx/" >> "$TEST_DIR/.git/info/exclude"
}

# ======================================================================
# TEST 01 — first push (no upstream configured)
# ======================================================================
test_01_first_push() {
    cleanup_tempdir
    setup_tempdir
    setup_ship_repo

    cd "$TEST_DIR"
    git_checkout -b feat/new-branch
    echo "new file" > new.txt && git add . && git_commit -m "new branch work"

    run_gx ship

    assert_exit_code "exits 0" 0
    assert_output_contains "push succeeded" "Pushed"
assert_output_contains "host undetected" "Could not detect"
	assert_branch "stays on branch" "feat/new-branch"
	assert_clean "working tree clean"
	assert_git_valid "repo integrity"
}

# ======================================================================
# TEST 02 — subsequent push (upstream already set, fast-forward)
# ======================================================================
test_02_subsequent_push() {
	cleanup_tempdir
	setup_tempdir
	setup_ship_repo

	cd "$TEST_DIR"
	# feature/test has upstream — catch up to origin first, then add a commit
	git_fetch origin feature/test
	git rebase origin/feature/test
	echo "more work" >> readme.md && git add . && git_commit -m "more feature work"

	run_gx ship

	assert_exit_code "exits 0" 0
	assert_output_contains "push succeeded" "Pushed"
	assert_output_contains "host undetected" "Could not detect"
    assert_branch "stays on branch" "feature/test"
    assert_clean "working tree clean"
    assert_git_valid "repo integrity"
}

# ======================================================================
# TEST 03 — protected branch blocked
# ======================================================================
test_03_protected_branch() {
    cleanup_tempdir
    setup_tempdir
    setup_ship_repo

    cd "$TEST_DIR"
    git_checkout develop

    run_gx ship

    assert_exit_code "exits 1" 1
    assert_output_contains "refusal message" "Refusing to push"
    assert_branch "stays on develop" "develop"
}

# ======================================================================
# TEST 04 — detached HEAD blocked
# ======================================================================
test_04_detached_head() {
    cleanup_tempdir
    setup_tempdir
    setup_ship_repo

    cd "$TEST_DIR"
    git checkout --detach HEAD

    run_gx ship

    assert_exit_code "exits 1" 1
    assert_output_contains "detached message" "detached HEAD"
}

# ======================================================================
# TEST 05 — diverged history blocked (no --force)
# ======================================================================
test_05_diverged_blocked() {
    cleanup_tempdir
    setup_tempdir
    setup_ship_repo

    cd "$TEST_DIR"
    # Catch up to origin
    git_fetch origin feature/test
    git rebase origin/feature/test
    # Add local commit
    echo "local divergence" >> readme.md && git add . && git_commit -m "local change"

    # Advance origin from OTHER_DIR
    cd "$OTHER_DIR"
    git_checkout feature/test
    echo "remote divergence" >> remote.txt && git add . && git_commit -m "remote change"
    git_push

    cd "$TEST_DIR"

    run_gx ship

    assert_exit_code "exits 1" 1
    assert_output_contains "diverged message" "diverged"
    assert_output_contains "ahead count" "commit(s) not on remote"
    assert_output_contains "behind count" "commit(s) not on local"
}

# ======================================================================
# TEST 06 — diverged history with --force succeeds
# ======================================================================
test_06_diverged_force() {
    cleanup_tempdir
    setup_tempdir
    setup_ship_repo

    cd "$TEST_DIR"
    # Catch up to origin
    git_fetch origin feature/test
    git rebase origin/feature/test
    # Add local commit
    echo "local force" >> readme.md && git add . && git_commit -m "local change for force"

    # Advance origin from OTHER_DIR
    cd "$OTHER_DIR"
    git_checkout feature/test
    echo "remote force" >> remote.txt && git add . && git_commit -m "remote change for force"
    git_push

    cd "$TEST_DIR"

    run_gx ship --force

    assert_exit_code "exits 0" 0
    assert_output_contains "push succeeded" "Pushed"
    assert_git_valid "repo integrity"
}

# ======================================================================
# TEST 07 — --no-pr suppresses PR URL
# ======================================================================
test_07_no_pr() {
    cleanup_tempdir
    setup_tempdir
    setup_ship_repo

    cd "$TEST_DIR"
    git_fetch origin feature/test
    git rebase origin/feature/test
    echo "no-pr test" >> readme.md && git add . && git_commit -m "no-pr commit"

    run_gx ship --no-pr

    assert_exit_code "exits 0" 0
    assert_output_contains "push succeeded" "Pushed"
    assert_output_not_contains "no PR URL" "Could not detect"
}

# ======================================================================
# TEST 08 — outside git repo blocked
# ======================================================================
test_08_outside_repo() {
    cleanup_tempdir
    setup_tempdir

    GX_RUN_DIR="$TEST_ROOT" run_gx ship

    assert_exit_code "exits 1" 1
    assert_output_contains "not a repo" "Git repository"
}

# ======================================================================
# TEST 09 — no .gx/config blocked
# ======================================================================
test_09_no_config() {
    cleanup_tempdir
    setup_tempdir
    setup_ship_repo

    cd "$TEST_DIR"
    rm -rf "$TEST_DIR/.gx"

    run_gx ship

    assert_exit_code "exits 1" 1
    assert_output_contains "no config" "gx init"
}

# ======================================================================
# TEST 10 — rebase in progress blocked
# ======================================================================
test_10_rebase_in_progress() {
    cleanup_tempdir
    setup_tempdir
    setup_ship_repo

    cd "$TEST_DIR"
    # Create a conflicting rebase: same file, different content on two branches
    git_checkout -b conflict-feature develop
    echo "feature content" > conflict.txt && git add . && git_commit -m "feature adds file"

    git_checkout develop
    echo "develop content" > conflict.txt && git add . && git_commit -m "develop adds file"

    # Rebase — conflict.txt added in both branches with different content → conflict
    git_checkout conflict-feature
    git rebase develop 2>/dev/null || true

    run_gx ship

    assert_exit_code "exits 1" 1
    assert_output_contains "rebase message" "rebase"
}

# ======================================================================
# TEST 11 — merge in progress blocked
# ======================================================================
test_11_merge_in_progress() {
    cleanup_tempdir
    setup_tempdir
    setup_ship_repo

    cd "$TEST_DIR"
    # Create a conflicting merge: same file, different content on two branches
    git_checkout -b merge-feature develop
    echo "feature merge" > merge.txt && git add . && git_commit -m "feature adds file"

    git_checkout develop
    echo "develop merge" > merge.txt && git add . && git_commit -m "develop adds file"

    # Merge — merge.txt added in both branches with different content → conflict
    git_checkout merge-feature
    git merge develop 2>/dev/null || true

    run_gx ship

    assert_exit_code "exits 1" 1
    assert_output_contains "merge message" "merge"
}

# ======================================================================
# TEST 12 — behind-only blocked (local is ancestor of remote)
# ======================================================================
test_12_behind_only() {
    cleanup_tempdir
    setup_tempdir
    setup_ship_repo

    cd "$TEST_DIR"
    # feature/test is behind origin/feature/test (setup pushed ahead from OTHER_DIR)
    # No local commits, remote is ahead

    run_gx ship

    assert_exit_code "exits 1" 1
    assert_output_contains "behind message" "ahead"
}

# ======================================================================
# Run all tests
# ======================================================================
build_gx

run_test test_01_first_push "Test 01: first push — no upstream"
run_test test_02_subsequent_push "Test 02: subsequent push — fast-forward"
run_test test_03_protected_branch "Test 03: protected branch blocked"
run_test test_04_detached_head "Test 04: detached HEAD blocked"
run_test test_05_diverged_blocked "Test 05: diverged history blocked (no --force)"
run_test test_06_diverged_force "Test 06: diverged history with --force succeeds"
run_test test_07_no_pr "Test 07: --no-pr suppresses PR URL"
run_test test_08_outside_repo "Test 08: outside git repo blocked"
run_test test_09_no_config "Test 09: no .gx/config blocked"
run_test test_10_rebase_in_progress "Test 10: rebase in progress blocked"
run_test test_11_merge_in_progress "Test 11: merge in progress blocked"
run_test test_12_behind_only "Test 12: behind-only blocked"

print_test_summary
[ "$TEST_FAIL" -eq 0 ]
