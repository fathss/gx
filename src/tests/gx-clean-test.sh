#!/bin/bash
# gx clean — comprehensive test suite
# Usage: bash tests/gx-clean-test.sh
# Re-runnable: cleans up and recreates everything each run.
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

# ======================================================================
# BEHAVIORAL CONTRACT (derived from docs, before reading implementation)
#
# ID   Class             Precondition                   Action            Expected outcome                                   Spec ref
# C01  SPEC              merged local branch            gx clean          local branch pruned, exit 0                        docs/commands/clean.md#normal-flow
# C02  SPEC              merged remote-tracking ref     gx clean          remote-tracking ref pruned, exit 0                 docs/commands/clean.md#normal-flow
# C03  SPEC              protected branch (main/dev)    gx clean          protected branch not pruned, exit 0                docs/commands/clean.md#edge-cases
# C04  SPEC              current checked-out branch     gx clean          current branch not pruned, exit 0                  docs/commands/clean.md#edge-cases
# C05  SPEC              default branch itself          gx clean          default branch not pruned, exit 0                  docs/commands/clean.md#edge-cases
# C06  SPEC              unmerged branch                gx clean          unmerged branch not pruned, exit 0                 docs/commands/clean.md#edge-cases
# C07  SPEC              merged branch on remote        gx clean          remote branch not deleted without --remote         docs/commands/clean.md#what-it-deletes
# C08  SPEC              merged remote branch           gx clean --remote prompts with branch list [y/N], exit 0             docs/commands/clean.md#candidate-selection
# C09  SPEC              confirmed --remote (y)         gx clean --remote deletes remote branch on origin, exit 0            docs/commands/clean.md#normal-flow
# C10  SPEC              declined --remote (n)          gx clean --remote remote branch intact, local/tracking pruned        docs/commands/clean.md#normal-flow
# C11  SPEC              empty input on prompt          gx clean --remote defaults to decline, remote branch intact          docs/commands/clean.md#edge-cases
# C12  SPEC              no local branch (collaborator) gx clean --remote prunes remote-tracking ref and remote branch       docs/commands/clean.md#edge-cases
# C13  SPEC              local-only merge (unpushed)    gx clean          remote ref not pruned (uses remote base), exit 0   docs/commands/clean.md#candidate-selection
# C14  SPEC              dirty working tree             gx clean          prunes branches, dirty files untouched, exit 0     docs/commands/clean.md#pre-flight
# C15  SPEC              outside a git repository       gx clean          exit 1, reports not inside a Git repository        docs/commands/clean.md#pre-flight
# C16  SPEC              missing .gx/config             gx clean          exit 1, hint to run gx init                        docs/commands/clean.md#pre-flight
# C17  SPEC              no branches to prune           gx clean          exits 0, reports "Pruned 0 branches."              docs/commands/clean.md#edge-cases
# C18  SPEC              <remote>/HEAD symbolic ref     gx clean          symbolic ref not pruned or corrupted, exit 0       docs/commands/clean.md#candidate-selection
# C19  SPEC              remote fetch fails             gx clean          exit 1, reports fetch failure, repo untouched      docs/commands/clean.md#edge-cases
# C20  INVARIANT         any repository state           gx clean          no commits lost, repo intact                       docs/commands/clean.md#usage
# C21  INVARIANT         any clean completion           gx clean          repository integrity passes fsck                   (safety)
# C22  INVARIANT         clean run twice in a row       gx clean          second run is idempotent no-op, exit 0             (safety)
# C23  EXPECTED-FAIL    detached HEAD                  gx clean          prunes merged branches, leaves HEAD detached       docs/commands/clean.md#candidate-selection
# ======================================================================

# ======================================================================
# Helpers
# ======================================================================

# run_gx_stdin runs gx with a given stdin string and captures output.
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

