# Status Domain

**File:** `internal/git/status.go`

## Overview

Status helpers used by `gx status` and the save workflow. Plumbing-level
queries (`GetParsedStatus`, `Status`) deliberately avoid `git status --porcelain`
as the source for structured parsing — see **Why separate commands** below.

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
}
```

Structured representation of the repository status.

---

## GetParsedStatus

```go
func GetParsedStatus(run *runner.Runner) (*ParsedStatus, error)
```

Returns a structured parse built from **four independent plumbing commands**:

| # | Category | Command |
|---|----------|---------|
| 1 | Staged | `git diff --cached --diff-filter=ACDMR --name-status` |
| 2 | Unstaged | `git diff --name-only` |
| 3 | Untracked | `git ls-files --others --exclude-standard` (via `UntrackedFiles`) |
| 4 | Conflicted | `git diff --name-only --diff-filter=U` (via `ConflictedFiles`) |

Staged output is tab-separated: `<status>\t<path>`, or `<status>\t<old>\t<new>`
for renames. The **last tab field is the destination path** — rename lines are
handled by taking `parts[len(parts)-1]`. Lines whose status letter isn't in
`StatusLabels` are skipped.

**Why separate commands:** `runner.Output` trims the leading whitespace of the
_full_ captured output, which strips the leading space of the first
`git status --porcelain` line — corrupting its status code (`X=' '` for
"unmodified in index"). Each plumbing command above is leading-space-free by
construction, so the parse is safe.

---

## FormatStagedLabel

```go
func FormatStagedLabel(letter string) string
```

Returns the human-readable label for a porcelain status letter. Falls back to
the raw letter wrapped in `status <letter>` when no label is known.

---

## Status

```
git status --porcelain
```

Parses porcelain v1 format — `XY<space>PATH`, or `XY<space>ORIG_PATH -> PATH`
for renames/copies. Returns `(modified, untracked []string, err)`.

- The path is always read from `line[2:]` with `strings.TrimSpace` — the
  separator space is naturally absorbed whether `runner.Output` trimmed the
  leading space of the first line or not.
- Rename/copy lines (`R`/`C`) keep only the destination: everything after the
  last `" -> "`.
- A status prefix of `"?"` routes the path to `untracked`; everything else to
  `modified`.

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

## UntrackedFiles

```
git ls-files --others --exclude-standard
```

Returns the list of untracked files that are not gitignored. Returns `nil`
when none exist.