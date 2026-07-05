#!/bin/bash
# gx guard — pre-flight guard test suite
# Tests the PersistentPreRunE logic in cmd/root.go:
#   - built-in commands (help, completion) skip all guards
#   - non-repo-aware commands (init, config) fail outside a repo
#   - repo-requiring commands (sync) fail outside a repo
#
# Usage: bash tests/gx-guard-test.sh
# Re-runnable: each test does its own cleanup_tempdir + setup_tempdir.
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

# ======================================================================
# TEST 01 — gx help outside a repository (no_repo + no_config)
# ======================================================================
test_01_help_outside_repo() {
    echo ""
    echo "=== Test 01: gx help outside a repository ==="
    cleanup_tempdir
    setup_tempdir
    mkdir -p "$TEST_DIR"

    GX_RUN_DIR="$TEST_DIR" run_gx help

    assert_exit_code "exits 0" 0
    assert_output_contains "shows usage" "Usage:"
}

# ======================================================================
# TEST 02 — gx completion outside a repository (no_repo + no_config)
# ======================================================================
test_02_completion_outside_repo() {
    echo ""
    echo "=== Test 02: gx completion bash outside a repository ==="
    cleanup_tempdir
    setup_tempdir
    mkdir -p "$TEST_DIR"

    GX_RUN_DIR="$TEST_DIR" run_gx completion bash

    assert_exit_code "exits 0" 0
    assert_output_contains "bash completions" "bash"
}

# ======================================================================
# TEST 03 — gx sync blocked outside a repository
# ======================================================================
test_03_sync_blocked_outside_repo() {
    echo ""
    echo "=== Test 03: gx sync blocked outside a repository ==="
    cleanup_tempdir
    setup_tempdir
    mkdir -p "$TEST_DIR"

    GX_RUN_DIR="$TEST_DIR" run_gx sync

    assert_exit_code "exits 1" 1
    assert_output_contains "repo guard" "Git repository"
    assert_output_contains "hint" "cloned repository"
}

# ======================================================================
# TEST 04 — gx init blocked outside a repository (no_config, but NOT no_repo)
# ======================================================================
test_04_init_blocked_outside_repo() {
    echo ""
    echo "=== Test 04: gx init blocked outside a repository ==="
    cleanup_tempdir
    setup_tempdir
    mkdir -p "$TEST_DIR"

    GX_RUN_DIR="$TEST_DIR" run_gx init

    assert_exit_code "exits 1" 1
    assert_output_contains "repo guard" "Git repository"
}

# ======================================================================
# TEST 05 — gx config blocked outside a repository (no_config, but NOT no_repo)
# ======================================================================
test_05_config_blocked_outside_repo() {
    echo ""
    echo "=== Test 05: gx config blocked outside a repository ==="
    cleanup_tempdir
    setup_tempdir
    mkdir -p "$TEST_DIR"

    GX_RUN_DIR="$TEST_DIR" run_gx config

    assert_exit_code "exits 1" 1
    assert_output_contains "repo guard" "Git repository"
}

# ======================================================================
# TEST 06 — gx help inside a repository (no_repo + no_config)
# ======================================================================
test_06_help_inside_repo() {
    echo ""
    echo "=== Test 06: gx help inside a repository ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    run_gx help

    assert_exit_code "exits 0" 0
    assert_output_contains "shows usage" "Usage:"
}

# ======================================================================
# TEST 07 — gx completion bash inside a repository (no_repo + no_config)
# ======================================================================
test_07_completion_inside_repo() {
    echo ""
    echo "=== Test 07: gx completion bash inside a repository ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    run_gx completion bash

    assert_exit_code "exits 0" 0
    assert_output_contains "bash completions" "bash"
}

# ======================================================================
# RUN ALL
# ======================================================================
cleanup_tempdir
build_gx
trap cleanup_tempdir EXIT

echo "============================================"
echo "  gx guard — test suite"
echo "============================================"
echo ""

run_test test_01_help_outside_repo "Test 01: help outside a repo"
run_test test_02_completion_outside_repo "Test 02: completion outside a repo"
run_test test_03_sync_blocked_outside_repo "Test 03: sync blocked outside repo"
run_test test_04_init_blocked_outside_repo "Test 04: init blocked outside repo"
run_test test_05_config_blocked_outside_repo "Test 05: config blocked outside repo"
run_test test_06_help_inside_repo "Test 06: help inside a repo"
run_test test_07_completion_inside_repo "Test 07: completion inside a repo"

echo ""
print_test_summary

cleanup_tempdir

[ "$TEST_FAIL" -eq 0 ]