setup_clean_repo() {
    cleanup_tempdir
    setup_tempdir
    mkdir -p "$ORIGIN_DIR"
    cd "$ORIGIN_DIR"
    git init --bare 2>/dev/null
    git symbolic-ref HEAD refs/heads/develop 2>/dev/null || true

    mkdir -p "$TEST_DIR"
    git clone "$ORIGIN_DIR" "$TEST_DIR" 2>/dev/null
    cd "$TEST_DIR"
    configure_git
    git checkout -b develop
    echo "initial" > readme.md
    git add readme.md && git_commit -m "init"
    git_push -u origin develop

    # Ensure origin HEAD points to develop
    git --git-dir="$ORIGIN_DIR" symbolic-ref HEAD refs/heads/develop 2>/dev/null || true

    write_config "develop" "origin" "rebase"
    echo ".gx/" >> "$TEST_DIR/.git/info/exclude"

    # Create OTHER_DIR as second clone (collaborator)
    git clone "$ORIGIN_DIR" "$OTHER_DIR" 2>/dev/null
    cd "$OTHER_DIR"
    configure_git
    git checkout develop >/dev/null 2>&1
    cd "$TEST_DIR"
}

# Helper: create a feature branch, push it, then merge into develop and push develop
# Usage: create_merged_branch <branch_name> [workdir]
create_merged_branch() {
    local branch="$1"
    local wdir="${2:-$TEST_DIR}"
    cd "$wdir"
    git_checkout -b "$branch" 2>/dev/null || git checkout -b "$branch" >/dev/null 2>&1
    echo "$branch work" > "${branch//\//_}.txt"
    git add "${branch//\//_}.txt" && git_commit -m "$branch work"
    git_push -u origin "$branch" 2>/dev/null || git_push origin "$branch" >/dev/null 2>&1
    git_checkout develop >/dev/null 2>&1
    git merge --no-ff "$branch" -m "merge $branch" >/dev/null 2>&1
    git_push origin develop >/dev/null 2>&1
}

# Helper: create a branch and push but do NOT merge (unmerged)
create_unmerged_branch() {
    local branch="$1"
    local wdir="${2:-$TEST_DIR}"
    cd "$wdir"
    git_checkout -b "$branch" 2>/dev/null || git checkout -b "$branch" >/dev/null 2>&1
    echo "$branch work" > "${branch//\//_}_un.txt"
    git add "${branch//\//_}_un.txt" && git_commit -m "$branch work unmerged"
    git_push -u origin "$branch" >/dev/null 2>&1
    git_checkout develop >/dev/null 2>&1
}

# Assertions for branch existence
assert_local_branch_exists() {
    local label="$1" branch="$2"
    if (cd "$TEST_DIR" && git show-ref --verify --quiet "refs/heads/$branch" 2>/dev/null); then
        pass "$label"
    else
        fail "$label — expected local branch '$branch' to exist"
    fi
}

assert_local_branch_not_exists() {
    local label="$1" branch="$2"
    if (cd "$TEST_DIR" && git show-ref --verify --quiet "refs/heads/$branch" 2>/dev/null); then
        fail "$label — expected local branch '$branch' NOT to exist"
    else
        pass "$label"
    fi
}

assert_remote_tracking_exists() {
    local label="$1" branch="$2"
    local remote="${3:-origin}"
    if (cd "$TEST_DIR" && git rev-parse --verify --quiet "refs/remotes/$remote/$branch" >/dev/null 2>&1); then
        pass "$label"
    else
        fail "$label — expected remote-tracking ref '$remote/$branch' to exist"
    fi
}

assert_remote_tracking_not_exists() {
    local label="$1" branch="$2"
    local remote="${3:-origin}"
    if (cd "$TEST_DIR" && git rev-parse --verify --quiet "refs/remotes/$remote/$branch" >/dev/null 2>&1); then
        fail "$label — expected remote-tracking ref '$remote/$branch' NOT to exist"
    else
        pass "$label"
    fi
}

assert_remote_branch_exists() {
    local label="$1" branch="$2"
    if git --git-dir="$ORIGIN_DIR" show-ref --verify --quiet "refs/heads/$branch" 2>/dev/null; then
        pass "$label"
    else
        fail "$label — expected remote branch '$branch' to exist on origin"
    fi
}

assert_remote_branch_not_exists() {
    local label="$1" branch="$2"
    if git --git-dir="$ORIGIN_DIR" show-ref --verify --quiet "refs/heads/$branch" 2>/dev/null; then
        fail "$label — expected remote branch '$branch' NOT to exist on origin"
    else
        pass "$label"
    fi
}

# Invariant helpers
snapshot_state() {
    SNAP_HEAD=$(cd "$TEST_DIR" && git rev-parse HEAD 2>/dev/null || echo "")
    SNAP_BRANCH=$(cd "$TEST_DIR" && git branch --show-current 2>/dev/null || echo "")
    SNAP_STATUS=$(cd "$TEST_DIR" && git status --porcelain 2>/dev/null || echo "")
    SNAP_COMMITS=$(cd "$TEST_DIR" && git rev-list --all 2>/dev/null || echo "")
}

