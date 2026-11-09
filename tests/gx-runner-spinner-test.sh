#!/bin/bash
# gx runner spinner & structured output test suite
# Usage: bash tests/gx-runner-spinner-test.sh
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

setup_runner_test() {
    setup_tempdir
    mkdir -p "$TEST_DIR"
    GX_RUN_DIR="$TEST_DIR"
}

# ======================================================================
# TEST 01 — Runner Success outputs clean message without symbols
# ======================================================================
test_01_runner_success_clean() {
    echo ""
    echo "=== Test 01: Runner Success outputs clean message without symbols ==="
    cleanup_tempdir
    setup_runner_test

    run_gx __ui runner success "Branch synced"
    assert_exit_code "runner success exits 0" 0
    assert_output_contains "shows message" "Branch synced"
    assert_output_not_contains "no checkmark glyph" "✓"
}

# ======================================================================
# TEST 02 — Runner Info outputs message
# ======================================================================
test_02_runner_info_formatting() {
    echo ""
    echo "=== Test 02: Runner Info outputs message ==="
    cleanup_tempdir
    setup_runner_test

    run_gx __ui runner info "7 files changed, 42 insertions(+)"
    assert_exit_code "runner info exits 0" 0
    assert_output_contains "shows info message" "7 files changed, 42 insertions(+)"
}

# ======================================================================
# TEST 03 — Runner Warn outputs message without warning symbol
# ======================================================================
test_03_runner_warn_formatting() {
    echo ""
    echo "=== Test 03: Runner Warn outputs message without warning symbol ==="
    cleanup_tempdir
    setup_runner_test

    run_gx __ui runner warn "Sensitive file detected" "Remove .env from repository"
    assert_exit_code "runner warn exits 0" 0
    assert_output_not_contains "no warning symbol" "⚠"
    assert_output_contains "shows warning message" "Sensitive file detected"
    assert_output_contains "shows hint line" "Remove .env from repository"
}

# ======================================================================
# TEST 04 — Runner State outputs state message
# ======================================================================
test_04_runner_state() {
    echo ""
    echo "=== Test 04: Runner State outputs state message ==="
    cleanup_tempdir
    setup_runner_test

    run_gx __ui runner state "Restoring stashed changes..."
    assert_exit_code "runner state exits 0" 0
    assert_output_contains "contains state text" "Restoring stashed changes..."
}

# ======================================================================
# TEST 05 — Verbose mode preserves '▸ git ...' header
# ======================================================================
test_05_verbose_preserves_git_header() {
    echo ""
    echo "=== Test 05: Verbose mode preserves '▸ git ...' header ==="
    cleanup_tempdir
    setup_runner_test

    run_gx --verbose __ui runner run version
    assert_exit_code "verbose exits 0" 0
    assert_output_contains "contains git header with verbose" "▸ git version"
    assert_output_contains "contains git output with verbose" "git version"
}

# ======================================================================
# TEST 06 — Workflow ship outputs preferred states and no symbols
# ======================================================================
test_06_workflow_ship_notification() {
    echo ""
    echo "=== Test 06: Workflow ship outputs preferred states and no symbols ==="
    cleanup_tempdir
    setup_tempdir
    mkdir -p "$ORIGIN_DIR"
    cd "$ORIGIN_DIR"
    git init --bare 2>/dev/null

    mkdir -p "$TEST_DIR"
    git clone "$ORIGIN_DIR" "$TEST_DIR" 2>/dev/null
    cd "$TEST_DIR"
    configure_git

    git checkout -b develop 2>/dev/null
    echo "init" > file.txt && git add file.txt && git_commit -m "init"
    git_push -u origin develop

    git checkout -b feature/test 2>/dev/null
    echo "feat" >> file.txt && git add file.txt && git_commit -m "feat"

    write_config "develop" "origin" "rebase"
    GX_RUN_DIR="$TEST_DIR"

    run_gx ship --no-pr
    assert_exit_code "gx ship exits 0" 0
    assert_output_contains "ship shows state" "Pushing feature/test to origin..."
    assert_output_contains "ship shows Pushed" "Pushed feature/test → origin"
    assert_output_not_contains "no checkmark" "✓"
}

# ======================================================================
# TEST 07 — Workflow clean outputs clean notification without symbols
# ======================================================================
test_07_workflow_clean_notification() {
    echo ""
    echo "=== Test 07: Workflow clean outputs clean notification without symbols ==="
    cleanup_tempdir
    setup_tempdir
    mkdir -p "$ORIGIN_DIR"
    cd "$ORIGIN_DIR"
    git init --bare 2>/dev/null

    mkdir -p "$TEST_DIR"
    git clone "$ORIGIN_DIR" "$TEST_DIR" 2>/dev/null
    cd "$TEST_DIR"
    configure_git

    git checkout -b develop 2>/dev/null
    echo "init" > file.txt && git add file.txt && git_commit -m "init"
    git_push -u origin develop

    write_config "develop" "origin" "rebase"
    GX_RUN_DIR="$TEST_DIR"

    run_gx clean --yes
    assert_exit_code "gx clean exits 0" 0
    assert_output_contains "clean shows Pruned" "Pruned 0 branches."
    assert_output_not_contains "no checkmark" "✓"
}

# ======================================================================
# RUN ALL
# ======================================================================
cleanup_tempdir
setup_runner_test
build_gx
trap cleanup_tempdir EXIT

echo "============================================"
echo "  gx runner spinner & structured output — test suite"
echo "============================================"
echo ""

run_test test_01_runner_success_clean "Test 01: Runner Success outputs clean message without symbols"
run_test test_02_runner_info_formatting "Test 02: Runner Info outputs message"
run_test test_03_runner_warn_formatting "Test 03: Runner Warn outputs message without warning symbol"
run_test test_04_runner_state "Test 04: Runner State outputs state message"
run_test test_05_verbose_preserves_git_header "Test 05: Verbose mode preserves '▸ git ...' header"
run_test test_06_workflow_ship_notification "Test 06: Workflow ship outputs preferred states and no symbols"
run_test test_07_workflow_clean_notification "Test 07: Workflow clean outputs clean notification without symbols"

echo ""
print_test_summary
exit $FAIL
