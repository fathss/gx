#!/bin/bash
# gx pager and diffstat test suite
# Usage: bash tests/gx-pager-diffstat-test.sh
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

setup_test_repo() {
    setup_tempdir
    mkdir -p "$ORIGIN_DIR"
    cd "$ORIGIN_DIR"
    git init --bare 2>/dev/null

    mkdir -p "$TEST_DIR"
    git clone "$ORIGIN_DIR" "$TEST_DIR" 2>/dev/null
    cd "$TEST_DIR"
    configure_git
    git checkout -b develop
    echo "initial line 1" > file1.txt
    echo "initial line 2" > file2.txt
    git add . && git_commit -m "init"
    git_push -u origin develop

    git checkout -b feature/test
    echo "feature work" >> file1.txt && git add . && git_commit -m "feature work"
    git_push -u origin feature/test

    write_config "develop" "origin" "rebase"
    echo ".gx/" >> "$TEST_DIR/.git/info/exclude"
}

# ======================================================================
# TEST 01 — gx status shows diff stats for staged and unstaged files
# ======================================================================
test_01_status_diffstat() {
    echo ""
    echo "=== Test 01: gx status shows diff stats for staged and unstaged files ==="
    cleanup_tempdir
    setup_test_repo

    cd "$TEST_DIR"
    # Create staged change: 2 additions, 1 deletion
    echo "modified staged" > file1.txt
    echo "new line staged" >> file1.txt
    git add file1.txt

    # Create unstaged change: 1 addition
    echo "more unstaged" >> file2.txt

    run_gx status

    assert_exit_code "exits 0" 0
    assert_output_contains "staged section shown" "Staged:"
    assert_output_contains "staged file listed" "file1.txt"
    assert_output_contains "staged diff stat shown" "(+"
    assert_output_contains "unstaged section shown" "Unstaged:"
    assert_output_contains "unstaged file listed" "file2.txt"
    assert_output_contains "unstaged diff stat shown" "(+1)"
}

# ======================================================================
# TEST 02 — gx save preview shows diff stats
# ======================================================================
test_02_save_diffstat() {
    echo ""
    echo "=== Test 02: gx save preview shows diff stats ==="
    cleanup_tempdir
    setup_test_repo

    cd "$TEST_DIR"
    echo "updated line" >> file1.txt

    run_gx save -m "feat: save with diffstat"

    assert_exit_code "exits 0" 0
    assert_output_contains "modified tracked section" "Modified tracked files:"
    assert_output_contains "diff stat in preview" "file1.txt (+1)"
    assert_output_contains "commit message" "feat: save with diffstat"
}

# ======================================================================
# TEST 03 — gx --no-pager flag works before and after commands
# ======================================================================
test_03_no_pager_flag() {
    echo ""
    echo "=== Test 03: gx --no-pager flag works before and after commands ==="
    cleanup_tempdir
    setup_test_repo

    cd "$TEST_DIR"
    run_gx --no-pager status
    assert_exit_code "--no-pager before command exits 0" 0
    assert_output_contains "status output shown" "On branch feature/test"

    run_gx status --no-pager
    assert_exit_code "--no-pager after command exits 0" 0
    assert_output_contains "status output shown" "On branch feature/test"

    run_gx --no-pager config
    assert_exit_code "--no-pager config exits 0" 0
    assert_output_contains "config listed" "defaultBranch"

    run_gx clean --no-pager --yes
    assert_exit_code "--no-pager clean exits 0" 0
}

# ======================================================================
# Runner
# ======================================================================
echo "============================================"
echo "  gx pager & diffstat — test suite"
echo "============================================"

run_test test_01_status_diffstat "Test 01: status shows diff stats"
run_test test_02_save_diffstat "Test 02: save preview shows diff stats"
run_test test_03_no_pager_flag "Test 03: --no-pager flag works"

echo ""
echo "============================================"
echo "  Tests: $TEST_PASS passed, $TEST_FAIL failed"
echo "============================================"

[ "$TEST_FAIL" -eq 0 ]