assert_no_commits_lost() {
    local label="$1"
    local lost=0
    if [ -n "$SNAP_COMMITS" ]; then
        for c in $SNAP_COMMITS; do
            if ! (cd "$TEST_DIR" && git cat-file -e "$c" 2>/dev/null); then
                lost=1
                break
            fi
        done
    fi
    if [ "$lost" -eq 0 ]; then
        pass "$label"
    else
        fail "$label — commit $c reachable before is no longer present"
    fi
}

assert_worktree_preserved() {
    local label="$1"
    local cur_status
    cur_status=$(cd "$TEST_DIR" && git status --porcelain 2>/dev/null || echo "")
    if [ "$cur_status" = "$SNAP_STATUS" ]; then
        pass "$label"
    else
        fail "$label — working tree status changed"
    fi
}

assert_state_unchanged() {
    local label="$1"
    local cur_head cur_branch cur_status
    cur_head=$(cd "$TEST_DIR" && git rev-parse HEAD 2>/dev/null || echo "")
    cur_branch=$(cd "$TEST_DIR" && git branch --show-current 2>/dev/null || echo "")
    cur_status=$(cd "$TEST_DIR" && git status --porcelain 2>/dev/null || echo "")
    if [ "$cur_head" = "$SNAP_HEAD" ] && [ "$cur_branch" = "$SNAP_BRANCH" ] && [ "$cur_status" = "$SNAP_STATUS" ]; then
        pass "$label"
    else
        fail "$label — repository state changed unexpectedly"
    fi
}

assert_recoverable() {
    local label="$1"
    if (cd "$TEST_DIR" && git fsck --no-dangling >/dev/null 2>&1); then
        pass "$label"
    else
        fail "$label — repository is not in a valid recoverable state"
    fi
}

assert_idempotent() {
    local label="$1"
    shift
    local subcmd=("$@")
    local head_before
    head_before=$(cd "$TEST_DIR" && git rev-parse HEAD 2>/dev/null || echo "")
    run_gx "${subcmd[@]}"
    local head_after
    head_after=$(cd "$TEST_DIR" && git rev-parse HEAD 2>/dev/null || echo "")
    if [ "$GX_STATUS" -eq 0 ] && [ "$head_before" = "$head_after" ]; then
        pass "$label"
    else
        fail "$label — command is not idempotent (status=$GX_STATUS, head changed)"
    fi
}

assert_exit_output_consistent() {
    local label="$1"
    if [ "$GX_STATUS" -eq 0 ]; then
        if echo "$GX_OUTPUT" | grep -Fq "✗"; then
            fail "$label — exit code 0 but found error marker ✗"
        else
            pass "$label"
        fi
    else
        if [ -z "$GX_OUTPUT" ]; then
            fail "$label — non-zero exit code but output was empty"
        else
            pass "$label"
        fi
    fi
}

# ======================================================================
# TEST 01 — Prune one merged local branch
# ======================================================================
# Contract: C01
# Class: SPEC
# Catches: clean failing to delete local branches that are fully merged into the base branch
test_01_prune_merged_local() {
    echo ""
    echo "=== Test 01: prune one merged local branch ==="
    setup_clean_repo
    cd "$TEST_DIR"
    create_merged_branch "feature/merged-local"
    git_checkout develop >/dev/null 2>&1

    assert_local_branch_exists "pre: local branch exists" "feature/merged-local"

    snapshot_state
    run_gx clean

    assert_exit_code "exits 0" 0
    assert_output_contains "shows Pruned summary" "Pruned"
    assert_local_branch_not_exists "local merged branch pruned" "feature/merged-local"
    assert_git_valid "repo integrity"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "clean output consistency"
}

# ======================================================================
# TEST 02 — Remote-tracking ref pruned (no --remote)
# ======================================================================
# Contract: C02
# Class: SPEC
# Catches: clean skipping deletion of remote-tracking refs for merged branches
test_02_prune_remote_tracking() {
    echo ""
    echo "=== Test 02: remote-tracking ref pruned ==="
    setup_clean_repo
    cd "$TEST_DIR"
    create_merged_branch "feature/remote-track"
    git_checkout develop >/dev/null 2>&1

    assert_remote_tracking_exists "pre: remote-tracking exists" "feature/remote-track"

    snapshot_state
    run_gx clean

    assert_exit_code "exits 0" 0
    assert_output_contains "shows Pruned summary" "Pruned"
    assert_remote_tracking_not_exists "remote-tracking ref pruned" "feature/remote-track"
    assert_remote_branch_exists "remote branch still exists without --remote" "feature/remote-track"
    assert_git_valid "repo integrity"
    assert_no_commits_lost "no commits lost"
}

