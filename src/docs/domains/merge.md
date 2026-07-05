# Merge Domain

**File:** `internal/git/merge.go`

## Merge

```
git merge --no-edit <branch>
```

Integrate the specified branch into the current branch. Uses `--no-edit`
to suppress the editor — the auto-generated merge message is used directly.

Used by the sync workflow when `syncStrategy: merge`.

| Edge case | Behaviour |
|---|---|
| Conflicts | Pauses — `IsMergeInProgress()` detects it, `MergeContinue()` resumes. |
| Already up to date | Git prints "Already up to date." and exits 0. |

---

## IsMergeInProgress

```
git rev-parse -q --verify MERGE_HEAD
```

Returns `true` when a merge is paused with conflicts. A paused merge creates a `MERGE_HEAD` reference.

Used in the Q5/Q6 state machine to detect paused merges and decide whether to block or auto-continue.

---

## MergeContinue

```
GIT_EDITOR=true git merge --continue
```

Completes a paused merge after all conflicts are resolved and staged.

`GIT_EDITOR=true` suppresses the editor so git reuses the auto-generated merge message automatically. Using `--no-edit` is invalid with `--continue`.

---

## MergeAbort

```
git merge --abort
```

Cancel a paused merge and restore the pre-merge state.

---

## ConflictedFiles

```
git diff --name-only --diff-filter=U
```

Returns the list of files with unresolved merge conflicts. Returned as
`[]string`, `nil` if none exist or on error. Works for conflicts from
both merge and rebase operations.

Used by the sync workflow to include the conflicted file list in error
messages when a rebase or merge is paused with conflicts.
