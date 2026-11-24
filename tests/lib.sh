# gx test library
# Source from any tests/gx-*.sh script
#
# Usage:
#   SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
#   source "$SCRIPT_DIR/lib.sh"

set -e

# --- Paths ---
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
GX_BIN="$REPO_ROOT/gx"

# --- Test filter ---
TEST_FILTER="${TEST_FILTER:-${1:-}}"

# --- Counters ---
PASS=0
FAIL=0

# --- Build ---
build_gx() {
    if [ -n "$GX_SKIP_BUILD" ] && [ -x "$GX_BIN" ]; then
        return 0
    fi
    cd "$REPO_ROOT/src" && go build -o "$GX_BIN" . >/dev/null
}

# --- Suite root for golden template caching and sandboxed test runs ---
SUITE_ROOT="$(mktemp -d -t gx-suite-XXXXXX)"
TESTS_STARTED=0
trap cleanup_tempdir EXIT INT TERM

# --- Temp dir ---
setup_tempdir() {
    [ -n "$SUITE_ROOT" ] && [ -d "$SUITE_ROOT" ] || SUITE_ROOT="$(mktemp -d -t gx-suite-XXXXXX)"
    rm -rf "$SUITE_ROOT"/test-* 2>/dev/null || true
    TEST_ROOT=$(mktemp -d -p "$SUITE_ROOT" test-XXXXXX)
    TEST_DIR="$TEST_ROOT/repo"
    ORIGIN_DIR="$TEST_ROOT/origin"
    OTHER_DIR="$TEST_ROOT/other"
    CONFIG_FILE="$TEST_DIR/.gx/config"
}

cleanup_tempdir() {
    rm -rf "$TEST_ROOT" 2>/dev/null || true
    if [ "${IN_TEST:-0}" -eq 0 ] && [ "${TESTS_STARTED:-0}" -eq 1 ]; then
        rm -rf "$SUITE_ROOT" 2>/dev/null || true
    fi
}

# --- Configure git identity ---
# Required in CI environments where global user.name/user.email may be unset
configure_git() {
    git config user.name "GX Test"
    git config user.email "gx@test.local"
    git config core.fsync none 2>/dev/null || true
    git config core.compression 0 2>/dev/null || true
    git config gc.auto 0 2>/dev/null || true
    git config index.threads 1 2>/dev/null || true
}

# ======================================================================
# Git helper wrappers
# ======================================================================
# These suppress stdout/stderr for cleaner test output.
# If you need tracing later, add it here instead of touching every test.

git_checkout() { git checkout "$@" >/dev/null 2>&1; }
git_commit()   { git commit "$@" >/dev/null; }
git_push()     { git push "$@" >/dev/null 2>&1; }
git_pull()     { git pull "$@" >/dev/null 2>&1; }
git_fetch()    { git fetch "$@" >/dev/null 2>&1; }
git_head()     { git rev-parse HEAD; }

# ======================================================================
# Repository setup & template caching
# ======================================================================

# copy_template caches a setup function in $SUITE_ROOT and copies it via cp -a.
# Avoids re-executing dozens of git init / clone / commit / push commands per test.
copy_template() {
    local tmpl_name="$1"
    local setup_func="$2"
    local tmpl_dir="$SUITE_ROOT/tmpl_$tmpl_name"

    if [ ! -d "$tmpl_dir" ]; then
        mkdir -p "$tmpl_dir"
        local saved_test_root="$TEST_ROOT"
        local saved_test_dir="$TEST_DIR"
        local saved_origin_dir="$ORIGIN_DIR"
        local saved_other_dir="$OTHER_DIR"
        local saved_config_file="$CONFIG_FILE"

        TEST_ROOT="$tmpl_dir"
        TEST_DIR="$tmpl_dir/repo"
        ORIGIN_DIR="$tmpl_dir/origin"
        OTHER_DIR="$tmpl_dir/other"
        CONFIG_FILE="$TEST_DIR/.gx/config"

        "$setup_func"

        TEST_ROOT="$saved_test_root"
        TEST_DIR="$saved_test_dir"
        ORIGIN_DIR="$saved_origin_dir"
        OTHER_DIR="$saved_other_dir"
        CONFIG_FILE="$saved_config_file"
    fi

    rm -rf "$ORIGIN_DIR" "$TEST_DIR" "$OTHER_DIR" 2>/dev/null || true
    mkdir -p "$TEST_ROOT"
    [ -d "$tmpl_dir/origin" ] && cp -a "$tmpl_dir/origin" "$ORIGIN_DIR"
    [ -d "$tmpl_dir/repo" ]   && cp -a "$tmpl_dir/repo"   "$TEST_DIR"
    [ -d "$tmpl_dir/other" ]  && cp -a "$tmpl_dir/other"  "$OTHER_DIR"

    # Ensure local origin urls point to the test's ORIGIN_DIR
    if [ -d "$TEST_DIR/.git" ] && [ -d "$ORIGIN_DIR" ]; then
        git -C "$TEST_DIR" remote set-url origin "$ORIGIN_DIR" 2>/dev/null || true
    fi
    if [ -d "$OTHER_DIR/.git" ] && [ -d "$ORIGIN_DIR" ]; then
        git -C "$OTHER_DIR" remote set-url origin "$ORIGIN_DIR" 2>/dev/null || true
    fi

    [ -d "$TEST_DIR" ] && cd "$TEST_DIR"
}

