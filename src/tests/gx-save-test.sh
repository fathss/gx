#!/bin/bash
# gx save — comprehensive test suite
# Usage: bash tests/gx-save-test.sh
# Re-runnable: cleans up and recreates everything each run.
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

# ======================================================================
# Helpers
# ======================================================================

# run_gx_stdin runs gx with a given stdin string and captures output.
# Usage: run_gx_stdin <stdin> <args...>
run_gx_stdin() {
    local stdin_input="$1"
    shift
    local saved_opts
    saved_opts=$(set +o)
    set +e
    (
        cd "${GX_RUN_DIR:-$TEST_DIR}" >/dev/null 2>&1 || exit 1
        echo "$stdin_input" | "$GX_BIN" "$@"
    ) >"$TEST_ROOT/gx-out.txt" 2>&1
    GX_STATUS=$?
    eval "$saved_opts"
    GX_OUTPUT=$(cat "$TEST_ROOT/gx-out.txt")
}

# setup_save_repo creates a basic repo with develop branch and a feature branch
# with a modified tracked file. Optionally accepts extra files to create.
setup_save_repo() {
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

    git checkout -b feature/save
    echo "feature work" >> readme.md && git add . && git_commit -m "feature work"
    git_push -u origin feature/save

    # Write config
    write_config "develop" "origin" "rebase"

    # Exclude .gx/ from untracked listing so it doesn't interfere with tests
    echo ".gx/" >> "$TEST_DIR/.git/info/exclude"
}

# ======================================================================
# TEST 01 — success: modified tracked files with -m
# ======================================================================
test_01_success_modified() {
    echo ""
    echo "=== Test 01: success — modified tracked files ==="
    cleanup_tempdir
    setup_save_repo

    cd "$TEST_DIR"
    git_checkout feature/save
    echo "new change" >> readme.md

    run_gx save -m "feat: test save"

    assert_exit_code "exits 0" 0
    assert_output_not_contains "no error indicators" "✗"
    assert_branch "still on feature/save" "feature/save"
    assert_clean "working tree clean"
    assert_output_contains "commit message shown" "feat: test save"
    assert_git_valid "repository intact"
}

# ======================================================================
# TEST 02 — outside repo
# ======================================================================
test_02_outside_repo() {
    echo ""
    echo "=== Test 02: outside repo ==="
    cleanup_tempdir
    setup_save_repo

    GX_RUN_DIR=/tmp run_gx save -m "test"

    assert_exit_code "exits non-zero" 1
    assert_output_contains "message: Not inside a Git repository" "Git repository"
    assert_output_contains "hint given" "cloned repository"
}

# ======================================================================
# TEST 03 — no config
# ======================================================================
test_03_no_config() {
    echo ""
    echo "=== Test 03: no config ==="
    cleanup_tempdir
    setup_save_repo

    cd "$TEST_DIR"
    rm -rf "$TEST_DIR/.gx"
    run_gx save -m "test"

    assert_exit_code "exits non-zero" 1
    assert_output_contains "no config error" "No existing"
    assert_output_contains "hint shows gx init" "gx init"
}

# ======================================================================
# TEST 04 — rebase in progress blocks save
# ======================================================================
test_04_rebase_in_progress() {
    echo ""
    echo "=== Test 04: rebase in progress ==="
    cleanup_tempdir
    setup_save_repo

    cd "$TEST_DIR"
    git_checkout feature/save

    # Create a conflict scenario to leave a rebase mid-way
    git_checkout develop
    echo "develop content" > conflict.txt
    git add . && git_commit -m "develop conflict"
    git_push origin develop

    git_checkout feature/save
    git rebase origin/feature/save >/dev/null 2>&1
    echo "feature content" > conflict.txt
    git add . && git_commit -m "feature conflict"

    # Start rebase manually — will conflict
    git rebase develop >/dev/null 2>&1 || true

    run_gx save -m "should block"

    assert_exit_code "exits non-zero" 1
    assert_output_contains "rebase guard message" "rebase is already in progress"
    assert_rebase_in_progress "rebase still in progress"

    cd "$TEST_DIR" && git rebase --abort >/dev/null 2>&1 || true
}

