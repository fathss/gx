#!/bin/bash
# Invariant helpers shared by the deep gx test suites.
# Source after tests/lib.sh:
#   source "$SCRIPT_DIR/lib.sh"
#   source "$SCRIPT_DIR/lib-invariants.sh"

assert_exit_nonzero() {
    local label="$1"
    if [ "$GX_STATUS" -ne 0 ]; then
        pass "$label"
    else
        fail "$label — expected non-zero exit, got 0"
    fi
}

assert_head_unchanged() {
    local label="$1" expected="$2"
    local actual
    actual=$(cd "$TEST_DIR" && git rev-parse HEAD 2>/dev/null || echo "")
    if [ "$actual" = "$expected" ]; then
        pass "$label"
    else
        fail "$label — expected HEAD $expected, got $actual"
    fi
}

assert_file_equals() {
    local label="$1" file="$2" expected="$3"
    local actual
    actual=$(cd "$TEST_DIR" && cat "$file" 2>/dev/null || echo "")
    if [ "$actual" = "$expected" ]; then
        pass "$label"
    else
        fail "$label — expected '$expected', got '$actual'"
    fi
}

assert_file_contains() {
    local label="$1" file="$2" needle="$3"
    if (cd "$TEST_DIR" && grep -Fq -- "$needle" "$file" 2>/dev/null); then
        pass "$label"
    else
        fail "$label — expected $file to contain: $needle"
    fi
}

assert_stash_count() {
    local label="$1" expected="$2"
    local actual
    actual=$(cd "$TEST_DIR" && git stash list 2>/dev/null | wc -l | tr -d ' ')
    if [ "$actual" = "$expected" ]; then
        pass "$label"
    else
        fail "$label — expected $expected stash entries, got $actual"
        echo "    $(cd "$TEST_DIR" && git stash list 2>/dev/null | head -5)"
    fi
}

assert_no_gx_stash() {
    local label="$1"
    if (cd "$TEST_DIR" && git stash list 2>/dev/null | grep -Fq "gx-sync/"); then
        fail "$label — gx-sync stash entry still present"
        echo "    $(cd "$TEST_DIR" && git stash list 2>/dev/null | grep -F 'gx-sync/')"
    else
        pass "$label"
    fi
}

snapshot_state() {
    SNAP_HEAD=$(cd "$TEST_DIR" && git rev-parse HEAD 2>/dev/null || echo "")
    SNAP_BRANCH=$(cd "$TEST_DIR" && git branch --show-current 2>/dev/null || echo "")
    SNAP_STATUS=$(cd "$TEST_DIR" && git status --porcelain 2>/dev/null || echo "")
    SNAP_DIFF=$(cd "$TEST_DIR" && git diff HEAD 2>/dev/null || echo "")
    SNAP_UNTRACKED=$(cd "$TEST_DIR" && git ls-files --others --exclude-standard 2>/dev/null || echo "")
    SNAP_COMMITS=$(cd "$TEST_DIR" && git rev-list --all 2>/dev/null || echo "")
}

snapshot_origin_refs() {
    SNAP_ORIGIN_REFS=$(git --git-dir="$ORIGIN_DIR" for-each-ref --format='%(refname) %(objectname)' 2>/dev/null | sort || echo "")
}

assert_no_commits_lost() {
    local label="$1"
    local lost=0 commit=""
    if [ -n "${SNAP_COMMITS:-}" ]; then
        for commit in $SNAP_COMMITS; do
            if ! (cd "$TEST_DIR" && git cat-file -e "$commit^{commit}" 2>/dev/null); then
                lost=1
                break
            fi
        done
    fi
    if [ "$lost" -eq 0 ]; then
        pass "$label"
    else
        fail "$label — commit $commit was present before but is gone"
    fi
}

assert_worktree_preserved() {
    local label="$1"
    local cur_diff cur_untracked
    cur_diff=$(cd "$TEST_DIR" && git diff HEAD 2>/dev/null || echo "")
    cur_untracked=$(cd "$TEST_DIR" && git ls-files --others --exclude-standard 2>/dev/null || echo "")
    if [ "$cur_diff" = "${SNAP_DIFF:-}" ] && [ "$cur_untracked" = "${SNAP_UNTRACKED:-}" ]; then
        pass "$label"
    else
        fail "$label — tracked or untracked working-tree content changed"
    fi
}

assert_state_unchanged() {
    local label="$1"
    local cur_head cur_branch cur_status cur_diff cur_untracked
    cur_head=$(cd "$TEST_DIR" && git rev-parse HEAD 2>/dev/null || echo "")
    cur_branch=$(cd "$TEST_DIR" && git branch --show-current 2>/dev/null || echo "")
    cur_status=$(cd "$TEST_DIR" && git status --porcelain 2>/dev/null || echo "")
    cur_diff=$(cd "$TEST_DIR" && git diff HEAD 2>/dev/null || echo "")
    cur_untracked=$(cd "$TEST_DIR" && git ls-files --others --exclude-standard 2>/dev/null || echo "")
    if [ "$cur_head" = "${SNAP_HEAD:-}" ] && [ "$cur_branch" = "${SNAP_BRANCH:-}" ] &&
       [ "$cur_status" = "${SNAP_STATUS:-}" ] && [ "$cur_diff" = "${SNAP_DIFF:-}" ] &&
       [ "$cur_untracked" = "${SNAP_UNTRACKED:-}" ]; then
        pass "$label"
    else
        fail "$label — repository state changed unexpectedly"
    fi
}

assert_origin_refs_unchanged() {
    local label="$1"
    local cur
    cur=$(git --git-dir="$ORIGIN_DIR" for-each-ref --format='%(refname) %(objectname)' 2>/dev/null | sort || echo "")
    if [ "$cur" = "${SNAP_ORIGIN_REFS:-}" ]; then
        pass "$label"
    else
        fail "$label — remote refs changed"
    fi
}

assert_recoverable() {
    local label="$1"
    if [ -d "$TEST_DIR/.git/rebase-merge" ] || [ -d "$TEST_DIR/.git/rebase-apply" ] ||
       (cd "$TEST_DIR" && git rev-parse -q --verify MERGE_HEAD >/dev/null 2>&1); then
        pass "$label"
        return
    fi
    assert_state_unchanged "$label"
}

assert_exit_output_consistent() {
    local label="$1"
    if [ "$GX_STATUS" -eq 0 ]; then
        if echo "$GX_OUTPUT" | grep -Fq "✗"; then
            fail "$label — exit 0 but an error marker was printed"
        else
            pass "$label"
        fi
    else
        if [ -z "$GX_OUTPUT" ]; then
            fail "$label — non-zero exit but no output"
        else
            pass "$label"
        fi
    fi
}

assert_idempotent() {
    local label="$1"
    shift
    local head_before head_after
    head_before=$(cd "$TEST_DIR" && git rev-parse HEAD 2>/dev/null || echo "")
    run_gx "$@"
    head_after=$(cd "$TEST_DIR" && git rev-parse HEAD 2>/dev/null || echo "")
    if [ "$GX_STATUS" -eq 0 ] && [ "$head_before" = "$head_after" ]; then
        pass "$label"
    else
        fail "$label — command was not a no-op (status=$GX_STATUS)"
    fi
}