# ======================================================================
# TEST 03 — Protected branch not pruned
# ======================================================================
# Contract: C03
# Class: SPEC
# Catches: clean deleting protected branches such as main or develop when merged
test_03_protected_not_pruned() {
    echo ""
    echo "=== Test 03: protected branch not pruned ==="
    setup_clean_repo
    cd "$TEST_DIR"
    git_checkout -b main 2>/dev/null || git checkout -b main >/dev/null 2>&1
    echo "main work" > main.txt
    git add main.txt && git_commit -m "main work"
    git_push origin main >/dev/null 2>&1 || true
    git_checkout develop >/dev/null 2>&1
    git merge --no-ff main -m "merge main" >/dev/null 2>&1
    git_push origin develop >/dev/null 2>&1

    assert_local_branch_exists "pre: protected branch exists" "main"

    snapshot_state
    run_gx clean

    assert_exit_code "exits 0" 0
    assert_local_branch_exists "protected branch still exists" "main"
    assert_git_valid "repo integrity"
    assert_no_commits_lost "no commits lost"
}

# ======================================================================
# TEST 04 — Current branch not pruned
# ======================================================================
# Contract: C04
# Class: SPEC
# Catches: clean attempting to delete the branch currently checked out in the working tree
test_04_current_not_pruned() {
    echo ""
    echo "=== Test 04: current branch not pruned ==="
    setup_clean_repo
    cd "$TEST_DIR"
    create_merged_branch "feature/current-merged"
    git_checkout feature/current-merged >/dev/null 2>&1

    snapshot_state
    run_gx clean

    assert_exit_code "exits 0" 0
    assert_local_branch_exists "current branch not pruned" "feature/current-merged"
    assert_branch "still on current branch" "feature/current-merged"
    assert_git_valid "repo integrity"
    assert_no_commits_lost "no commits lost"
}

# ======================================================================
# TEST 05 — Default branch not pruned
# ======================================================================
# Contract: C05
# Class: SPEC
# Catches: clean attempting to delete the default base branch itself
test_05_default_not_pruned() {
    echo ""
    echo "=== Test 05: default branch not pruned ==="
    setup_clean_repo
    cd "$TEST_DIR"
    git_checkout develop >/dev/null 2>&1
    assert_local_branch_exists "pre: default branch exists" "develop"

    snapshot_state
    run_gx clean

    assert_exit_code "exits 0" 0
    assert_local_branch_exists "default branch still exists" "develop"
    assert_branch "still on develop" "develop"
    assert_git_valid "repo integrity"
    assert_no_commits_lost "no commits lost"
}

# ======================================================================
# TEST 06 — Unmerged branch not pruned
# ======================================================================
# Contract: C06
# Class: SPEC
# Catches: clean deleting branches that still contain unmerged work
test_06_unmerged_not_pruned() {
    echo ""
    echo "=== Test 06: unmerged branch not pruned ==="
    setup_clean_repo
    cd "$TEST_DIR"
    create_unmerged_branch "feature/unmerged"

    assert_local_branch_exists "pre: unmerged branch exists" "feature/unmerged"

    snapshot_state
    run_gx clean

    assert_exit_code "exits 0" 0
    assert_local_branch_exists "unmerged branch still exists" "feature/unmerged"
    assert_remote_tracking_exists "unmerged remote-tracking ref still exists" "feature/unmerged"
    assert_git_valid "repo integrity"
    assert_no_commits_lost "no commits lost"
}

