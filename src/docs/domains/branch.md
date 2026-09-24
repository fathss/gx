# Branch Domain

**File:** `internal/git/branch.go`

## HEADState

A struct holding the current HEAD position:
- `Branch` — current branch name, `""` when detached
- `ShortHash` — abbreviated hash (7 chars), `""` on error
- `IsDetached` — true when HEAD is not on a named branch

## GetHEADState

```
git branch --show-current   (for branch name)
git rev-parse --short HEAD  (for short hash)
```

Returns a `*HEADState` with the current branch name (or empty if detached)
and the abbreviated HEAD hash. Returns an error only if both commands fail.

Used by `gx status` for the branch/HEAD section.

## ShortHeadHash

```
git rev-parse --short HEAD
```

Returns the 7-character abbreviated hash of HEAD. Returns `""` if HEAD
cannot be resolved (empty repository with no commits).

## LocalBranchExists

```
git rev-parse --verify refs/heads/<branch>
```

Returns `true` if the given local branch exists.

## CurrentBranch

```
git branch --show-current
```

Returns the current checked-out branch name, or an empty string when HEAD is detached.

| Edge case | Behaviour |
|---|---|
| Detached HEAD | Returns empty string. Workflow decides whether to proceed or error. |

---

## Checkout

```
git checkout <branch>
```

Switch to an existing branch.

| Edge case | Behaviour |
|---|---|
| Branch doesn't exist | Fails. |
| Uncommitted changes conflict | Fails — checkout blocked by dirty working tree. |
| Merge conflict in progress | Fails. |

---

## MergedBranches

```
git branch --merged <base>
```

Returns local branches merged into base. Strips the `* ` marker from the
current branch entry and skips empty lines. Returns `nil` when no branches
are merged.

---

## MergedRemoteBranches

```
git branch -r --merged <remote>/<base>
```

Returns remote-tracking refs merged into the given base. Qualifies the base
with `<remote>/` when it is not already qualified (e.g. `develop` becomes
`origin/develop`). Returns `nil` when no refs are merged.

---

## DeleteLocalBranch

```
git branch -d <branch>
```

Deletes a local branch with safe delete (refuses when not fully merged).

| Edge case | Behaviour |
|---|---|
| Branch not fully merged | Fails — safe delete refuses. |

---

## DeleteRemoteTrackingBranch

```
git branch -d -r <remote>/<branch>
```

Deletes a remote-tracking ref. Normalizes the branch argument so a bare name
or an already-qualified ref both resolve to `<remote>/<branch>`.
