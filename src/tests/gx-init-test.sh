#!/bin/bash
# gx init — comprehensive test suite
# Usage: bash tests/gx-init-test.sh
# Re-runnable: cleans up and recreates everything each run.
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

# ======================================================================
# HELPERS
# ======================================================================

# assert_config_key checks that the config file has a specific JSON key-value.
assert_config_key() {
    local label="$1" key="$2" expected="$3"
    local actual
    actual=$(grep -o "\"$key\": \"[^\"]*\"" "$CONFIG_FILE" | head -1 | grep -o '"[^"]*"$' | tr -d '"')
    if [ "$actual" = "$expected" ]; then
        pass "$label — $key=$expected"
    else
        fail "$label — expected $key='$expected', got '$actual'"
    fi
}

# ======================================================================
# TEST 01 — gx init on a fresh repo (auto-detect)
# ======================================================================
test_01_init_fresh_repo() {
    echo ""
    echo "=== Test 01: gx init on a fresh repo — auto-detect ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo
    rm -rf "$TEST_DIR/.gx"

    run_gx init

    assert_exit_code "exits 0" 0
    assert_output_contains "shows wrote config" ".gx/config"
    assert_output_contains "shows remote" "remote:            origin"
    assert_output_contains "shows defaultBranch" "defaultBranch:     develop"
    assert_output_contains "shows syncStrategy" "syncStrategy:      rebase"
}

# ======================================================================
# TEST 02 — gx init when config already exists
# ======================================================================
test_02_init_already_exists() {
    echo ""
    echo "=== Test 02: gx init when config already exists ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo

    run_gx init

    assert_exit_code "exits non-zero" 1
    assert_output_contains "already exists error" "already exists"
}

# ======================================================================
# TEST 03 — gx init with remote override (positional)
# ======================================================================
test_03_init_remote_override() {
    echo ""
    echo "=== Test 03: gx init upstream (remote override) ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo
    rm -rf "$TEST_DIR/.gx"

    run_gx init upstream

    assert_exit_code "exits 0" 0
    assert_output_contains "shows override remote" "remote:            upstream"
    assert_output_contains "shows defaultBranch" "defaultBranch:     develop"
}

# ======================================================================
# TEST 04 — gx init with both overrides (positional)
# ======================================================================
test_04_init_both_overrides() {
    echo ""
    echo "=== Test 04: gx init upstream main (both overrides) ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo
    rm -rf "$TEST_DIR/.gx"

    run_gx init upstream main

    assert_exit_code "exits 0" 0
    assert_output_contains "shows override remote" "remote:            upstream"
    assert_output_contains "shows override branch" "defaultBranch:     main"
}

# ======================================================================
# TEST 05 — gx init outside a git repository
# ======================================================================
test_05_init_outside_repo() {
    echo ""
    echo "=== Test 05: gx init outside a git repository ==="
    cleanup_tempdir
    setup_tempdir
    mkdir -p "$TEST_DIR/empty"

    GX_RUN_DIR="$TEST_DIR/empty" run_gx init

    assert_exit_code "exits non-zero" 1
    assert_output_contains "not a repo error" "Git repository"
}

# ======================================================================
# TEST 06 — gx init with flag overrides
# ======================================================================
test_06_init_flag_overrides() {
    echo ""
    echo "=== Test 06: gx init --remote upstream --branch main ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo
    rm -rf "$TEST_DIR/.gx"

    run_gx init --remote upstream --branch main

    assert_exit_code "exits 0" 0
    assert_output_contains "shows override remote" "remote:            upstream"
    assert_output_contains "shows override branch" "defaultBranch:     main"
}

# ======================================================================
# TEST 07 — Verify .gx/config file content after init
# ======================================================================
test_07_init_file_content() {
    echo ""
    echo "=== Test 07: Verify .gx/config content after init ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo
    rm -rf "$TEST_DIR/.gx"

    run_gx init

    assert_exit_code "exits 0" 0
    assert_config_key "file has remote" "remote" "origin"
    assert_config_key "file has defaultBranch" "defaultBranch" "develop"
    assert_config_key "file has syncStrategy" "syncStrategy" "rebase"
}