# ======================================================================
# TEST 07 — Remote branch not deleted without --remote
# ======================================================================
# Contract: C07
# Class: SPEC
# Catches: clean deleting remote branches when --remote flag was not specified
test_07_remote_not_deleted_without_flag() {
    echo ""
    echo "=== Test 07: remote branch not deleted without --remote ==="
    setup_clean_repo
    cd "$TEST_DIR"
    create_merged_branch "feature/no-remote-flag"
    git_checkout develop >/dev/null 2>&1

    assert_remote_branch_exists "pre: remote branch exists" "feature/no-remote-flag"

    snapshot_state
    run_gx clean

    assert_exit_code "exits 0" 0
    assert_remote_branch_exists "remote branch untouched without --remote" "feature/no-remote-flag"
    assert_local_branch_not_exists "local branch was pruned" "feature/no-remote-flag"
    assert_git_valid "repo integrity"
    assert_no_commits_lost "no commits lost"
}

# ======================================================================
# TEST 08 — --remote prompt lists candidate branches
# ======================================================================
# Contract: C08
# Class: SPEC
# Catches: clean failing to show the candidate remote branch list and confirmation prompt
test_08_remote_prompt_lists_branches() {
    echo ""
    echo "=== Test 08: --remote prompt lists candidate branches ==="
    setup_clean_repo
    cd "$TEST_DIR"
    create_merged_branch "feature/list-a"
    create_merged_branch "feature/list-b"
    git_checkout develop >/dev/null 2>&1

    snapshot_state
    run_gx_stdin "n" clean --remote

    assert_exit_code "exits 0" 0
    assert_output_contains "shows origin/feature/list-a" "origin/feature/list-a"
    assert_output_contains "shows origin/feature/list-b" "origin/feature/list-b"
    assert_output_contains "shows [y/N] prompt" "[y/N]"
    assert_remote_branch_exists "origin/feature/list-a still exists" "feature/list-a"
    assert_remote_branch_exists "origin/feature/list-b still exists" "feature/list-b"
    assert_git_valid "repo integrity"
}

# ======================================================================
# TEST 09 — --remote with confirm (y) deletes remote branches
# ======================================================================
# Contract: C09
# Class: SPEC
# Catches: clean failing to delete remote branches on origin after user confirms with y
test_09_remote_confirm_deletes() {
    echo ""
    echo "=== Test 09: --remote confirm (y) deletes remote branches ==="
    setup_clean_repo
    cd "$TEST_DIR"
    create_merged_branch "feature/remote-confirm"
    git_checkout develop >/dev/null 2>&1

    assert_remote_branch_exists "pre: remote branch exists" "feature/remote-confirm"

    snapshot_state
    run_gx_stdin "y" clean --remote

    assert_exit_code "exits 0" 0
    assert_output_contains "Pruned summary" "Pruned"
    assert_remote_branch_not_exists "remote branch deleted after confirm" "feature/remote-confirm"
    assert_local_branch_not_exists "local branch pruned" "feature/remote-confirm"
    assert_remote_tracking_not_exists "remote-tracking pruned" "feature/remote-confirm"
    assert_git_valid "repo integrity"
}

# ======================================================================
# TEST 10 — --remote with decline (n) keeps remote branches
# ======================================================================
# Contract: C10
# Class: SPEC
# Catches: clean deleting remote branches despite user answering n at confirmation prompt
test_10_remote_decline_keeps_remote() {
    echo ""
    echo "=== Test 10: --remote decline (n) keeps remote ==="
    setup_clean_repo
    cd "$TEST_DIR"
    create_merged_branch "feature/remote-decline"
    git_checkout develop >/dev/null 2>&1

    assert_remote_branch_exists "pre: remote branch exists" "feature/remote-decline"

    snapshot_state
    run_gx_stdin "n" clean --remote

    assert_exit_code "exits 0" 0
    assert_remote_branch_exists "remote branch still intact after decline" "feature/remote-decline"
    assert_local_branch_not_exists "local branch pruned even on decline" "feature/remote-decline"
    assert_remote_tracking_not_exists "remote-tracking pruned even on decline" "feature/remote-decline"
    assert_git_valid "repo integrity"
}

# ======================================================================
# TEST 11 — --remote empty input defaults to decline
# ======================================================================
# Contract: C11
# Class: SPEC
# Catches: clean treating empty input as affirmative confirmation instead of default decline
test_11_remote_empty_defaults_decline() {
    echo ""
    echo "=== Test 11: --remote empty input defaults to decline ==="
    setup_clean_repo
    cd "$TEST_DIR"
    create_merged_branch "feature/empty-decline"
    git_checkout develop >/dev/null 2>&1

    assert_remote_branch_exists "pre: remote branch exists" "feature/empty-decline"

    snapshot_state
    run_gx_stdin "" clean --remote

    assert_exit_code "exits 0" 0
    assert_remote_branch_exists "remote branch still intact (empty = decline)" "feature/empty-decline"
    assert_local_branch_not_exists "local branch pruned" "feature/empty-decline"
    assert_git_valid "repo integrity"
}

