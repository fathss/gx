---
name: docs-updater
description: >
  Scan the codebase, identify stale documentation that doesn't match the actual
  code, and update it. Creates a comprehensive diff of all doc changes. Use when
  the user asks to "update docs", "fix stale docs", "sync docs with code",
  "audit documentation", or similar requests about keeping docs in sync with the
  codebase.
---

# docs-updater

Scan the codebase, compare documentation against actual source code, and fix
any stale, inaccurate, or missing documentation.

## Constraints

- **Do not modify source code.** Only change documentation files.
- **Do not create new documentation** unless explicitly asked. Fix what exists.
- Preserve the doc's structure, tone, and level of detail — fix only what's wrong.
- Prefer `edit` tool for targeted changes over rewriting entire files.

## Workflow

### 1. Discover all documentation files

Use `glob` to find all `.md` files in the project:

```bash
globs: **/*.md, **/docs/**
```

Typically these live under:
- `docs/commands/` — per-command user documentation
- `docs/domains/` — per-git-domain developer documentation
- `docs/architecture.md` — architecture overview
- `AGENTS.md` — agent project rules (at project root or in `.agents/`)
- `gx.md` — project README/overview (if it exists)

Also read the `.agents/skills/` directory for existing skill files to understand the skill structure.

### 2. Read every source file referenced by the docs

To detect stale content, you need the actual code. Read these categories:

| What | Why |
|---|---|
| `cmd/*.go` | Cobra command definitions: flags, usage, annotations |
| `internal/workflow/*.go` | All `cli.Error` messages, flow logic, guard behavior |
| `internal/git/*.go` | All exported function signatures and behavior |
| `internal/config/config.go` | Config struct, defaults, Set/Get/Exists behavior |
| `internal/runner/runner.go` | Runner method signatures |
| `internal/cli/error.go` | `cli.Error` type |
| `main.go` | Error printing logic |
| `tests/*.sh` | Existing test files (to verify test-layout docs) |

### 3. Cross-reference docs against code — check these patterns

#### A. Flag and usage discrepancies

For each `docs/commands/<cmd>.md`, compare against `cmd/<cmd>.go`:

- Every flag in `cmd/<cmd>.go` must be documented in the command's doc.
- Every flag documented must exist in `cmd/<cmd>.go`.
- Usage lines must match (optional args, positional args).

**Red flag:** A flag defined in cobra but missing from the doc. Or vice versa.

#### B. Error message drift

For each `docs/commands/<cmd>.md` edge-case table, compare against `internal/workflow/<cmd>.go`:

- Every `cli.Error{Message: ...}` in the workflow should have a corresponding edge case in the doc (or at least be reflected in the flow description).
- The exact `Message` and `Hint` strings in the doc should match the code.
- Code paths that exit early (guards) should be documented edge cases.

#### C. Architecture layer mismatch

Compare `docs/architecture.md` against the actual file tree and signatures:

- **File tables** — every file that exists must be listed (or mentioned by pattern). Missing files = stale.
- **Function tables** — every exported function should appear.
- **Signature accuracy** — return types in example code snippets. `Runner.Run()` returns `error`, not `string`.
- **Config examples** — default values and JSON shapes.

#### D. Domain doc function signatures

Compare each `docs/domains/<domain>.md` against `internal/git/<domain>.go`:

- Every exported function should be documented.
- Parameters and return types in example code should match.
- Descriptions of behavior should match the implementation.
- "Hardcoded defaults" / "always active" claims are a common trap — check the actual code.

#### E. AGENTS.md / project rules

Common traps:
- Runner method return types (`string` vs `error`).
- Outdated config shapes or default values.
- Missing files or functions.
- Stale layer rules (e.g., `no_repo`/`no_config` annotations).

### 4. Apply targeted edits

For each discrepancy found:

1. **Quote the exact stale text** and the **exact replacement**.
2. Use `edit` tool with `oldString`/`newString` — preserve surrounding structure.
3. Prefer minimal edits over rewriting whole sections.
4. If a new feature flag or function was added to the code, add it to the doc's flag table or function list rather than restructuring the doc.

**Common fixes:**

- Add missing flags to flag tables.
- Add missing edge cases to edge-case tables.
- Add missing functions to architecture file tables.
- Fix return types in code snippets.
- Fix "always active" → "seeded by gx init" for sensitive patterns.
- Add missing developer reference entries (`| file.go | functions |`).
- Update test-file lists when new test files appear.

### 5. Verify

After all edits, do a final consistency check:

- `go vet ./...` passes (if any `.md` references code, the mental model should be consistent).
- Every doc change has a corresponding code justification you can explain.
- The docs now accurately describe the current codebase.

### 6. Summarize

Return a summary of all changes made:

```markdown
## Docs updated

| File | Changes |
|------|---------|
| `docs/commands/save.md` | Added `--exclude` flag, fixed sensitive patterns wording |
| `docs/commands/config.md` | Added `--overwrite` flag, variadic syntax |
| ... | ... |
```

For each change, note the **code discrepancy** that motivated it.
