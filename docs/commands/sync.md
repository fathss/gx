# gx sync

Synchronize the current feature branch with the configured base branch.

---

## Usage

```bash
gx sync [<remote> [<baseBranch>]]
gx sync [--rebase | --merge]
gx sync --continue
gx sync --skip
gx sync --abort
```

`gx sync` automates the ritual of synchronizing feature branches:

1. Stashes uncommitted working tree changes with label `gx-sync/<branch>/<timestamp>` (only when the tree is dirty).
2. Fetches latest commits from `<remote>`, pruning remote-tracking refs whose remote branch is gone.
3. Fast-forwards the local `<baseBranch>` to match `<remote>/<baseBranch>`.
4. Records a `.git/gx-sync-state` ownership marker, then integrates updates into the current branch using either `rebase` (default) or `merge`.
5. Restores stashed changes cleanly (the marker is cleared), or keeps both marker and stash on a conflict pause.

For state classification and recovery invariants, see the [Sync State Machine](../concepts/sync-machine.md).

---

## Flags

| Flag              | Description                                                                    |
| ----------------- | ------------------------------------------------------------------------------ |
| `--continue`      | Resume a paused rebase or merge after conflicts have been resolved and staged. |
| `--skip`          | Skip the current commit during a paused rebase.                                |
| `--abort`         | Abort a paused rebase or merge and restore stashed working directory changes.  |
| `--rebase`        | Force rebase strategy for this run (overrides config).                         |
| `--merge`         | Force merge strategy for this run (overrides config).                          |
| `--verbose`, `-v` | Show underlying git plumbing commands.                                         |

`--rebase` and `--merge` are mutually exclusive. `--continue`, `--skip`, and `--abort` cannot be combined with strategy flags.

---

## Examples

### Standard feature branch sync

```bash
gx sync
# Stashing uncommitted changes...
# ▸ git fetch origin
# ▸ git checkout develop
# ▸ git merge --ff-only origin/develop
# ▸ git checkout feat/my-feature
# ▸ git rebase develop
# Restoring stashed changes...
```

### Sync directly on base branch (fast-forward only)

```bash
# While on develop branch
gx sync
# ▸ git fetch origin
# ▸ git merge --ff-only origin/develop
```

### Override remote and base branch for a single run

```bash
gx sync upstream main
```

### Conflict resolution workflow

```bash
# 1. Sync pauses on conflict
gx sync
# ✗ Rebase stopped because of conflicts while applying commit 'fix: login' (abc1234).
#   Conflicted files: src/auth.go
# hint: Resolve the conflicts, stage the files, then run gx sync --continue.

# 2. Resolve conflicts manually and stage
git add src/auth.go

# 3. Resume sync and restore stash
gx sync --continue
```

### Abort in-progress sync and restore state

```bash
gx sync --abort
# ▸ git rebase --abort
# Restoring stashed changes...
```

---

## Edge Cases & Recovery

| Scenario                       | Behavior & Recovery                                                                                                                                                                                                         |
| ------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Detached HEAD                  | Refused: `Detached HEAD is not supported.` Check out a branch first.                                                                                                                                                        |
| Conflicts encountered          | Sync pauses leaving rebase or merge in progress. The ownership marker is preserved, and the stash is preserved when one was created. Resolve conflicts, `git add`, and run `gx sync --continue`. Works for clean trees too. |
| Unresolved conflicts on resume | Refuses with: `There are still unresolved conflicts... Stage the files, then run gx sync --continue again.`                                                                                                                 |
| Manual rebase/merge detected   | If a rebase/merge is active without a gx ownership marker or branch-scoped `gx-sync/` stash, gx refuses to interfere. Use `git rebase` or `git merge` commands directly.                                                    |
| Orphaned gx stash              | If a previous sync was interrupted, gx restores the orphaned stash for the current branch before proceeding.                                                                                                                |
| Concurrent gx sync             | A per-worktree lock blocks the second run: `Another gx sync is already running...` No repository mutation occurs.                                                                                                           |
| Remote offline / fetch fails   | Error reported with hint to check network or `git remote -v`. Working tree is untouched.                                                                                                                                    |
| Base branch missing locally    | Fails with a hint to create the branch, e.g. `git checkout -b develop origin/develop`.                                                                                                                                      |
| Incompatible flags             | Passing conflicting flags (e.g. `--abort` and `--continue`, or `--rebase` and `--merge`) exits with an immediate usage error.                                                                                               |
