---
name: gx-test-analyst
description: >
  Run a gx test script (bash) and produce a structured, class-aware pass/fail report. Understands the test classes emitted by gx-test-creator (SPEC, INVARIANT, EXPECTED-FAIL, CHARACTERIZATION), triages failures using the spec as the oracle (a failing SPEC/INVARIANT test means a suspected CODE bug, not a test to be "fixed"), and audits whether the suite itself is trustworthy. Trigger when the user says "run the test script", "run tests", "execute test suite", "test report", "run /path/to/test.sh", "check if tests pass", or asks to evaluate a test script's output. Do NOT use for code review, debugging the code itself, or running non-test scripts.
---

# gx-test-analyst

Run a test script and produce a structured report that separates:

1. **Did the code behave correctly?** (results, triaged by test class)
2. **Can we trust the tests that said so?** (suite-quality audit)

## Core principle: a red test is evidence about the code first

The creator skill writes tests from the spec, not from the implementation.
Therefore:

- A failing `SPEC` or `INVARIANT` test means **the code is presumed wrong**.
  Do NOT default to "the test assertion may need updating".
- Only conclude "test logic issue" when you can show the _test itself_
  contradicts the spec or is mechanically broken (see "Failure triage").
- Never recommend changing an assertion merely to make it match observed output.
  That is the tautology this pipeline exists to prevent.
- A **passing** suite is not automatically good news. Report how trustworthy
  the passes are (see "Suite-quality audit").

## Constraints

- **Do NOT modify any code.** This skill is analysis-only.
- Do not edit the test script, source files, docs, or configuration.
- Do not install dependencies or patch the environment.
- Do not run `git add`, `git commit`, or `git push`.
- If the script fails mid-way (e.g. `set -e` bails), that is part of the
  result. Report it.
- The static analysis in step 3 reads files only. It must not change anything.

## Test classes (emitted by gx-test-creator)

Each test function in the script carries a doc block:

```
# Contract: C01
# Class: SPEC | INVARIANT | EXPECTED-FAIL | CHARACTERIZATION
# Catches: <bug this test would detect>
```

| Class              | Meaning                                                                | Result meaning                                                         |
| ------------------ | ---------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| `SPEC`             | Derived from documented behavior.                                      | Pass = behaves per spec. **Fail = suspected code bug.**                |
| `INVARIANT`        | Cross-cutting safety property (no data loss, recoverable, idempotent). | Pass = property held. **Fail = serious code bug.**                     |
| `EXPECTED-FAIL`    | Creator found spec/code divergence; test asserts the spec.             | **Fail = confirmed known bug (expected).** Pass = surprise, see below. |
| `CHARACTERIZATION` | Spec silent; pins current behavior. Lowest confidence.                 | Pass = unchanged. Fail = behavior drifted; needs human judgment.       |

Tests with no class tag are `UNCLASSIFIED` (legacy or malformed). Treat as
`SPEC` for triage but flag the missing tag in the audit.

## Workflow

### 1. Locate the script

If the user gave a path, use it directly. If not, ask for it. Verify the file
exists and can be run via `bash <path>`.

### 2. Run the script

Run `bash <script-path> 2>&1` and capture the full stdout + stderr.

Run as-is. If the script has unmet dependencies (compiler, network, repo state),
run it anyway and report setup errors as failures with category `environment`.
Note the script's own exit code.

### 3. Static analysis (read-only, do this even if the run crashed)

Parse the script file itself. Run under `bash` (these use process substitution).

**Contract rows:**

```bash
grep -E '^#[[:space:]]+C[0-9]+[[:space:]]' <script>
```

**Test functions:**

```bash
grep -E '^test_[0-9]+_[A-Za-z0-9_]*\(\)' <script>
```

**Class / contract / catches per test function:**

```bash
awk '
  /^# Contract:/ { c=$3 }
  /^# Class:/    { k=$3 }
  /^# Catches:/  { sub(/^# Catches:[ ]*/,""); w=$0 }
  /^test_[0-9]+_[A-Za-z0-9_]*\(\)/ {
    n=$1; sub(/\(\).*/,"",n)
    printf "%-28s contract=%-5s class=%-17s catches=[%s]\n", n, c, k, w
    c=""; k=""; w=""
  }' <script>
```

**Class counts:**

```bash
grep -E '^# Class:' <script> | awk '{print $3}' | sort | uniq -c
```

**Contract IDs cited by tests but missing from the contract table:**

```bash
comm -13 \
  <(grep -E '^#[[:space:]]+C[0-9]+[[:space:]]' <script> | awk '{print $2}' | sort -u) \
  <(grep -E '^# Contract:' <script> | awk '{print $3}' | sort -u)
```

**SPEC/INVARIANT contract rows that have no test:**

```bash
comm -23 \
  <(grep -E '^#[[:space:]]+C[0-9]+[[:space:]]+(SPEC|INVARIANT)' <script> | awk '{print $2}' | sort -u) \
  <(grep -E '^# Contract:' <script> | awk '{print $3}' | sort -u)
```

