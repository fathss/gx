#!/bin/bash
# run-all.sh — Run every *-test.sh suite in the tests/ directory
# Usage: bash tests/run-all.sh [options]
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ALL_PASS=0
declare -a FAILED_SUITES
declare -a FAILED_TESTS

# Detect physical/virtual CPU limit
MAX_CORES=$(nproc 2>/dev/null || echo 2)
[ "$MAX_CORES" -lt 1 ] && MAX_CORES=1

# Concurrency default: max available cores
JOBS="$MAX_CORES"

# Parse CLI options
while [[ $# -gt 0 ]]; do
    case "$1" in
        -j|--jobs)
            if [ -z "${2:-}" ]; then
                echo "Error: -j/--jobs requires an argument" >&2
                exit 1
            fi
            RAW_VAL="$2"
            shift 2
            if [ "$RAW_VAL" = "max" ]; then
                JOBS="$MAX_CORES"
            elif [[ "$RAW_VAL" =~ ^[0-9]+$ ]] && [ "$RAW_VAL" -ge 1 ]; then
                JOBS="$RAW_VAL"
            else
                echo "Warning: Invalid concurrency '$RAW_VAL'; defaulting to 1 (serial)." >&2
                JOBS=1
            fi
            ;;
        --serial|-s)
            JOBS=1
            shift
            ;;
        -h|--help)
            echo "Usage: bash tests/run-all.sh [options]"
            echo ""
            echo "Options:"
            echo "  -j, --jobs <N|max>   Concurrency level (default: max, up to $MAX_CORES cores)"
            echo "  -s, --serial         Run suites sequentially (same as -j 1)"
            echo "  -h, --help           Show this help"
            echo ""
            echo "Environment variables:"
            echo "  GX_PARALLEL_JOBS     Default concurrency level (clamped to max CPU cores)"
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            echo "Run 'bash tests/run-all.sh --help' for usage." >&2
            exit 1
            ;;
    esac
done

# If set via env var and not overridden by CLI
if [ -n "${GX_PARALLEL_JOBS:-}" ] && [ "$JOBS" -eq "$MAX_CORES" ]; then
    if [ "$GX_PARALLEL_JOBS" = "max" ]; then
        JOBS="$MAX_CORES"
    elif [[ "$GX_PARALLEL_JOBS" =~ ^[0-9]+$ ]] && [ "$GX_PARALLEL_JOBS" -ge 1 ]; then
        JOBS="$GX_PARALLEL_JOBS"
    fi
fi

# CPU constraint clamp check
if [ "$JOBS" -gt "$MAX_CORES" ]; then
    echo "  Notice: Concurrency $JOBS exceeds available CPU cores ($MAX_CORES); clamped to $MAX_CORES."
    JOBS="$MAX_CORES"
fi

# GitHub Actions CI detection
IS_CI=false
if [ "${GITHUB_ACTIONS:-}" = "true" ] || [ "${CI:-}" = "true" ]; then
    IS_CI=true
fi

echo ""
echo "============================================"
echo "  gx — full test suite runner"
echo "============================================"
echo "  repo root:   $REPO_ROOT"
if [ "$JOBS" -eq 1 ]; then
    echo "  concurrency: 1 (serial mode)"
else
    echo "  concurrency: $JOBS workers (max CPU cores: $MAX_CORES)"
fi
if [ "$IS_CI" = "true" ]; then
    echo "  environment: CI / GitHub Actions"
fi
echo ""

# Clean up any stale test artifacts from previous interrupted runs
rm -rf /tmp/gx-suite-* /tmp/gx-runall-* 2>/dev/null || true

# Pre-build the binary once upfront so suites don't recompile
echo "Building gx..."
cd "$REPO_ROOT/src" && go build -o "$REPO_ROOT/gx" . >/dev/null
echo "  ✓ gx built at $REPO_ROOT/gx"
export GX_SKIP_BUILD=1

RUN_TMP=$(mktemp -d -t gx-runall-XXXXXX)
trap 'rm -f "$REPO_ROOT/gx"; rm -rf "$RUN_TMP" 2>/dev/null || true' EXIT INT TERM
echo ""