_setup_base_repo_impl() {
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

    # feature branch
    git checkout -b feature/test
    echo "feature work" >> readme.md && git add . && git_commit -m "feature work"
    git_push -u origin feature/test

    # other user advances develop on origin (different file — no conflict)
    git clone "$ORIGIN_DIR" "$OTHER_DIR" 2>/dev/null
    cd "$OTHER_DIR"
    configure_git
    git checkout develop >/dev/null 2>&1
    echo "# gx project" > Makefile
    git add . && git_commit -m "add Makefile"
    git_push

    # also push feature branch ahead
    git_checkout feature/test
    echo "other feature work" >> feature.txt && git add . && git_commit -m "other feat work"
    git_push

    # go back to test repo and write default config
    cd "$TEST_DIR"
    write_config "develop" "origin" "rebase"
}

# Creates a bare origin, clones test repo, creates develop + feature/test
# branches, pushes everything, and sets up OTHER_DIR for divergence.
setup_base_repo() {
    copy_template "base" _setup_base_repo_impl
}

# ======================================================================
# Config
# ======================================================================
write_config() {
    local branch="$1" remote="$2" strategy="$3"
    mkdir -p "$TEST_DIR/.gx"
    printf '{"remote":"%s","defaultBranch":"%s","syncStrategy":"%s"}\n' \
        "$remote" "$branch" "$strategy" > "$CONFIG_FILE"
}

# ======================================================================
# Run gx
# ======================================================================
# Captures both exit code (GX_STATUS) and output (GX_OUTPUT).
run_gx() {
    local saved_opts
    saved_opts=$(set +o)
    set +e
    (
        cd "${GX_RUN_DIR:-$TEST_DIR}" >/dev/null 2>&1 || exit 1
        "$GX_BIN" "$@"
    ) >"$TEST_ROOT/gx-out.txt" 2>&1 </dev/null
    GX_STATUS=$?
    eval "$saved_opts"
    GX_OUTPUT=$(cat "$TEST_ROOT/gx-out.txt")
}

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

# ======================================================================
# Pass / fail primitives
# ======================================================================

pass() {
    echo "  ✓ $1"
    PASS=$((PASS + 1))
}

fail() {
    echo "  ✗ $1"
    FAIL=$((FAIL + 1))
}

# ======================================================================
# Assertion helpers
# ======================================================================

assert_exit_code() {
    local label="$1" expected="$2"
    if [ "$GX_STATUS" -eq "$expected" ]; then
        pass "$label"
    else
        fail "$label — expected exit code $expected, got $GX_STATUS"
        echo "    output: $(echo "$GX_OUTPUT" | head -3)"
    fi
}

assert_output_contains() {
    local label="$1" needle="$2"
    if echo "$GX_OUTPUT" | grep -Fq -- "$needle"; then
        pass "$label"
    else
        fail "$label — expected output to contain: $needle"
        echo "    got: $(echo "$GX_OUTPUT" | head -5)"
    fi
}

assert_output_not_contains() {
    local label="$1" needle="$2"
    if ! echo "$GX_OUTPUT" | grep -Fq -- "$needle"; then
        pass "$label"
    else
        fail "$label — expected output NOT to contain: $needle"
    fi
}

# --- Git state assertions ---

assert_branch() {
    local label="$1" expected="$2"
    local actual
    actual=$(cd "$TEST_DIR" && git branch --show-current 2>/dev/null)
    if [ "$actual" = "$expected" ]; then
        pass "$label"
    else
        fail "$label — expected branch '$expected', got '$actual'"
    fi
}

assert_clean() {
    local label="$1"
    local status
    status=$(cd "$TEST_DIR" && git status --porcelain --untracked-files=no 2>/dev/null)
    if [ -z "$status" ]; then
        pass "$label"
    else
        fail "$label — working tree has uncommitted changes"
        echo "    $status"
    fi
}