# ======================================================================
# TEST 12 — Collaborator with no local branch still prunes remote
# ======================================================================
# Contract: C12
# Class: SPEC
# Catches: clean failing to discover or prune remote branches when local clone lacks local branch
test_12_no_local_still_prunes_remote() {
    echo ""
    echo "=== Test 12: no local branch still prunes remote ==="
    setup_clean_repo

    cd "$OTHER_DIR"
    git_checkout develop >/dev/null 2>&1
    create_merged_branch "feature/no-local" "$OTHER_DIR"

    cd "$TEST_DIR"
    git_fetch origin >/dev/null 2>&1

    assert_local_branch_not_exists "pre: no local branch" "feature/no-local"
    assert_remote_tracking_exists "pre: remote-tracking exists" "feature/no-local"
    assert_remote_branch_exists "pre: remote branch exists" "feature/no-local"

    snapshot_state
    run_gx_stdin "y" clean --remote

    assert_exit_code "exits 0" 0
    assert_remote_branch_not_exists "remote branch deleted" "feature/no-local"
    assert_remote_tracking_not_exists "remote-tracking pruned" "feature/no-local"
    assert_git_valid "repo integrity"
}

# ======================================================================
# TEST 13 — Remote candidates checked against <remote>/<defaultBranch>
# ======================================================================
# Contract: C13
# Class: SPEC
# Catches: clean using local base instead of remote base and deleting branches merged only locally
test_13_remote_candidates_use_remote_base() {
    echo ""
    echo "=== Test 13: remote candidates use remote base ==="
    setup_clean_repo
    cd "$TEST_DIR"
    git_checkout -b feature/only-local-merge develop >/dev/null 2>&1
    echo "only local" > only-local.txt
    git add only-local.txt && git_commit -m "only local merge work"
    git_push -u origin feature/only-local-merge >/dev/null 2>&1
    git_checkout develop >/dev/null 2>&1
    git merge --no-ff feature/only-local-merge -m "merge only-local" >/dev/null 2>&1

    assert_remote_tracking_exists "pre: remote-tracking exists" "feature/only-local-merge"

    snapshot_state
    run_gx clean

    assert_exit_code "exits 0" 0
    assert_remote_tracking_exists "remote-tracking not pruned (not merged on remote base)" "feature/only-local-merge"
    assert_local_branch_not_exists "local branch pruned (merged into local base)" "feature/only-local-merge"
    assert_git_valid "repo integrity"
}

# ======================================================================
# TEST 14 — Dirty working tree preserved
# ======================================================================
# Contract: C14
# Class: SPEC
# Catches: clean refusing to run or destroying uncommitted changes in dirty working tree
test_14_dirty_working_tree_preserved() {
    echo ""
    echo "=== Test 14: dirty working tree preserved ==="
    setup_clean_repo
    cd "$TEST_DIR"
    create_merged_branch "feature/dirty-test"
    git_checkout develop >/dev/null 2>&1

    echo "dirty content" >> readme.md

    snapshot_state
    run_gx clean

    assert_exit_code "exits 0" 0
    assert_output_contains "Pruned summary" "Pruned"
    assert_local_branch_not_exists "merged branch pruned despite dirty tree" "feature/dirty-test"
    assert_worktree_preserved "dirty changes preserved"
    assert_git_valid "repo integrity"
    assert_no_commits_lost "no commits lost"
}

# ======================================================================
# TEST 15 — Outside git repository
# ======================================================================
# Contract: C15
# Class: SPEC
# Catches: clean attempting git operations or crashing when executed outside a git repo
test_15_outside_git_repo() {
    echo ""
    echo "=== Test 15: outside git repository ==="
    setup_clean_repo

    snapshot_state
    GX_RUN_DIR=/tmp run_gx clean

    assert_exit_code "exits 1" 1
    assert_output_contains "mentions git repo" "Git repository"
    assert_state_unchanged "repo state untouched"
}

