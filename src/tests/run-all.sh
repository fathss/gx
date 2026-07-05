#!/bin/bash
# run-all.sh — Run every *-test.sh suite in the tests/ directory
# Usage: bash tests/run-all.sh
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ALL_PASS=0
declare -a FAILED_SUITES
declare -a FAILED_TESTS

echo ""
echo "============================================"
echo "  gx — full test suite runner"
echo "============================================"
echo "  repo root: $REPO_ROOT"
echo ""

# Pre-build the binary so every suite doesn't recompile
echo "Building gx..."
cd "$REPO_ROOT" && go build -o gx . >/dev/null
echo "  ✓ gx built at $REPO_ROOT/gx"
trap 'rm -f "$REPO_ROOT/gx"' EXIT
echo ""

# Run each test suite in sequence
for suite in "$SCRIPT_DIR"/*-test.sh; do
    [ -f "$suite" ] || continue
    name=$(basename "$suite")

    echo "--------------------------------------------"
    echo "  Suite: $name"
    echo "--------------------------------------------"

    # Capture output to a temp file for marker parsing while streaming live
    suite_out=$(mktemp)
    bash "$suite" 2>&1 | tee "$suite_out"
    rc=${PIPESTATUS[0]}

    if [ "$rc" -eq 0 ]; then
        echo "  ✓ $name — all tests passed"
    else
        echo "  ✗ $name — some tests FAILED"
        FAILED_SUITES+=("$name")
        ALL_PASS=1

        # Parse ##TEST_FAIL markers to identify which individual tests failed
        failed_tests=$(grep '^##TEST_FAIL:' "$suite_out" | sed 's/^##TEST_FAIL://')
        if [ -n "$failed_tests" ]; then
            while IFS= read -r test_label; do
                echo "      ✗ $test_label"
                FAILED_TESTS+=("$name|$test_label")
            done <<< "$failed_tests"
        fi
    fi
    rm -f "$suite_out"
    echo ""
done

# Summary
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