**Tests with empty or `n/a` `# Catches:`:**

```bash
awk '
  /^# Catches:/  { sub(/^# Catches:[ ]*/,""); w=$0 }
  /^test_[0-9]+_/ { n=$1; sub(/\(\).*/,"",n); if (w=="" || tolower(w)=="n/a") print n; w="" }' <script>
```

**Tests that call `run_gx` without `snapshot_state`:**

```bash
awk '
  /^test_[0-9]+_/ { if (fn && used && !snap) print fn; fn=$1; sub(/\(\).*/,"",fn); used=0; snap=0 }
  /run_gx/ { used=1 }
  /snapshot_state/ { snap=1 }
  END { if (fn && used && !snap) print fn }' <script>
```

**Weak-assertion check** — flag tests whose only assertions are exit code and
message text (no state assertion). A test is **weak** if every assertion helper
it uses is in `{assert_exit_code, assert_exit_nonzero, assert_output_contains,
assert_output_not_contains}`:

```bash
awk '
  BEGIN { weak["assert_exit_code"]=1; weak["assert_exit_nonzero"]=1;
          weak["assert_output_contains"]=1; weak["assert_output_not_contains"]=1 }
  function flush() { if (fn && n>0 && !strong) print fn " WEAK"; else if (fn) print fn " ok" }
  /^test_[0-9]+_/ { flush(); fn=$1; sub(/\(\).*/,"",fn); n=0; strong=0 }
  match($0, /assert_[a-z_]+/) { h=substr($0,RSTART,RLENGTH); n++; if (!(h in weak)) strong=1 }
  END { flush() }' <script>
```

Output is one line per test: `<name> WEAK` or `<name> ok`.

**Crash detection** — compare functions found against results reported:

```bash
found=$(grep -cE '^test_[0-9]+_[A-Za-z0-9_]*\(\)' <script>)
ran=$(grep -cE '^##TEST_(PASS|FAIL):' <captured-output-file>)
[ "$ran" -lt "$found" ] && echo "INCOMPLETE RUN"
```

(Save the run output from step 2 to a temp file outside the repo, e.g. under
`/tmp`, to run this. Do not write into the project tree.)

If the script has no `# Class:` / `# Contract:` lines at all, say so. It predates
or ignores the creator conventions. Report every test as `UNCLASSIFIED` and
skip the contract-coverage checks.

### 4. Parse the run output

Extract:

- **Per-test results.** Prefer the `##TEST_PASS:` / `##TEST_FAIL:` markers
  emitted by `run_test`. Fall back to `✓` / `✗` lines and section headers.
- **Assertion-level results** for each test (`✓` / `✗` lines, with labels and
  any "expected/actual" detail printed).
- **Totals.** The suite's own summary line (test-level and assertion-level).
- **Crash detection.** If the number of tests that reported a result is fewer
  than the number of test functions found in step 3, the script died early
  (often `set -e`). Report which tests never ran.
  Join the two sources: for each test, attach its class, contract ID, and
  `Catches` line to its run result.

### 5. Triage every non-passing result

Handle by class.

**`SPEC` / `INVARIANT` failed**
Default verdict: **suspected code bug**. Report:

- the contract row and spec reference it came from,
- expected vs actual,
- the specific invariant or requirement violated,
- severity: `INVARIANT` failures involving data loss, lost commits, or
  unrecoverable repo state are **critical**.
  Reclassify as a **test issue** ONLY if you can point to concrete evidence:
- the assertion contradicts the spec text cited in its contract row, or
- the failure is mechanical (setup bug, wrong helper arguments, script error
  before the command under test ran, bad path), or
- the assertion is over-specific in a way the spec does not require
  (e.g. an exact full sentence where only a keyword is justified).
  State the evidence explicitly. If uncertain, keep the verdict as
  "suspected code bug, needs human confirmation" and say why.
  **`EXPECTED-FAIL` failed**
  Report as: **confirmed known bug**. This is the suite working as designed.
  Restate the divergence from the creator's tag/comment.

**`EXPECTED-FAIL` passed**
Report as: **unexpected pass**. Either the bug was fixed (the tag should be
promoted to `SPEC`) or the test does not actually exercise the divergence
(the test is too weak). Say which is more likely from its assertions.

**`CHARACTERIZATION` failed**
Report as: **behavior drift**. The test pins old behavior and the code has
changed. This needs human judgment; do not label it a bug or a test fix.

**`environment` failures**
Missing binary, missing network, permissions, missing helper in `lib.sh`, script
syntax error. Report as environment. State what is missing.

**Never** propose "change the assertion to match the actual output" as the fix
for a `SPEC` or `INVARIANT` failure.

### 6. Suite-quality audit

Regardless of pass/fail, assess whether the suite deserves trust. Report the
findings from step 3:

