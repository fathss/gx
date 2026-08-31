# Stash Domain

**File:** `internal/git/stash.go`

## Overview

Stash operations for gx-orchestrated pauses. Every command that stashes uses
its own label prefix via `SyncStashPrefix`, and all matching (pop, drop,
presence checks) is done **by label prefix** — never by `stash@{0}`.

**Why matching by prefix instead of `stash@{0}`:** the stash stack is a shared
resource. When the user runs `git stash push`/`git stash pop` themselves, the
indexes shift — `stash@{0}` would point at the user's newest stash, not at
gx's entry. Matching by the `gx-sync/` message prefix always finds the right
entry regardless of stack movement.

**Why it matters for the state machine:** the presence of a `gx-sync/` stash is
the signal distinguishing gx-orchestrated pauses from manual ones — a paused
rebase/merge **with** a gx stash is gx's own work and can be auto-continued; one
**without** it is a manual pause and gx blocks (see the sync state machine in
`docs/architecture.md`).

---

## SyncStashPrefix

```go
const SyncStashPrefix = "gx-sync/"
```

Label prefix used for all gx-created stashes. Per-command prefixes keep
`HasGXStash` scoped to stashes created by that command.

---

## StashList

```
git stash list
```

Returns all stash entries as lines. Returns `nil` if there are no stashes or
on error.

---

## StashPush

```
git stash push -m <msg>
```

Stashes tracked-file changes with a descriptive message label.

---

## StashPop

Finds the stash matching `SyncStashPrefix` via `findStashByPrefix` and pops it:

```
git stash pop <ref>
```

Pops the correct entry even when the user has pushed or popped other stashes
in between. Returns `nil` if no matching stash is found — the entry was
already dealt with.

---

## findStashByPrefix

```
git stash list   (parsed)
```

Returns the stash reference (e.g. `stash@{2}`) of the most recent stash entry
whose message starts with `prefix`. Returns `""` if none found.

Parses each line as `"stash@{N}: ...: <message>"` — split on `": "` into 3
parts; the reference is the first part, the message the last.

---

## DropGXStash

Loops `findStashByPrefix` + `stash drop` until no `gx-sync/` stash remains:

```
git stash drop <ref>
```

Used for cleanup of orphaned gx entries.

---

## IsClean

```
git status --porcelain --untracked-files=no
```

Returns `true` when no tracked files have staged or unstaged changes. Untracked
files are ignored — they don't block checkout or rebase.

---

## HasGXStash

Returns `true` if any stash entry was created by gx sync (label starts with
`SyncStashPrefix`). Used to distinguish gx-orchestrated rebases/merges from
manual ones — see the sync state machine in `docs/architecture.md`.