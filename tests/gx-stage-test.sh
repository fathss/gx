#!/bin/bash
# gx stage — comprehensive test suite
# Usage: bash tests/gx-stage-test.sh
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

setup_stage_repo() {
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
    echo "app code" > app.js
    git add . && git_commit -m "init"
    git_push -u origin develop

    git checkout -b feature/stage
    echo "feature work" >> readme.md && git add . && git_commit -m "feature work"
    git_push -u origin feature/stage

    write_config "develop" "origin" "rebase"
    echo ".gx/" >> "$TEST_DIR/.git/info/exclude"
}

# ======================================================================
# TEST 01 — success: modified tracked files
# ======================================================================
test_01_success_modified() {
    echo ""
    echo "=== Test 01: success — modified tracked files ==="
    cleanup_tempdir
    setup_stage_repo

    cd "$TEST_DIR"
    echo "more changes" >> readme.md

    run_gx stage
    assert_exit_code "exits 0" 0
    assert_output_contains "shows staged summary" "Staged 1 file"
    assert_output_contains "shows diffstat" "+1"

    # Verify file is actually in git index
    staged=$(git diff --name-only --cached)
    echo "$staged" | grep -q "readme.md" && pass "readme.md staged in index" || fail "readme.md not in index"
}

# ======================================================================
# TEST 02 — outside git repo
# ======================================================================
test_02_outside_repo() {
    echo ""
    echo "=== Test 02: outside repo ==="
    cleanup_tempdir
    setup_stage_repo

    GX_RUN_DIR=/tmp run_gx stage
    assert_exit_code "exits non-zero" 1
    assert_output_contains "message: Not inside a Git repository" "Git repository"
}

# ======================================================================
# TEST 03 — no config
# ======================================================================
test_03_no_config() {
    echo ""
    echo "=== Test 03: no config ==="
    cleanup_tempdir
    setup_stage_repo

    cd "$TEST_DIR"
    rm -rf "$TEST_DIR/.gx"

    run_gx stage
    assert_exit_code "exits non-zero" 1
    assert_output_contains "no config error" "No existing"
}

# ======================================================================
# TEST 04 — specific file args
# ======================================================================
test_04_specific_files() {
    echo ""
    echo "=== Test 04: specific file args ==="
    cleanup_tempdir
    setup_stage_repo

    cd "$TEST_DIR"
    echo "change in readme" >> readme.md
    echo "change in app" >> app.js

    run_gx stage app.js
    assert_exit_code "exits 0" 0
    assert_output_contains "staged 1 file" "Staged 1 file"

    staged=$(git diff --name-only --cached)
    echo "$staged" | grep -q "app.js" && pass "app.js staged" || fail "app.js not staged"
    echo "$staged" | grep -qv "readme.md" && pass "readme.md not staged" || fail "readme.md staged unexpectedly"
}

# ======================================================================
# TEST 05 — gx stage -a stages all files
# ======================================================================
test_05_stage_all() {
    echo ""
    echo "=== Test 05: gx stage -a stages all files ==="
    cleanup_tempdir
    setup_stage_repo

    cd "$TEST_DIR"
    echo "change in readme" >> readme.md
    echo "new untracked file" > new.txt

    run_gx stage -a
    assert_exit_code "exits 0" 0
    assert_output_contains "staged 2 files" "Staged 2 files"

    staged=$(git diff --name-only --cached)
    echo "$staged" | grep -q "readme.md" && pass "readme.md staged" || fail "readme.md missing"
    echo "$staged" | grep -q "new.txt" && pass "new.txt staged" || fail "new.txt missing"
}

# ======================================================================
# TEST 06 — sensitive pattern block and allow-sensitive
# ======================================================================
test_06_sensitive_pattern() {
    echo ""
    echo "=== Test 06: sensitive pattern block ==="
    cleanup_tempdir
    setup_stage_repo

    cd "$TEST_DIR"
    echo "SECRET=123" > .env

    run_gx stage
    assert_exit_code "exits non-zero" 1
    assert_output_contains "sensitive block" "Sensitive file"
    assert_output_contains "matched .env" ".env"

    run_gx stage -a --allow-sensitive
    assert_exit_code "allow-sensitive exits 0" 0
    staged=$(git diff --name-only --cached)
    echo "$staged" | grep -q ".env" && pass ".env staged with allow-sensitive" || fail ".env not staged"
}