# ======================================================================
# TEST 05 — merge in progress blocks save
# ======================================================================
test_05_merge_in_progress() {
    echo ""
    echo "=== Test 05: merge in progress ==="
    cleanup_tempdir
    setup_save_repo

    cd "$TEST_DIR"
    git_checkout feature/save

    # Create a conflict
    git_checkout develop
    echo "develop content" > conflict.txt
    git add . && git_commit -m "develop conflict"
    git_push origin develop

    git_checkout feature/save
    git fetch origin feature/save >/dev/null 2>&1
    echo "feature content" > conflict.txt
    git add . && git_commit -m "feature conflict"

    # Start merge manually — will conflict
    git merge develop >/dev/null 2>&1 || true

    run_gx save -m "should block"

    assert_exit_code "exits non-zero" 1
    assert_output_contains "merge guard message" "merge is already in progress"
    assert_merge_in_progress "merge still in progress"

    cd "$TEST_DIR" && git merge --abort >/dev/null 2>&1 || true
}

# ======================================================================
# TEST 06 — empty guard: nothing staged prompts (answer N)
# ======================================================================
test_06_empty_guard_no() {
    echo ""
    echo "=== Test 06: empty guard — nothing staged (answer N) ==="
    cleanup_tempdir
    setup_save_repo

    cd "$TEST_DIR"
    git_checkout feature/save

    # No changes — clean working tree
    run_gx_stdin "n" save -m "empty"

    assert_exit_code "exits non-zero" 1
    assert_output_contains "empty prompt" "Nothing staged"
    assert_output_contains "save aborted" "Save aborted"
}

# ======================================================================
# TEST 07 — empty guard: answer Y
# ======================================================================
test_07_empty_guard_yes() {
    echo ""
    echo "=== Test 07: empty guard — nothing staged (answer Y) ==="
    cleanup_tempdir
    setup_save_repo

    cd "$TEST_DIR"
    git_checkout feature/save

    # No changes — clean working tree. Answer yes.
    run_gx_stdin "y" save -m "empty"

    assert_exit_code "exits 0" 0
    assert_output_contains "empty commit" "empty"
    assert_git_valid "repository intact"
}

# ======================================================================
# TEST 08 — --allow-empty skips prompt
# ======================================================================
test_08_allow_empty() {
    echo ""
    echo "=== Test 08: --allow-empty ==="
    cleanup_tempdir
    setup_save_repo

    cd "$TEST_DIR"
    git_checkout feature/save

    # No changes, but --allow-empty should skip prompt
    run_gx save --allow-empty -m "empty via flag"

    assert_exit_code "exits 0" 0
    assert_output_not_contains "no prompt" "Nothing staged"
    assert_output_contains "empty commit" "empty via flag"
    assert_git_valid "repository intact"
}

# ======================================================================
# TEST 09 — sensitive pattern: hard-block
# ======================================================================
test_09_sensitive_block() {
    echo ""
    echo "=== Test 09: sensitive pattern — hard-block ==="
    cleanup_tempdir
    setup_save_repo

    cd "$TEST_DIR"
    git_checkout feature/save
    echo "API_KEY=secret" > .env
    git add .env >/dev/null 2>&1

    run_gx save -m "should block"

    assert_exit_code "exits non-zero" 1
    assert_output_contains "sensitive block message" "Sensitive file"
    assert_output_contains "matched pattern" ".env"
}

# ======================================================================
# TEST 10 — sensitive pattern: --allow-sensitive override
# ======================================================================
test_10_sensitive_allow() {
    echo ""
    echo "=== Test 10: sensitive pattern — --allow-sensitive ==="
    cleanup_tempdir
    setup_save_repo

    cd "$TEST_DIR"
    git_checkout feature/save
    echo "API_KEY=secret" > .env
    git add .env >/dev/null 2>&1

    run_gx save --allow-sensitive -m "override"

    assert_exit_code "exits 0" 0
    assert_output_contains "commit message" "override"
    assert_clean "working tree clean"
    assert_git_valid "repository intact"
}

