#!/bin/bash
# gx config — comprehensive test suite
# Usage: bash tests/gx-config-test.sh
# Re-runnable: cleans up and recreates everything each run.
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

# ======================================================================
# Helpers
# ======================================================================

setup_config_repo() {
    setup_tempdir
    setup_base_repo
}

# ======================================================================
# TEST 01 — success: set and get remote
# ======================================================================
test_01_set_get_remote() {
    echo ""
    echo "=== Test 01: success — set and get remote ==="
    cleanup_tempdir
    setup_config_repo

    cd "$TEST_DIR"
    run_gx config remote upstream

    assert_exit_code "set remote exits 0" 0

    run_gx config remote

    assert_exit_code "get remote exits 0" 0
    assert_output_contains "get returns upstream" "upstream"
}

# ======================================================================
# TEST 02 — success: set and get defaultBranch
# ======================================================================
test_02_set_get_default_branch() {
    echo ""
    echo "=== Test 02: success — set and get defaultBranch ==="
    cleanup_tempdir
    setup_config_repo

    cd "$TEST_DIR"
    run_gx config defaultBranch main

    assert_exit_code "set defaultBranch exits 0" 0

    run_gx config defaultBranch

    assert_exit_code "get defaultBranch exits 0" 0
    assert_output_contains "get returns main" "main"
}

# ======================================================================
# TEST 03 — success: set and get syncStrategy
# ======================================================================
test_03_set_get_sync_strategy() {
    echo ""
    echo "=== Test 03: success — set and get syncStrategy ==="
    cleanup_tempdir
    setup_config_repo

    cd "$TEST_DIR"
    run_gx config syncStrategy merge

    assert_exit_code "set syncStrategy exits 0" 0

    run_gx config syncStrategy

    assert_exit_code "get syncStrategy exits 0" 0
    assert_output_contains "get returns merge" "merge"
}

# ======================================================================
# TEST 04 — success: append sensitivePatterns
# ======================================================================
test_04_sensitive_append() {
    echo ""
    echo "=== Test 04: success — append sensitivePatterns ==="
    cleanup_tempdir
    setup_config_repo

    cd "$TEST_DIR"

    # Append two patterns
    run_gx config sensitivePatterns .env .gitignore

    assert_exit_code "append patterns exits 0" 0

    # Get the value
    run_gx config sensitivePatterns

    assert_exit_code "get patterns exits 0" 0
    assert_output_contains "contains .env" ".env"
    assert_output_contains "contains .gitignore" ".gitignore"

    # Append another pattern
    run_gx config sensitivePatterns *.tfvars

    assert_exit_code "append more patterns exits 0" 0

    # Get the value — should contain all three
    run_gx config sensitivePatterns

    assert_exit_code "get all patterns exits 0" 0
    assert_output_contains "contains .env" ".env"
    assert_output_contains "contains .gitignore" ".gitignore"
    assert_output_contains "contains .tfvars" "tfvars"
}

# ======================================================================
# TEST 05 — success: overwrite sensitivePatterns
# ======================================================================
test_05_sensitive_overwrite() {
    echo ""
    echo "=== Test 05: success — overwrite sensitivePatterns ==="
    cleanup_tempdir
    setup_config_repo

    cd "$TEST_DIR"

    # Append initial patterns (avoid substring overlap with overwrite patterns)
    run_gx config sensitivePatterns "*.old" "*.bak"
    assert_exit_code "append initial patterns exits 0" 0

    # Overwrite with new patterns
    run_gx config sensitivePatterns .env1 .env2 --overwrite

    assert_exit_code "overwrite patterns exits 0" 0

    # Get the value
    run_gx config sensitivePatterns

    assert_exit_code "get overwritten patterns exits 0" 0
    assert_output_contains "contains .env1" ".env1"
    assert_output_contains "contains .env2" ".env2"
    assert_output_not_contains "no old pattern" "old"
    assert_output_not_contains "no bak pattern" "bak"
}

# ======================================================================
# TEST 06 — block: --overwrite with no patterns
# ======================================================================
test_06_overwrite_no_patterns() {
    echo ""
    echo "=== Test 06: block — --overwrite with no patterns ==="
    cleanup_tempdir
    setup_config_repo

    cd "$TEST_DIR"
    run_gx config sensitivePatterns --overwrite

    assert_exit_code "exits non-zero" 1
    assert_output_contains "error: no patterns specified" "no patterns specified"
}

# ======================================================================
# TEST 07 — success: gx config list shows formatted output
# ======================================================================
test_07_config_list() {
    echo ""
    echo "=== Test 07: success — gx config list ==="
    cleanup_tempdir
    setup_config_repo

    cd "$TEST_DIR"
    run_gx config list

    assert_exit_code "list exits 0" 0
    assert_output_contains "shows remote key" "remote"
    assert_output_contains "shows defaultBranch key" "defaultBranch"
    assert_output_contains "shows syncStrategy key" "syncStrategy"
    assert_output_contains "shows sensitivePatterns key" "sensitivePatterns"
    assert_output_contains "shows origin value" "origin"
    assert_output_contains "shows develop default" "develop"
    assert_output_contains "shows rebase default" "rebase"
}

# ======================================================================
# TEST 08 — success: default values without config file
# ======================================================================
test_08_defaults_without_config() {
    echo ""
    echo "=== Test 08: success — default values without config file ==="
    cleanup_tempdir
    setup_config_repo

    cd "$TEST_DIR"
    rm -f "$TEST_DIR/.gx/config"

    run_gx config remote

    assert_exit_code "get remote exits 0" 0
    assert_output_contains "default remote is origin" "origin"

    run_gx config defaultBranch

    assert_exit_code "get defaultBranch exits 0" 0
    assert_output_contains "default branch is develop" "develop"

    run_gx config syncStrategy

    assert_exit_code "get syncStrategy exits 0" 0
    assert_output_contains "default strategy is rebase" "rebase"

    run_gx config sensitivePatterns

    assert_exit_code "get sensitivePatterns exits 0" 0
    assert_output_contains "empty default is []" "[]"
}

# ======================================================================
# RUN ALL
# ======================================================================
cleanup_tempdir
build_gx
trap cleanup_tempdir EXIT

echo "============================================"
echo "  gx config — test suite"
echo "============================================"
echo ""

run_test test_01_set_get_remote "Test 01: success — set and get remote"
run_test test_02_set_get_default_branch "Test 02: success — set and get defaultBranch"
run_test test_03_set_get_sync_strategy "Test 03: success — set and get syncStrategy"
run_test test_04_sensitive_append "Test 04: success — append sensitivePatterns"
run_test test_05_sensitive_overwrite "Test 05: success — overwrite sensitivePatterns"
run_test test_06_overwrite_no_patterns "Test 06: block — --overwrite with no patterns"
run_test test_07_config_list "Test 07: success — gx config --list"
run_test test_08_defaults_without_config "Test 08: success — default values without config"

echo ""
print_test_summary

cleanup_tempdir

[ "$TEST_FAIL" -eq 0 ]