# Collect suites
suites=()
for s in "$SCRIPT_DIR"/*-test.sh; do
    [ -f "$s" ] || continue
    suites+=("$s")
done

total_suites=${#suites[@]}

if [ "$JOBS" -le 1 ]; then
    # ======================================================================
    # Serial Execution Mode
    # ======================================================================
    for suite in "${suites[@]}"; do
        name=$(basename "$suite")

        if [ "$IS_CI" = "true" ]; then
            echo "::group::Suite: $name"
        else
            echo "--------------------------------------------"
            echo "  Suite: $name"
            echo "--------------------------------------------"
        fi

        suite_out="$RUN_TMP/$name.out"
        set +e
        bash "$suite" 2>&1 | tee "$suite_out"
        rc=${PIPESTATUS[0]}
        set -e

        if [ "$IS_CI" = "true" ]; then
            echo "::endgroup::"
        fi

        if [ "$rc" -eq 0 ]; then
            echo "  ✓ $name — all tests passed"
        else
            echo "  ✗ $name — some tests FAILED"
            FAILED_SUITES+=("$name")
            ALL_PASS=1

            failed_tests=$(grep '^##TEST_FAIL:' "$suite_out" | sed 's/^##TEST_FAIL://' || true)
            if [ -n "$failed_tests" ]; then
                while IFS= read -r test_label; do
                    echo "      ✗ $test_label"
                    FAILED_TESTS+=("$name|$test_label")
                    if [ "$IS_CI" = "true" ]; then
                        echo "::error file=tests/$name::Test failure in $name: $test_label"
                    fi
                done <<< "$failed_tests"
            fi
        fi
        echo ""
    done
else
    # ======================================================================
    # Parallel Swarm Mode
    # ======================================================================
    run_suite_worker() {
        local suite="$1"
        local name="$2"
        local out="$RUN_TMP/$name.out"
        local stat="$RUN_TMP/$name.stat"

        local t0
        t0=$(date +%s)
        set +e
        bash "$suite" > "$out" 2>&1
        local rc=$?
        local t1
        t1=$(date +%s)
        local dur=$((t1 - t0))
        echo "$rc $dur" > "$stat"
    }

    declare -A active_jobs

    check_finished_jobs() {
        for pid in "${!active_jobs[@]}"; do
            local name="${active_jobs[$pid]}"
            local stat_file="$RUN_TMP/$name.stat"
            if [ -f "$stat_file" ]; then
                local rc dur
                read -r rc dur < "$stat_file"
                local out_file="$RUN_TMP/$name.out"

                if [ "$rc" -eq 0 ]; then
                    echo "  ✓ $name — all tests passed (${dur}s)"
                    if [ "$IS_CI" = "true" ]; then
                        echo "::group::Suite: $name (${dur}s)"
                        cat "$out_file"
                        echo "::endgroup::"
                    fi
                else
                    echo "  ✗ $name — some tests FAILED (${dur}s)"
                    FAILED_SUITES+=("$name")
                    ALL_PASS=1

                    local failed_tests
                    failed_tests=$(grep '^##TEST_FAIL:' "$out_file" | sed 's/^##TEST_FAIL://' || true)
                    if [ -n "$failed_tests" ]; then
                        while IFS= read -r test_label; do
                            echo "      ✗ $test_label"
                            FAILED_TESTS+=("$name|$test_label")
                            if [ "$IS_CI" = "true" ]; then
                                echo "::error file=tests/$name::Test failure in $name: $test_label"
                            fi
                        done <<< "$failed_tests"
                    fi

                    if [ "$IS_CI" = "true" ]; then
                        echo "::group::Suite: $name (FAILED - full output)"
                        cat "$out_file"
                        echo "::endgroup::"
                    else
                        echo "--------------------------------------------"
                        echo "  Output from failed suite: $name"
                        echo "--------------------------------------------"
                        cat "$out_file"
                        echo "--------------------------------------------"
                    fi
                fi

                unset "active_jobs[$pid]"
            fi
        done
    }

    echo "Running $total_suites test suites in parallel with $JOBS workers..."
    echo ""

    for suite in "${suites[@]}"; do
        name=$(basename "$suite")

        while [ "${#active_jobs[@]}" -ge "$JOBS" ]; do
            wait -n 2>/dev/null || true
            check_finished_jobs
            [ "${#active_jobs[@]}" -ge "$JOBS" ] && sleep 0.05
        done

        run_suite_worker "$suite" "$name" &
        active_jobs["$!"]="$name"
    done

    # Wait for all remaining background jobs to finish
    while [ "${#active_jobs[@]}" -gt 0 ]; do
        wait -n 2>/dev/null || true
        check_finished_jobs
        [ "${#active_jobs[@]}" -gt 0 ] && sleep 0.05
    done
    echo ""
fi

# ======================================================================
# Final Summary
# ======================================================================
echo "============================================"
if [ ${#FAILED_TESTS[@]} -eq 0 ]; then
    echo "  ✓ All test suites passed!"
else
    echo "  ✗ FAILED:"
    prev_suite=""
    for entry in "${FAILED_TESTS[@]}"; do
        suite_name="${entry%%|*}"
        test_label="${entry#*|}"
        if [ "$suite_name" != "$prev_suite" ]; then
            echo ""
            echo "    $suite_name:"
            prev_suite="$suite_name"
        fi
        echo "      ✗ $test_label"
    done
fi
echo "============================================"

exit "$ALL_PASS"
