#!/bin/bash
# gx UI & styling test suite — colors, NO_COLOR, and spinner
# Usage: bash tests/gx-ui-test.sh
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

setup_ui_test() {
    setup_tempdir
    mkdir -p "$TEST_DIR"
    GX_RUN_DIR="$TEST_DIR"
}

# ======================================================================
# TEST 01 — Color escape codes emitted when enabled
# ======================================================================
test_01_color_codes_emitted() {
    echo ""
    echo "=== Test 01: Color escape codes emitted when enabled ==="
    cleanup_tempdir
    setup_ui_test

    # Plain white: \033[37m ... \033[0m
    run_gx __ui --force-color color white "test-white"
    assert_exit_code "white exits 0" 0
    echo "$GX_OUTPUT" | grep -q $'\033\[37mtest-white\033\[0m'
    assert_exit_code "white has ANSI 37m" 0

    # Dim: \033[2m ... \033[0m
    run_gx __ui --force-color color dim "test-dim"
    assert_exit_code "dim exits 0" 0
    echo "$GX_OUTPUT" | grep -q $'\033\[2mtest-dim\033\[0m'
    assert_exit_code "dim has ANSI 2m" 0

    # Success (green): \033[32m ... \033[0m
    run_gx __ui --force-color color success "test-success"
    assert_exit_code "success exits 0" 0
    echo "$GX_OUTPUT" | grep -q $'\033\[32mtest-success\033\[0m'
    assert_exit_code "success has ANSI 32m" 0

    # Danger (red): \033[31m ... \033[0m
    run_gx __ui --force-color color danger "test-danger"
    assert_exit_code "danger exits 0" 0
    echo "$GX_OUTPUT" | grep -q $'\033\[31mtest-danger\033\[0m'
    assert_exit_code "danger has ANSI 31m" 0

    # Warning (yellow): \033[33m ... \033[0m
    run_gx __ui --force-color color warning "test-warning"
    assert_exit_code "warning exits 0" 0
    echo "$GX_OUTPUT" | grep -q $'\033\[33mtest-warning\033\[0m'
    assert_exit_code "warning has ANSI 33m" 0
}

# ======================================================================
# TEST 02 — Status glyph helpers
# ======================================================================
test_02_status_glyphs() {
    echo ""
    echo "=== Test 02: Status glyph helpers ==="
    cleanup_tempdir
    setup_ui_test

    # Checkmark
    run_gx __ui --force-color color checkmark ""
    assert_exit_code "checkmark exits 0" 0
    assert_output_contains "checkmark contains ✓" "✓"

    # Cross
    run_gx __ui --force-color color cross ""
    assert_exit_code "cross exits 0" 0
    assert_output_contains "cross contains failure glyph" "✗"

    # Warning symbol
    run_gx __ui --force-color color warningsymbol ""
    assert_exit_code "warningsymbol exits 0" 0
    assert_output_contains "warningsymbol contains ⚠" "⚠"
}

# ======================================================================
# TEST 03 — NO_COLOR environment variable disables color codes
# ======================================================================
test_03_no_color_suppression() {
    echo ""
    echo "=== Test 03: NO_COLOR environment variable disables color codes ==="
    cleanup_tempdir
    setup_ui_test

    # Standard NO_COLOR=1
    NO_COLOR=1 run_gx __ui color success "clean-text"
    assert_exit_code "NO_COLOR=1 exits 0" 0
    assert_output_contains "plain text preserved with NO_COLOR=1" "clean-text"
    if echo "$GX_OUTPUT" | grep -q $'\033\['; then
        echo "Failed: ANSI escape found when NO_COLOR=1"
        return 1
    fi
    assert_exit_code "no escape codes with NO_COLOR=1" 0

    # NO_COLOR with arbitrary string value
    NO_COLOR=true run_gx __ui color danger "error-text"
    assert_exit_code "NO_COLOR=true exits 0" 0
    assert_output_contains "plain text preserved with NO_COLOR=true" "error-text"
    if echo "$GX_OUTPUT" | grep -q $'\033\['; then
        echo "Failed: ANSI escape found when NO_COLOR=true"
        return 1
    fi
    assert_exit_code "no escape codes with NO_COLOR=true" 0
}

# ======================================================================
# TEST 04 — Non-TTY pipe / dumb terminal suppression
# ======================================================================
test_04_terminal_suppression() {
    echo ""
    echo "=== Test 04: Non-TTY pipe and dumb terminal suppression ==="
    cleanup_tempdir
    setup_ui_test

    # In subshell without TTY and without --force-color, colors should be disabled
    run_gx __ui color warning "pipe-text"
    assert_exit_code "pipe run exits 0" 0
    assert_output_contains "plain text preserved in pipe" "pipe-text"
    if echo "$GX_OUTPUT" | grep -q $'\033\['; then
        echo "Failed: ANSI escape found in non-TTY pipe"
        return 1
    fi
    assert_exit_code "non-TTY disables color escapes by default" 0

    # TERM=dumb disables color
    TERM=dumb run_gx __ui color white "dumb-text"
    assert_exit_code "TERM=dumb run exits 0" 0
    assert_output_contains "plain text preserved with TERM=dumb" "dumb-text"
    if echo "$GX_OUTPUT" | grep -q $'\033\['; then
        echo "Failed: ANSI escape found with TERM=dumb"
        return 1
    fi
    assert_exit_code "TERM=dumb disables color escapes" 0
}

