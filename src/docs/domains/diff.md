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

Returns `true` when there are staged changes in the index. Exits 0 if nothing
is staged — the error from the non-zero exit signals that changes exist.

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
