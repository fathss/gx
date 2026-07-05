# Rebase Domain

**File:** `internal/git/rebase.go`

## Rebase

```
git rebase <branch>
```

Replay the current branch's commits on top of `<branch>`.

| Edge case | Behaviour |
|---|---|
| Conflicts | Rebase pauses. `IsRebaseInProgress()` detects the paused state. |

---

## RebaseAbort

```
git rebase --abort
```

Cancel an in-progress rebase and return to the original branch.

---

## RebaseContinue

```
GIT_EDITOR=true git rebase --continue
```

Continue a paused rebase after conflicts are resolved and staged.

`GIT_EDITOR=true` suppresses the editor — the original commit message from
the replayed commit is reused automatically. Using `--no-edit` is invalid
with `--continue`.

---

## IsRebaseInProgress

```
os.Stat(".git/rebase-merge") or os.Stat(".git/rebase-apply")
```

Returns `true` when a rebase is active. Checks for `.git/rebase-merge`
(merge backend, default since git 2.26) or `.git/rebase-apply` (apply backend).

Used instead of `git rebase --show-current-patch` which returns
false-negatives during interactive rebase pauses, `--skip` on the last
patch, or `--stop`.

---

## RebaseSkip

```
GIT_EDITOR=true git rebase --skip
```

Skips the currently-applying commit during a rebase. `GIT_EDITOR=true` suppresses the editor prompt.

Used by `gx sync --skip` during a gx-orchestrated rebase.

---

## CurrentRebasePatchInfo

Parses `git rebase --show-current-patch` output to extract a human-readable
descriptor, e.g. `'Fix login bug' (abc1234)`.

Handles two git output formats:
- **Merge backend** (default since git 2.26): `commit <hash>` with indented subject
- **Apply backend** (older git): `From <hash>` with `Subject: [PATCH] <subject>`

Returns empty string when no rebase is in progress or parsing fails.

| Edge case | Behaviour |
|---|---|
| No rebase in progress | Empty string. |
| Unknown format during rebase | Falls back to hash only, or subject only, or empty. |