# ======================================================================
# TEST 05 — Explicit disable flag overrides
# ======================================================================
test_05_explicit_flag_overrides() {
    echo ""
    echo "=== Test 05: Explicit flag overrides ==="
    cleanup_tempdir
    setup_ui_test

    # --disable-color suppresses output even without NO_COLOR
    run_gx __ui --disable-color color success "flag-disabled"
    assert_exit_code "--disable-color exits 0" 0
    assert_output_contains "plain text preserved with --disable-color" "flag-disabled"
    if echo "$GX_OUTPUT" | grep -q $'\033\['; then
        echo "Failed: ANSI escape found when --disable-color passed"
        return 1
    fi
    assert_exit_code "--disable-color suppresses escapes" 0
}

# ======================================================================
# TEST 06 — ASCII slash spinner frames
# ======================================================================
test_06_spinner_frames() {
    echo ""
    echo "=== Test 06: ASCII slash spinner frames ==="
    cleanup_tempdir
    setup_ui_test

    run_gx __ui frames
    assert_exit_code "frames command exits 0" 0
    assert_output_contains "contains slash sequence" '|,/,-,\'
}

# ======================================================================
# TEST 07 — Spinner tick lifecycle and cursor restoration
# ======================================================================
test_07_spinner_tick_and_cursor() {
    echo ""
    echo "=== Test 07: Spinner tick lifecycle and cursor restoration ==="
    cleanup_tempdir
    setup_ui_test

    run_gx __ui spinner tick
    assert_exit_code "spinner tick exits 0" 0

    # Must contain cursor hide: \033[?25l
    echo "$GX_OUTPUT" | grep -q $'\033\[?25l'
    assert_exit_code "spinner start hides cursor" 0

    # Must contain frames
    echo "$GX_OUTPUT" | grep -q "|"
    assert_exit_code "first tick rendered" 0
    echo "$GX_OUTPUT" | grep -q "/"
    assert_exit_code "second tick rendered" 0

    # Must contain cursor show / restore: \033[?25h
    echo "$GX_OUTPUT" | grep -q $'\033\[?25h'
    assert_exit_code "spinner stop restores cursor" 0
}

# ======================================================================
# TEST 08 — Spinner message updating
# ======================================================================
test_08_spinner_message_update() {
    echo ""
    echo "=== Test 08: Spinner message updating ==="
    cleanup_tempdir
    setup_ui_test

    run_gx __ui spinner update
    assert_exit_code "spinner update exits 0" 0
    assert_output_contains "shows initial message" "Initial message"
    assert_output_contains "shows updated message" "Updated message"
}

# ======================================================================
# TEST 09 — Spinner completion states: success with checkmark
# ======================================================================
test_09_spinner_completion_success() {
    echo ""
    echo "=== Test 09: Spinner completion success ==="
    cleanup_tempdir
    setup_ui_test

    run_gx __ui spinner success "Pushed branch"
    assert_exit_code "spinner success exits 0" 0
    assert_output_contains "checkmark glyph shown" "✓"
    assert_output_contains "success message shown" "Pushed branch"
}

# ======================================================================
# TEST 10 — Spinner completion states: failure with cross and warning hint
# ======================================================================
test_10_spinner_completion_failure() {
    echo ""
    echo "=== Test 10: Spinner completion failure with hint ==="
    cleanup_tempdir
    setup_ui_test

    run_gx __ui spinner fail "Sync failed" "run git status to inspect conflicts"
    assert_exit_code "spinner fail exits 0" 0
    assert_output_contains "failure glyph shown" "✗"
    assert_output_contains "failure message shown" "Sync failed"
    assert_output_contains "warning hint shown" "hint: run git status to inspect conflicts"
}

# ======================================================================
# RUN ALL
# ======================================================================
cleanup_tempdir
setup_ui_test
build_gx
trap cleanup_tempdir EXIT

echo "============================================"
echo "  gx UI & spinner — test suite"
echo "============================================"
echo ""

run_test test_01_color_codes_emitted "Test 01: Color escape codes emitted when enabled"
run_test test_02_status_glyphs "Test 02: Status glyph helpers"
run_test test_03_no_color_suppression "Test 03: NO_COLOR environment variable disables color codes"
run_test test_04_terminal_suppression "Test 04: Non-TTY pipe and dumb terminal suppression"
run_test test_05_explicit_flag_overrides "Test 05: Explicit flag overrides"
run_test test_06_spinner_frames "Test 06: ASCII slash spinner frames"
run_test test_07_spinner_tick_and_cursor "Test 07: Spinner tick lifecycle and cursor restoration"
run_test test_08_spinner_message_update "Test 08: Spinner message updating"
run_test test_09_spinner_completion_success "Test 09: Spinner completion success"
run_test test_10_spinner_completion_failure "Test 10: Spinner completion failure with hint"

echo ""
print_test_summary

cleanup_tempdir

[ "$TEST_FAIL" -eq 0 ]
