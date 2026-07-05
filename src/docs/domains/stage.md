# Stage Domain

**File:** `internal/git/stage.go`

## AddAll

```
git add -A
```

Stage all changes in the working tree — modified tracked files, new files,
and deleted files. Equivalent to `git add --all`.

## Add

```
git add <files...>
```

Stage the specified files. If no files are given, stages everything
(identical to `git add -A` in practice, but via the `-A` flag in AddAll).

Used by `gx save` when specific file arguments are provided.

## AddPatch

```
git add -p [<files>...]
```

Run interactive hunk selection. If files are given, only hunks in those
files are shown. User decides per-hunk whether to stage (y/n/s/e/q).

Used by `gx save --patch`.

| Edge case | Behaviour |
|---|---|
| No files given | Shows hunks from all changed files. |
| Specific files given | Only hunks from those files are shown. |
| No hunks to select | `git add -p` exits silently, nothing staged. |