assert_dirty() {
    local label="$1"
    local status
    status=$(cd "$TEST_DIR" && git status --porcelain --untracked-files=no 2>/dev/null)
    if [ -n "$status" ]; then
        pass "$label"
    else
        fail "$label — expected dirty working tree, got clean"
    fi
}

assert_merge_base_equals() {
    local label="$1" ancestor="$2" descendant="$3"
    local merge_base rev_ancestor
    merge_base=$(cd "$TEST_DIR" && git merge-base "$ancestor" "$descendant" 2>/dev/null)
    rev_ancestor=$(cd "$TEST_DIR" && git rev-parse "$ancestor" 2>/dev/null)
    if [ "$merge_base" = "$rev_ancestor" ]; then
        pass "$label"
    else
        fail "$label — $ancestor is not an ancestor of $descendant"
    fi
}

assert_head_changed() {
    local label="$1" old_head="$2"
    local new_head
    new_head=$(cd "$TEST_DIR" && git_head)
    if [ "$old_head" != "$new_head" ]; then
        pass "$label"
    else
        fail "$label — HEAD did not change (still $old_head)"
    fi
}

assert_no_rebase_in_progress() {
    local label="$1"
    if [ ! -d "$TEST_DIR/.git/rebase-merge" ] && [ ! -d "$TEST_DIR/.git/rebase-apply" ]; then
        pass "$label"
    else
        fail "$label — rebase still in progress"
    fi
}

assert_rebase_in_progress() {
    local label="$1"
    if [ -d "$TEST_DIR/.git/rebase-merge" ] || [ -d "$TEST_DIR/.git/rebase-apply" ]; then
        pass "$label"
    else
        fail "$label — expected rebase in progress, none found"
    fi
}

assert_merge_in_progress() {
    local label="$1"
    if (cd "$TEST_DIR" && git rev-parse -q --verify MERGE_HEAD >/dev/null 2>&1); then
        pass "$label"
    else
        fail "$label — expected merge in progress, none found"
    fi
}

assert_no_merge_in_progress() {
    local label="$1"
    if ! (cd "$TEST_DIR" && git rev-parse -q --verify MERGE_HEAD >/dev/null 2>&1); then
        pass "$label"
    else
        fail "$label — merge still in progress"
    fi
}

assert_git_valid() {
    local label="$1"
    local rc=0
    (cd "$TEST_DIR" && git fsck --no-dangling >/dev/null 2>&1) || rc=$?
    if [ "$rc" -eq 0 ]; then
        pass "$label"
    else
        fail "$label — repository integrity check failed"
    fi
}

assert_file_exists() {
    local label="$1" file="$2"
    if [ -f "$file" ]; then
        pass "$label"
    else
        fail "$label — expected file to exist: $file"
    fi
}

assert_file_not_exists() {
    local label="$1" file="$2"
    if [ ! -f "$file" ]; then
        pass "$label"
    else
        fail "$label — expected file not to exist: $file"
    fi
}

# ======================================================================
# Test runner — wraps a test function, captures output, tracks pass/fail
# ======================================================================
# Usage: run_test test_func_name "Test NN: human-readable label"
#
# Runs the test function in a subshell (isolates set -e aborts).
# Detects failure by: non-zero exit OR any "✗" in assertion output.
# Emits ##TEST_PASS: or ##TEST_FAIL: marker for run-all.sh to consume.

TEST_PASS=0
TEST_FAIL=0

run_test() {
    TESTS_STARTED=1
    local test_func="$1"
    local test_label="$2"

    if [ -n "$TEST_FILTER" ]; then
        if [[ "$test_func" != *"$TEST_FILTER"* ]] && [[ "$test_label" != *"$TEST_FILTER"* ]]; then
            return 0
        fi
    fi

    local output rc
    output=$(IN_TEST=1 "$test_func" 2>&1)
    rc=$?

    echo "$output"

    if [ "$rc" -ne 0 ] || echo "$output" | grep -qF "✗"; then
        echo "##TEST_FAIL:$test_label"
        TEST_FAIL=$((TEST_FAIL + 1))
        return 1
    else
        echo "##TEST_PASS:$test_label"
        TEST_PASS=$((TEST_PASS + 1))
        return 0
    fi
}

print_test_summary() {
    TESTS_STARTED=1
    echo "============================================"
    if [ -n "$TEST_FILTER" ]; then
        echo "  Tests (filter '$TEST_FILTER'): $TEST_PASS passed, $TEST_FAIL failed"
    else
        echo "  Tests: $TEST_PASS passed, $TEST_FAIL failed"
    fi
    echo "============================================"
}
