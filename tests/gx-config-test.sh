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
    assert_output_contains "missing config seeds default patterns" "*.pem"
}

# ======================================================================
# TEST 09 — success: read and write config from child folder
# ======================================================================
test_09_child_folder_get_and_set() {
    echo ""
    echo "=== Test 09: success — read and write config from child folder ==="
    cleanup_tempdir
    setup_config_repo

    # Set root config
    cd "$TEST_DIR"
    run_gx config remote origin-root

    # Create nested child folders
    mkdir -p "$TEST_DIR/sub1/sub2/sub3"

    # Read config from deep child folder
    GX_RUN_DIR="$TEST_DIR/sub1/sub2/sub3" run_gx config remote

    assert_exit_code "get remote from child exits 0" 0
    assert_output_contains "reads root remote" "origin-root"

    # Write config from child folder
    GX_RUN_DIR="$TEST_DIR/sub1/sub2/sub3" run_gx config remote updated-from-child

    assert_exit_code "set remote from child exits 0" 0

    # Verify root config was updated
    run_gx config remote
    assert_output_contains "root config updated" "updated-from-child"

    # Verify no .gx was created in child folders
    if [ -d "$TEST_DIR/sub1/.gx" ] || [ -d "$TEST_DIR/sub1/sub2/.gx" ] || [ -d "$TEST_DIR/sub1/sub2/sub3/.gx" ]; then
        echo "FAIL: .gx directory was created inside a child folder"
        TEST_FAIL=$((TEST_FAIL + 1))
    fi
}

# ======================================================================
# TEST 10 — success: gx init from child folder targets repo root
# ======================================================================
test_10_child_folder_init() {
    echo ""
    echo "=== Test 10: success — gx init from child folder targets repo root ==="
    cleanup_tempdir
    setup_config_repo

    cd "$TEST_DIR"
    rm -rf "$TEST_DIR/.gx"

    mkdir -p "$TEST_DIR/pkg/api"

    # Run init from child folder
    GX_RUN_DIR="$TEST_DIR/pkg/api" run_gx init origin develop

    assert_exit_code "init from child exits 0" 0

    # Verify .gx/config exists at repo root
    if [ ! -f "$TEST_DIR/.gx/config" ]; then
        echo "FAIL: .gx/config was not created at repo root"
        TEST_FAIL=$((TEST_FAIL + 1))
    fi

    # Verify no .gx was created in pkg/ or pkg/api/
    if [ -d "$TEST_DIR/pkg/.gx" ] || [ -d "$TEST_DIR/pkg/api/.gx" ]; then
        echo "FAIL: .gx directory was created inside a child folder"
        TEST_FAIL=$((TEST_FAIL + 1))
    fi

    # Reading config from child folder finds root config
    GX_RUN_DIR="$TEST_DIR/pkg/api" run_gx config remote
    assert_output_contains "child reads remote" "origin"
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
run_test test_09_child_folder_get_and_set "Test 09: success — child folder get and set"
run_test test_10_child_folder_init "Test 10: success — child folder init"

echo ""
print_test_summary

cleanup_tempdir

[ "$TEST_FAIL" -eq 0 ]
