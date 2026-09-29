# Status Domain

**File:** `internal/git/status.go`

## Overview

Status helpers used by `gx status` and the save workflow. Both structured
views (`GetParsedStatus`, `Status`) are projections of a single
`git status --porcelain` parse — `parsePorcelain` is the one classification
point, so the views can never disagree about what is staged, untracked, or
conflicted.

---

## StatusLabels

```go
var StatusLabels = map[string]string{
    "M": "modified",
    "A": "new file",
    "D": "deleted",
    "R": "renamed",
}
```

Maps porcelain status letters to human-readable labels.

---

## ParsedStatus

```go
type ParsedStatus struct {
    StagedFiles     map[string][]string // indexed by status letter (M, A, D, R)
    UnstagedFiles   []string
    UntrackedFiles  []string
    ConflictedFiles []string
    ModifiedFiles   []string // every non-untracked entry, in porcelain order (Status()'s view)
}
```

Structured representation of the repository status.

---

## parsePorcelain

Pure function over the output of:

```
git status --porcelain --untracked-files=all
```

Format: `XY<space>PATH`, or `XY<space>ORIG_PATH -> PATH` for renames — the
separator sits at index 2, so the leading status column (`X` may be a space)
survives the runner's right-trim and is always readable. `-uall` keeps
untracked directories expanded to individual files (matching what the old
`ls-files --others` query reported).

Classification per line:

| Condition | Bucket(s) |
|---|---|
| `??` | `UntrackedFiles` |
| `U` in either column, or `AA`/`DD` | `ConflictedFiles` + `ModifiedFiles` (never staged/unstaged) |
| `X` ∈ `StatusLabels` | `StagedFiles[X]` + `ModifiedFiles` |
| `Y` ∈ {`M`,`D`} | `UnstagedFiles` + `ModifiedFiles` |

Rename/copy lines keep only the destination: everything after the last
`" -> "`.

---

## GetParsedStatus

```go
func GetParsedStatus(run runner.Executor) (*ParsedStatus, error)
```

Runs the single porcelain query above and returns `parsePorcelain(out)` —
one subprocess instead of the four plumbing commands this function used to
issue (staged/unstaged/untracked/conflicted were each a separate git call
because `runner.Output` used to strip porcelain's leading status column).

---

## FormatStagedLabel

```go
func FormatStagedLabel(letter string) string
```

Returns the human-readable label for a porcelain status letter. Falls back to
the raw letter wrapped in `status <letter>` when no label is unknown.

---

## Status

```go
func Status(run runner.Executor) (modified, untracked []string, err error)
```

The save workflow's view of the same porcelain parse: `ModifiedFiles`
(every non-untracked entry, porcelain order, destination path for renames)
and `UntrackedFiles`.

---

## NonEmptyStatus

```
git status --porcelain --untracked-files=no
```

Returns `true` when there is any output for tracked files (staged or unstaged).
Untracked files are ignored.

---

## SubmodulePaths

```
git ls-files --stage
```

Scans the index for `160000` (gitlink) mode entries — format
`160000 <hash> 0\t<path>` — and returns a `map[path]bool` of submodule paths.
Used by the save workflow to skip submodules.

---

## ConflictedFiles

```
git diff --name-only --diff-filter=U
```

Independent conflict query used directly by the sync workflow's conflict
reporting (`sync.go`). The status views get their conflicts from the
porcelain parse instead.
