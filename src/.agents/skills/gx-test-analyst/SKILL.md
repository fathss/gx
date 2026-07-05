---
name: gx-test-analyst
description: >
  Run a test script (bash, shell) and produce a structured pass/fail report.
  Trigger when the user says "run the test script", "run tests", "execute test suite",
  "test report", "run /path/to/test.sh", "check if tests pass", or asks to evaluate
  a test script's output. Do NOT use for code review, debugging the code itself,
  or running non-test scripts.
---

# gx-test-analyst

Run a test script and produce a structured pass/fail report with suggestions.

## Constraints

- **Do NOT modify any code.** This skill is analysis-only.
- Do not edit the test script, source files, or configuration.
- Do not install dependencies or patch the environment.
- If the script fails mid-way (e.g. `set -e` bails), that is part of the result — report it.

## Workflow

### 1. Locate the script

If the user gave a path, use it directly. If they didn't, ask for the path to the test script. Verify the file exists and is executable (or can be run via `bash <path>`).

### 2. Run the script

Run `bash <script-path> 2>&1` and capture the full stdout + stderr.

If the script has dependencies (e.g. requires a compiled binary, a repo to exist, etc.), run it as-is and report any setup errors as failures.

### 3. Parse the output

Extract from the output:

- **Per-test results** — for each named test or section, whether it passed (✓) or failed (✗).
- **Assertion-level granularity** — if the script reports individual assertions (`✓` / `✗`), enumerate them per test.
- **Totals** — `X passed, Y failed`, or equivalent summary line.
- **Error details** — for any failing assertion, capture the error message.

### 4. Report

Present results in a table or bulleted list organized by test section:

- Title of the test section
- Assertions within that section (passed / failed count)
- Overall pass/fail status for the section

Then provide:

**Summary line** — e.g. "31 passed, 0 failed"

**If all passed:** state that clearly.

**If any failed:**
- List each failure with the error message
- Categorize: test logic issue vs. code under test issue vs. environment issue
- Suggest specific changes (without making them) — e.g. "The `branch missing` test expects the output to contain 'Failed to checkout' but the actual message was 'error: pathspec'. The test assertion string may need updating."

### 5. Prohibitions

- Do not write, patch, or suggest edits to any file.
- Do not run `git add`, `git commit`, or push.
- Do not suggest what the user should do next unless failures were found.

## Output template

```
## Results: X passed, Y failed

| Test | Assertions | Status |
|------|-----------|--------|
| **01** — test name | N/N | ✅ Passed / ❌ Failed |

[If failures:]
### Failures

**Test N — assertion name**
- Expected: ...
- Actual: ...
- Suggested fix: ...

### Suggestions
1. ...
```