# ======================================================================
# TEST 11 — sensitive pattern: custom config pattern
# ======================================================================
test_11_sensitive_custom_pattern() {
    echo ""
    echo "=== Test 11: sensitive pattern — custom config pattern ==="
    cleanup_tempdir
    setup_save_repo

    cd "$TEST_DIR"
    git_checkout feature/save

    # Configure custom patterns
    run_gx config sensitivePatterns '["*.tfvars"]'

    echo 'variable = "value"' > secrets.tfvars
    git add secrets.tfvars >/dev/null 2>&1

    run_gx save -m "should block custom"

    assert_exit_code "exits non-zero" 1
    assert_output_contains "custom pattern blocked" "Sensitive file"
    assert_output_contains "tfvars matched" "tfvars"
}

# ======================================================================
# TEST 12 — success with untracked files (confirm Y)
# ======================================================================
test_12_untracked_confirm_yes() {
    echo ""
    echo "=== Test 12: untracked files — confirm yes ==="
    cleanup_tempdir
    setup_save_repo

    cd "$TEST_DIR"
    git_checkout feature/save
    echo "new file" > newfile.go
    echo "change" >> readme.md

    # Answer y to untracked prompt
    run_gx_stdin "y" save -m "with untracked"

    assert_exit_code "exits 0" 0
    assert_clean "working tree clean"
    assert_output_contains "shows untracked section" "New untracked files"
    assert_output_contains "shows newfile.go" "newfile.go"
    assert_output_contains "commit message" "with untracked"
    assert_git_valid "repository intact"
}

# ======================================================================
# TEST 13 — untracked files: confirm N
# ======================================================================
test_13_untracked_confirm_no() {
    echo ""
    echo "=== Test 13: untracked files — confirm no ==="
    cleanup_tempdir
    setup_save_repo

    cd "$TEST_DIR"
    git_checkout feature/save
    echo "new file" > newfile.go
    echo "change" >> readme.md

    # Answer n to untracked prompt
    run_gx_stdin "n" save -m "only tracked"

    assert_exit_code "exits 0" 0
    assert_output_contains "commit message" "only tracked"
    assert_git_valid "repository intact"

    # The untracked file should still be present (not committed, not deleted)
    cd "$TEST_DIR"
    untracked_count=$(git ls-files --others --exclude-standard | wc -l)
    [ "$untracked_count" -ge 1 ] && pass "untracked file remains unstaged" || fail "untracked file was unexpectedly staged or deleted"
}

# ======================================================================
# TEST 14 — specific file args
# ======================================================================
test_14_specific_files() {
    echo ""
    echo "=== Test 14: specific file args ==="
    cleanup_tempdir
    setup_save_repo

    cd "$TEST_DIR"
    git_checkout feature/save
    echo "change1" >> readme.md
    echo "other" > other.txt
    git add other.txt >/dev/null 2>&1

    # Stage and commit only other.txt (readme.md change stays unstaged)
    run_gx save -m "specific file" other.txt

    assert_exit_code "exits 0" 0
    assert_output_contains "commit message" "specific file"
    assert_git_valid "repository intact"

    # readme.md should still be dirty (not staged)
    cd "$TEST_DIR"
    dirty_count=$(git status --porcelain -- readme.md | wc -l)
    [ "$dirty_count" -ge 1 ] && pass "readme.md remains dirty (not staged)" || fail "readme.md was unexpectedly staged"
}

# ======================================================================
# TEST 15 — detached HEAD
# ======================================================================
test_15_detached_head() {
    echo ""
    echo "=== Test 15: detached HEAD ==="
    cleanup_tempdir
    setup_save_repo

    cd "$TEST_DIR"
    git checkout --detach >/dev/null 2>&1
    echo "detached change" >> readme.md

    run_gx save -m "detached save"

    assert_exit_code "exits 0" 0
    assert_clean "working tree clean"
    assert_output_contains "commit message" "detached save"
    assert_git_valid "repository intact"
}