| Check                                            | Signal                                                                               |
| ------------------------------------------------ | ------------------------------------------------------------------------------------ |
| Class mix                                        | Mostly `CHARACTERIZATION` is a warning: the suite mirrors the code.                  |
| Zero `EXPECTED-FAIL` on a complex command        | Suspicious; either the code is perfect or the spec is too thin to expose divergence. |
| Contract IDs cited but not in the contract table | Broken traceability.                                                                 |
| `SPEC`/`INVARIANT` contract rows with no test    | Coverage gap.                                                                        |
| Tests with empty or `n/a` `# Catches:`           | Cannot name the bug they catch; likely tautological.                                 |
| Tests calling `run_gx` with no `snapshot_state`  | Invariants cannot be verified for that test.                                         |
| Weak tests (exit code + message only)            | Verifies wording, not behavior.                                                      |
| No contract block at all                         | Suite was not built spec-first.                                                      |
| `UNCLASSIFIED` tests                             | Legacy or malformed; trust is lower.                                                 |

Then give a single **Trust rating**:

- **High**: contract present, no orphan/empty items, mix dominated by
  `SPEC`/`INVARIANT`, few or no weak tests.
- **Medium**: minor gaps (a few weak tests or empty `Catches`).
- **Low**: no contract, mostly `CHARACTERIZATION`/`UNCLASSIFIED`, or many weak
  tests. A fully green result at Low trust means little.
  Base the rating on the audit facts only. State which facts drove it.

### 7. Report

Use the output template below. Rules:

- Lead with the **class-aware verdict**, then the per-test table, then details.
- Give totals at both test level and assertion level.
- If all tests passed, say so clearly, then state the trust rating. Do not
  imply that green means correct if trust is Medium or Low.
- Recommendations: findings only. Suggest what a human should investigate or
  decide. Do not write patches and do not tell anyone to edit files.
- Only suggest next steps when there are failures, `EXPECTED-FAIL` results,
  unexpected passes, drift, or trust findings. Otherwise stop after the report.

## Output template

```
## Verdict

<one-sentence class-aware summary, e.g.:
 "2 suspected code bugs (1 critical invariant violation), 1 confirmed known bug,
  1 unexpected pass. Suite trust: Medium.">

## Results: X/Y tests passed  (A/B assertions passed)

| # | Test | Class | Contract | Assertions | Result |
|---|------|-------|----------|-----------|--------|
| 01 | happy path | SPEC | C01 | 9/9 | ✅ Pass |
| 05 | merge conflict | SPEC | C06 | 4/6 | ❌ Suspected code bug |
| 08 | no commits lost | INVARIANT | C08 | 1/2 | 🔴 Critical |
| 09 | outside repo | EXPECTED-FAIL | C02 | 0/3 | ⚠️ Known bug (expected) |
| 11 | dirty tree | EXPECTED-FAIL | C05 | 5/5 | 🔔 Unexpected pass |
| 12 | detached HEAD | CHARACTERIZATION | - | 2/3 | 🔄 Behavior drift |

[If the script crashed:]
### Incomplete run
Ran N of M tests. Did not run: <list>. Cause: <e.g. `set -e` bailed in test 07>.

## Failures and findings

### Suspected code bugs (SPEC / INVARIANT failed)

**Test 05: merge conflict** (SPEC, contract C06, docs/commands/sync.md#conflicts)
- Spec says: <what the contract row requires>
- Expected: ...
- Actual: ...
- Violated: <requirement / invariant>
- Severity: normal | critical
- Verdict: suspected code bug | test issue (evidence: ...)

### Confirmed known bugs (EXPECTED-FAIL failed)
- Test 09: <divergence as documented by the creator>

### Unexpected passes (EXPECTED-FAIL passed)
- Test 11: <likely fixed vs. test too weak, and why>

### Behavior drift (CHARACTERIZATION failed)
- Test 12: <what changed>. Needs human decision.

### Environment issues
- <missing binary / helper / permission>

## Suite-quality audit

**Trust rating: High | Medium | Low**

| Check | Result |
|---|---|
| Class mix | SPEC 6, INVARIANT 3, EXPECTED-FAIL 2, CHARACTERIZATION 1 |
| Contract present | yes/no |
| Orphan contract IDs | none / C05 |
| Uncovered SPEC/INVARIANT rows | none / C07 |
| Empty `Catches` | none / test_03, test_04 |
| Missing `snapshot_state` | none / test_02 |
| Weak tests (no state assertion) | none / test_06, test_10 |

Rating driven by: <the 1-3 facts that decided it>.

## Items for human review
1. <e.g. "Confirm whether test 05's expectation matches the intended behavior
   in docs/commands/sync.md#conflicts">
2. <e.g. "Tests 03 and 04 cannot name the bug they catch; consider whether they
   add value">
```

If everything passed and trust is High, the report may be just the Verdict, the
Results table, and the audit table, with no "Failures" or "Items for human
review" sections.