# ======================================================================
# TEST 16 — Missing .gx/config
# ======================================================================
# Contract: C16
# Class: SPEC
# Catches: clean proceeding with unconfigured defaults when .gx/config is missing
test_16_missing_config() {
    echo ""
    echo "=== Test 16: missing .gx/config ==="
    setup_clean_repo
    cd "$TEST_DIR"
    rm -rf "$TEST_DIR/.gx"

    snapshot_state
    run_gx clean

    assert_exit_code "exits 1" 1
    assert_output_contains "hint to run gx init" "gx init"
    assert_git_valid "repo integrity"
    assert_recoverable "repo recoverable"
}

# ======================================================================
# TEST 17 — Nothing to prune
# ======================================================================
# Contract: C17
# Class: SPEC
# Catches: clean returning non-zero or printing false errors when no branches need pruning
test_17_nothing_to_prune() {
    echo ""
    echo "=== Test 17: nothing to prune ==="
    setup_clean_repo
    cd "$TEST_DIR"
    git_checkout develop >/dev/null 2>&1

    snapshot_state
    run_gx clean

    assert_exit_code "exits 0" 0
    assert_output_contains "reports Pruned 0" "Pruned 0"
    assert_git_valid "repo integrity"
    assert_no_commits_lost "no commits lost"
    assert_exit_output_consistent "exit output consistent"
}

# ======================================================================
# TEST 18 — <remote>/HEAD symbolic ref preserved
# ======================================================================
# Contract: C18
# Class: SPEC
# Catches: clean attempting to delete origin/HEAD symbolic reference
test_18_remote_head_symref_preserved() {
    echo ""
    echo "=== Test 18: <remote>/HEAD symbolic ref preserved ==="
    setup_clean_repo
    cd "$TEST_DIR"
    git remote set-head origin develop >/dev/null 2>&1

    snapshot_state
    run_gx clean

    assert_exit_code "exits 0" 0
    assert_remote_tracking_exists "origin/HEAD symbolic ref intact" "HEAD"
    assert_git_valid "repo integrity"
    assert_no_commits_lost "no commits lost"
}

# ======================================================================
# TEST 19 — Remote fetch failure aborts cleanly
# ======================================================================
# Contract: C19
# Class: SPEC
# Catches: clean continuing with branch deletion when remote fetch fails
test_19_fetch_failure_aborts() {
    echo ""
    echo "=== Test 19: remote fetch failure aborts cleanly ==="
    setup_clean_repo
    cd "$TEST_DIR"
    create_merged_branch "feature/fetch-fail"
    git_checkout develop >/dev/null 2>&1

    # Break remote URL to provoke fetch failure
    git remote set-url origin "/nonexistent/path/to/origin" >/dev/null 2>&1

    snapshot_state
    run_gx clean

    assert_exit_code "exits non-zero" 1
    assert_output_contains "fetch error message" "fetch"
    assert_local_branch_exists "local branch not deleted on fetch failure" "feature/fetch-fail"
    assert_state_unchanged "repo state untouched on fetch failure"
    assert_recoverable "repo remains recoverable"
}

# ======================================================================
# TEST 20 — Invariant: no commits lost
# ======================================================================
# Contract: C20
# Class: INVARIANT
# Catches: clean causing commit loss on any branch or base
test_20_no_commits_lost_invariant() {
    echo ""
    echo "=== Test 20: invariant — no commits lost ==="
    setup_clean_repo
    cd "$TEST_DIR"
    create_merged_branch "feature/safety-commits"
    git_checkout develop >/dev/null 2>&1

    snapshot_state
    run_gx clean

    assert_exit_code "exits 0" 0
    assert_no_commits_lost "all commits remain reachable"
    assert_local_branch_not_exists "branch was safely pruned" "feature/safety-commits"
    assert_git_valid "repo integrity"
}

# ======================================================================
# TEST 21 — Invariant: git fsck repository validity
# ======================================================================
# Contract: C21
# Class: INVARIANT
# Catches: clean corrupting repository git objects or references
test_21_git_valid_invariant() {
    echo ""
    echo "=== Test 21: invariant — git fsck repository validity ==="
    setup_clean_repo
    cd "$TEST_DIR"
    create_merged_branch "feature/fsck-check"
    git_checkout develop >/dev/null 2>&1

    snapshot_state
    run_gx clean

    assert_exit_code "exits 0" 0
    assert_git_valid "repository integrity check passes fsck"
    assert_local_branch_not_exists "branch was safely pruned" "feature/fsck-check"
}