# ======================================================================
# TEST 16 — pre-existing staged files
# ======================================================================
test_16_prestaged_files() {
    echo ""
    echo "=== Test 16: pre-existing staged files ==="
    cleanup_tempdir
    setup_save_repo

    cd "$TEST_DIR"
    git_checkout feature/save

    # Manually stage one file
    echo "staged content" > staged.txt
    git add staged.txt >/dev/null 2>&1

    # Then modify another file (unstaged)
    echo "unstaged change" >> readme.md

    # gx save should include both — confirm the partial-staging warning
    run_gx_stdin "y" save -m "includes prestaged"

    assert_exit_code "exits 0" 0
    assert_clean "working tree clean"
    assert_output_contains "commit message" "includes prestaged"
    assert_git_valid "repository intact"

    # Verify both files are in the last commit
    cd "$TEST_DIR"
    commit_files=$(git diff-tree --no-commit-id --name-only -r HEAD)
    echo "$commit_files" | grep -q "staged.txt" && pass "staged.txt in commit" || fail "staged.txt missing from commit"
    echo "$commit_files" | grep -q "readme.md" && pass "readme.md in commit" || fail "readme.md missing from commit"
}

# ======================================================================
# TEST 17 — --exclude .env excludes a file from staging
# ======================================================================
test_17_exclude_single() {
    echo ""
    echo "=== Test 17: --exclude .env ==="
    cleanup_tempdir
    setup_save_repo

    cd "$TEST_DIR"
    git_checkout feature/save
    echo "SECRET=key" > .env
    echo "another change" >> readme.md

    run_gx save --exclude .env -m "exclude env"

    assert_exit_code "exits 0" 0
    assert_output_contains "commit message" "exclude env"
    assert_git_valid "repository intact"

    # Verify .env is NOT in the commit
    cd "$TEST_DIR"
    commit_files=$(git diff-tree --no-commit-id --name-only -r HEAD)
    echo "$commit_files" | grep -q ".env" && fail ".env was committed despite --exclude" || pass ".env excluded from commit"
    echo "$commit_files" | grep -q "readme.md" && pass "readme.md in commit" || fail "readme.md missing from commit"

    # .env should still be present (not deleted, just unstaged)
    [ -f .env ] && pass ".env still present in working tree" || fail ".env was deleted"
}

# ======================================================================
# TEST 18 — --exclude .env,.gitignore comma-separated
# ======================================================================
test_18_exclude_comma() {
    echo ""
    echo "=== Test 18: --exclude comma-separated ==="
    cleanup_tempdir
    setup_save_repo

    cd "$TEST_DIR"
    git_checkout feature/save
    echo "SECRET=key" > .env
    echo "*.log" > .gitignore
    echo "another change" >> readme.md

    run_gx save --exclude .env,.gitignore -m "comma exclude"

    assert_exit_code "exits 0" 0
    assert_output_contains "commit message" "comma exclude"
    assert_git_valid "repository intact"

    # Verify neither .env nor .gitignore is in the commit
    cd "$TEST_DIR"
    commit_files=$(git diff-tree --no-commit-id --name-only -r HEAD)
    echo "$commit_files" | grep -q ".env" && fail ".env was committed despite --exclude" || pass ".env excluded from commit"
    echo "$commit_files" | grep -q ".gitignore" && fail ".gitignore was committed despite --exclude" || pass ".gitignore excluded from commit"
    echo "$commit_files" | grep -q "readme.md" && pass "readme.md in commit" || fail "readme.md missing from commit"
}