# ======================================================================
# TEST 08 — gx init --branch flag overrides auto-detection
# ======================================================================
test_08_init_branch_flag() {
    echo ""
    echo "=== Test 08: gx init --branch main (flag only) ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo
    rm -rf "$TEST_DIR/.gx"

    run_gx init --branch main

    assert_exit_code "exits 0" 0
    assert_output_contains "remote auto-detected" "remote:            origin"
    assert_output_contains "branch from flag" "defaultBranch:     main"
}

# ======================================================================
# TEST 09 — gx init with too many positional args
# ======================================================================
test_09_init_too_many_args() {
    echo ""
    echo "=== Test 09: gx init remote branch extra (too many args) ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo
    rm -rf "$TEST_DIR/.gx"

    run_gx init remote branch extra

    assert_exit_code "exits non-zero" 1
}

# ======================================================================
# TEST 10 — gx init in a repo with no remotes
# ======================================================================
test_10_init_no_remotes() {
    echo ""
    echo "=== Test 10: gx init in a repo with no remotes ==="
    cleanup_tempdir
    setup_tempdir
    mkdir -p "$TEST_DIR"
    cd "$TEST_DIR" && git init >/dev/null 2>&1
    configure_git
    git_commit --allow-empty -m "init"

    run_gx init

    assert_exit_code "detection failure" 1
    assert_output_contains "failed remote" "detect remote"
    assert_output_contains "hint" "explicitly"
}

# ======================================================================
# TEST 11 — gx init when .gx directory is read-only
# ======================================================================
test_11_init_readonly_dir() {
    echo ""
    echo "=== Test 11: gx init with read-only .gx directory ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo
    rm -rf "$TEST_DIR/.gx"
    mkdir -p "$TEST_DIR/.gx" && chmod 0 "$TEST_DIR/.gx"

    run_gx init

    assert_exit_code "write error" 1
    assert_output_contains "failed write" "write configuration"
    assert_output_contains "hint" "permissions"

    chmod 755 "$TEST_DIR/.gx" 2>/dev/null || true
    rm -rf "$TEST_DIR/.gx" 2>/dev/null || true
}

# ======================================================================
# TEST 12 — gx init with --remote flag only
# ======================================================================
test_12_init_remote_flag() {
    echo ""
    echo "=== Test 12: gx init --remote upstream ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo
    rm -rf "$TEST_DIR/.gx"

    run_gx init --remote upstream

    assert_exit_code "exits 0" 0
    assert_output_contains "shows override remote" "remote:            upstream"
    assert_output_contains "shows defaultBranch" "defaultBranch:     develop"
}

# ======================================================================
# TEST 13 — gx init with --branch flag pointing to nonexistent branch
# ======================================================================
test_13_init_branch_nonexistent() {
    echo ""
    echo "=== Test 13: gx init --branch nonexistent ==="
    cleanup_tempdir
    setup_tempdir
    setup_base_repo
    rm -rf "$TEST_DIR/.gx"

    run_gx init --branch nonexistent

    assert_exit_code "exits 0" 0
    assert_output_contains "remote auto-detected" "remote:            origin"
    assert_output_contains "custom branch" "defaultBranch:     nonexistent"
}

# ======================================================================
# RUN ALL
# ======================================================================
cleanup_tempdir
build_gx
trap cleanup_tempdir EXIT

echo "============================================"
echo "  gx init — test suite"
echo "============================================"
echo ""

run_test test_01_init_fresh_repo "Test 01: init fresh repo — auto-detect"
run_test test_02_init_already_exists "Test 02: init when config already exists"
run_test test_03_init_remote_override "Test 03: init with remote override"
run_test test_04_init_both_overrides "Test 04: init with both overrides"
run_test test_05_init_outside_repo "Test 05: init outside a repo"
run_test test_06_init_flag_overrides "Test 06: init with flag overrides"
run_test test_07_init_file_content "Test 07: verify config file content"
run_test test_08_init_branch_flag "Test 08: init --branch flag"
run_test test_09_init_too_many_args "Test 09: init with too many args"
run_test test_10_init_no_remotes "Test 10: init with no remotes"
run_test test_11_init_readonly_dir "Test 11: init with read-only .gx dir"
run_test test_12_init_remote_flag "Test 12: init --remote flag"
run_test test_13_init_branch_nonexistent "Test 13: init --branch nonexistent"

echo ""
print_test_summary

cleanup_tempdir

[ "$TEST_FAIL" -eq 0 ]