# ======================================================================
# TEST 07 — clean tree: nothing to stage
# ======================================================================
test_07_clean_tree() {
    echo ""
    echo "=== Test 07: clean tree ==="
    cleanup_tempdir
    setup_stage_repo

    cd "$TEST_DIR"
    run_gx stage
    assert_exit_code "exits 0" 0
    assert_output_contains "clean tree message" "nothing to stage"
}

# ======================================================================
# TEST 08 — end-to-end: gx stage then gx save without prompts
# ======================================================================
test_08_stage_then_save_e2e() {
    echo ""
    echo "=== Test 08: end-to-end: gx stage then gx save ==="
    cleanup_tempdir
    setup_stage_repo

    cd "$TEST_DIR"
    echo "feature update" >> app.js
    echo "docs update" >> readme.md

    # Stage only app.js
    run_gx stage app.js
    assert_exit_code "gx stage exits 0" 0

    # Save should commit only app.js without prompting
    run_gx save -m "feat: update app"
    assert_exit_code "gx save exits 0" 0
    assert_output_contains "shows commit message" "feat: update app"
    assert_output_contains "commit card shows app.js" "app.js"

    # readme.md should still be dirty/unstaged
    unstaged=$(git status --porcelain)
    echo "$unstaged" | grep -q "readme.md" && pass "readme.md remains unstaged" || fail "readme.md was unexpectedly committed"
}

# ======================================================================
# TEST 09 — preview shows only unstaged files (ignores already staged)
# ======================================================================
test_09_only_unstaged_in_preview() {
    echo ""
    echo "=== Test 09: preview shows only unstaged files ==="
    cleanup_tempdir
    setup_stage_repo

    cd "$TEST_DIR"
    echo "already staged" >> app.js
    git add app.js

    echo "unstaged change" >> readme.md

    run_gx stage
    assert_exit_code "gx stage exits 0" 0
    assert_output_contains "shows unstaged section" "Unstaged tracked files:"
    assert_output_contains "shows readme.md" "readme.md"
    assert_output_not_contains "does not show already staged app.js" "app.js"
}

# ======================================================================
# TEST 10 — deleted file shown with D badge
# ======================================================================
test_10_deleted_file_badge() {
    echo ""
    echo "=== Test 10: deleted file shown with D badge ==="
    cleanup_tempdir
    setup_stage_repo

    cd "$TEST_DIR"
    rm app.js

    run_gx stage
    assert_exit_code "gx stage exits 0" 0
    assert_output_contains "shows deleted file with D badge" "D  app.js"

    staged=$(git diff --name-only --diff-filter=D --cached)
    echo "$staged" | grep -q "app.js" && pass "app.js staged as deleted" || fail "app.js not staged as deleted"
}

# ======================================================================
# RUN ALL
# ======================================================================
cleanup_tempdir
setup_stage_repo
build_gx
trap cleanup_tempdir EXIT

echo "============================================"
echo "  gx stage — test suite"
echo "============================================"
echo ""

run_test test_01_success_modified "Test 01: success — modified tracked files"
run_test test_02_outside_repo "Test 02: outside repo"
run_test test_03_no_config "Test 03: no config"
run_test test_04_specific_files "Test 04: specific file args"
run_test test_05_stage_all "Test 05: gx stage -a stages all files"
run_test test_06_sensitive_pattern "Test 06: sensitive pattern block & override"
run_test test_07_clean_tree "Test 07: clean tree"
run_test test_08_stage_then_save_e2e "Test 08: end-to-end: gx stage then gx save"
run_test test_09_only_unstaged_in_preview "Test 09: preview shows only unstaged files"
run_test test_10_deleted_file_badge "Test 10: deleted file shown with D badge"

echo ""
print_test_summary
exit $FAIL
