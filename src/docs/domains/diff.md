# Diff Domain

**File:** `internal/git/diff.go`

## DiffStatCached

```
git diff --stat --cached
```

Returns the diffstat of staged changes (e.g. `src/main.go | 5 +++++`).
Returns empty string if nothing is staged.

Used by `gx save` to show a summary of what will be committed.

| Edge case | Behaviour |
|---|---|---|
| Nothing staged | Returns empty string. |
| Large diff | Git truncates stat to terminal width. |
| Binary files | Shows binary size change instead of line counts. |

---

## HasStagedChanges

```
git diff --cached --quiet
```

```go
func HasStagedChanges(run runner.Executor) (bool, error)
```

Reports whether the index has staged changes. The underlying command's exit
code is the verdict:

| Exit code | Result |
|---|---|
| 0 | `false, nil` — index is clean |
| 1 | `true, nil` — differences exist |
| anything else (or non-ExitError) | `false, err` — a broken repository is never reported as "nothing staged" |

Used by `gx save` to check whether to warn about overwriting partial staging,
and by `gx save` conflict-marker detection.

---

## DiffUnstagedFiles

```
git diff --name-only
```

Returns the names of tracked files that have unstaged modifications, joined
by newlines. Returns empty string if nothing is unstaged.

Used by `gx save` to warn about overwriting partial staging.

---

## CheckCachedDiff

```
git diff --cached --check
```

Runs `git diff --cached --check` to detect conflict markers in staged files.
Returns combined output. Used by `gx save` before committing.