# ======================================================================
# TEST 22 — Invariant: clean is idempotent
# ======================================================================
# Contract: C22
# Class: INVARIANT
# Catches: clean failing or altering repository state on second consecutive execution
test_22_idempotent_clean() {
    echo ""
    echo "=== Test 22: invariant — clean is idempotent ==="
    setup_clean_repo
    cd "$TEST_DIR"
    create_merged_branch "feature/idempotent-test"
    git_checkout develop >/dev/null 2>&1

    snapshot_state
    run_gx clean
    assert_exit_code "first clean exits 0" 0
    assert_local_branch_not_exists "branch pruned after first clean" "feature/idempotent-test"

    snapshot_state
    assert_idempotent "second clean run is a no-op" clean
    assert_git_valid "repo integrity"
    assert_no_commits_lost "no commits lost"
}

# ======================================================================
# TEST 23 — Detached HEAD leaves HEAD detached and prunes merged branch
# ======================================================================
# Contract: C23
# Class: EXPECTED-FAIL
# Catches: clean crashing or misidentifying current branch when repository is in detached HEAD
# Expected: prunes merged branches, leaves HEAD detached (docs/commands/clean.md#candidate-selection)
# Actual: gx clean fails with "branch '(HEAD detached refs/heads/develop)' not found" — it
#         does not strip the "* " marker from the detached-HEAD branch listing and attempts
#         to delete the literal "(HEAD detached refs/heads/develop)" branch.
test_23_detached_head() {
    echo ""
    echo "=== Test 23: detached HEAD leaves HEAD detached ==="
    setup_clean_repo
    cd "$TEST_DIR"
    create_merged_branch "feature/detached-prune"
    git checkout --detach develop >/dev/null 2>&1

    assert_local_branch_exists "pre: branch exists" "feature/detached-prune"

    snapshot_state
    run_gx clean

    assert_exit_code "exits 0" 0
    assert_local_branch_not_exists "merged branch pruned under detached HEAD" "feature/detached-prune"
    assert_branch "HEAD is still detached" ""
    assert_git_valid "repo integrity"
    assert_no_commits_lost "no commits lost"
}

# ======================================================================
# RUN ALL
# ======================================================================
build_gx
trap cleanup_tempdir EXIT

echo "============================================"
echo "  gx clean — test suite"
echo "============================================"
echo ""

run_test test_01_prune_merged_local "Test 01: prune one merged local branch"
run_test test_02_prune_remote_tracking "Test 02: remote-tracking ref pruned"
run_test test_03_protected_not_pruned "Test 03: protected branch not pruned"
run_test test_04_current_not_pruned "Test 04: current branch not pruned"
run_test test_05_default_not_pruned "Test 05: default branch not pruned"
run_test test_06_unmerged_not_pruned "Test 06: unmerged branch not pruned"
run_test test_07_remote_not_deleted_without_flag "Test 07: remote branch not deleted without --remote"
run_test test_08_remote_prompt_lists_branches "Test 08: --remote prompt lists candidate branches"
run_test test_09_remote_confirm_deletes "Test 09: --remote confirm (y) deletes remote branches"
run_test test_10_remote_decline_keeps_remote "Test 10: --remote decline (n) keeps remote"
run_test test_11_remote_empty_defaults_decline "Test 11: --remote empty input defaults to decline"
run_test test_12_no_local_still_prunes_remote "Test 12: no local branch still prunes remote"
run_test test_13_remote_candidates_use_remote_base "Test 13: remote candidates use remote base"
run_test test_14_dirty_working_tree_preserved "Test 14: dirty working tree preserved"
run_test test_15_outside_git_repo "Test 15: outside git repository"
run_test test_16_missing_config "Test 16: missing .gx/config"
run_test test_17_nothing_to_prune "Test 17: nothing to prune"
run_test test_18_remote_head_symref_preserved "Test 18: <remote>/HEAD symbolic ref preserved"
run_test test_19_fetch_failure_aborts "Test 19: remote fetch failure aborts cleanly"
run_test test_20_no_commits_lost_invariant "Test 20: invariant — no commits lost"
run_test test_21_git_valid_invariant "Test 21: invariant — git fsck repository validity"
run_test test_22_idempotent_clean "Test 22: invariant — clean is idempotent"
run_test test_23_detached_head "Test 23: detached HEAD leaves HEAD detached"

echo ""
print_test_summary

cleanup_tempdir

[ "$TEST_FAIL" -eq 0 ]