# ======================================================================
# TEST 19 — --exclude .env --exclude secret.key repeatable
# ======================================================================
test_19_exclude_multi_flag() {
    echo ""
    echo "=== Test 19: --exclude repeatable ==="
    cleanup_tempdir
    setup_save_repo

    cd "$TEST_DIR"
    git_checkout feature/save
    echo "SECRET=key" > .env
    echo "mykey" > secret.key
    echo "another change" >> readme.md

    run_gx save --exclude .env --exclude secret.key -m "multi exclude"

    assert_exit_code "exits 0" 0
    assert_output_contains "commit message" "multi exclude"
    assert_git_valid "repository intact"

    # Verify neither .env nor secret.key is in the commit
    cd "$TEST_DIR"
    commit_files=$(git diff-tree --no-commit-id --name-only -r HEAD)
    echo "$commit_files" | grep -q ".env" && fail ".env was committed despite --exclude" || pass ".env excluded from commit"
    echo "$commit_files" | grep -q "secret.key" && fail "secret.key was committed despite --exclude" || pass "secret.key excluded from commit"
    echo "$commit_files" | grep -q "readme.md" && pass "readme.md in commit" || fail "readme.md missing from commit"
}

# ======================================================================
# TEST 20 — specific file + --exclude where exclude wins
# ======================================================================
test_20_exclude_overrides_file_arg() {
    echo ""
    echo "=== Test 20: exclude wins over explicit file arg ==="
    cleanup_tempdir
    setup_save_repo

    cd "$TEST_DIR"
    git_checkout feature/save
    echo "SECRET=key" > .env
    echo "another change" >> readme.md

    # Pass .env and readme.md as explicit file args, but --exclude .env — exclude should win for .env
    run_gx save -m "exclude wins" .env readme.md --exclude .env

    assert_exit_code "exits 0" 0
    assert_output_contains "commit message" "exclude wins"
    assert_git_valid "repository intact"

    # .env should NOT be in the commit (exclude wins over explicit file arg)
    cd "$TEST_DIR"
    commit_files=$(git diff-tree --no-commit-id --name-only -r HEAD)
    echo "$commit_files" | grep -q ".env" && fail ".env was committed despite --exclude" || pass ".env excluded from commit"
    # readme.md SHOULD be in the commit (was passed as file arg and not excluded)
    echo "$commit_files" | grep -q "readme.md" && pass "readme.md in commit (expected)" || fail "readme.md missing from commit"
}

# ======================================================================
# RUN ALL
# ======================================================================
cleanup_tempdir
build_gx
trap cleanup_tempdir EXIT

echo "============================================"
echo "  gx save — test suite"
echo "============================================"
echo ""

run_test test_01_success_modified "Test 01: success — modified tracked files"
run_test test_02_outside_repo "Test 02: outside repo"
run_test test_03_no_config "Test 03: no config"
run_test test_04_rebase_in_progress "Test 04: rebase in progress"
run_test test_05_merge_in_progress "Test 05: merge in progress"
run_test test_06_empty_guard_no "Test 06: empty guard — answer N"
run_test test_07_empty_guard_yes "Test 07: empty guard — answer Y"
run_test test_08_allow_empty "Test 08: --allow-empty"
run_test test_09_sensitive_block "Test 09: sensitive pattern — hard-block"
run_test test_10_sensitive_allow "Test 10: sensitive pattern — --allow-sensitive"
run_test test_11_sensitive_custom_pattern "Test 11: sensitive pattern — custom config"
run_test test_12_untracked_confirm_yes "Test 12: untracked files — confirm yes"
run_test test_13_untracked_confirm_no "Test 13: untracked files — confirm no"
run_test test_14_specific_files "Test 14: specific file args"
run_test test_15_detached_head "Test 15: detached HEAD"
run_test test_16_prestaged_files "Test 16: pre-existing staged files"
run_test test_17_exclude_single "Test 17: --exclude .env"
run_test test_18_exclude_comma "Test 18: --exclude comma-separated"
run_test test_19_exclude_multi_flag "Test 19: --exclude repeatable"
run_test test_20_exclude_overrides_file_arg "Test 20: exclude wins over explicit file arg"

echo ""
print_test_summary

cleanup_tempdir

[ "$TEST_FAIL" -eq 0 ]
