# gx sync

Synchronize the current branch with its remote tracking branch and the configured base branch.

---

## Usage

```bash
gx sync [<remote> [<target_branch>]]
gx sync [--rebase | --merge]
gx sync --continue
gx sync --skip
gx sync --abort
```

`gx sync` automates the ritual of synchronizing branches:

1. Stashes uncommitted working tree changes with label `gx-sync/<branch>/<timestamp>` (only when the tree is dirty).
2. Fetches latest commits from `<remote>`, pruning remote-tracking refs whose remote branch is gone.
3. If on a feature branch (normal sync):
   - **Phase 1 (Remote Feature Branch)**: Reconciles with `<remote>/<current>`: fast-forwards if local is behind; rebases (or merges) if local and remote have diverged.
   - **Phase 2 (Base Branch)**: Fast-forwards local base branch and integrates it into the current branch using either `rebase` (default) or `merge`.
4. If on a base branch: Fast-forwards the base branch to match `<remote>/<baseBranch>`.
5. If `<target_branch>` is specified:
   - Blocks immediately if `<target_branch>` does not match the currently checked out branch.
   - If it matches, synchronizes *only* with `<remote>/<target_branch>` (skips base branch reconciliation).
6. Restores stashed changes cleanly (the marker is cleared), or keeps both marker and stash on a conflict pause.
7. Upon successful completion on a feature branch with commits ready to push, hints `Run gx ship to push.`

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
# Fast-forwarding feat/my-feature from origin/feat/my-feature...
# Fast-forwarding develop...
# Rebasing onto develop...
# Restoring stashed changes...
# hint: Run gx ship to push.
```

### Sync directly on base branch (fast-forward only)

```bash
# While on develop branch
gx sync
# ▸ git fetch origin
# ▸ git merge --ff-only origin/develop
```

### Sync only current branch with remote (skipping base branch)

```bash
# While on feat/login
gx sync origin feat/login
# ▸ git fetch origin
# Fast-forwarding feat/login from origin/feat/login...
```

### Accidental target branch specification is guarded

```bash
# While on feat/login
gx sync origin feat/dashboard
# ✗ You are currently on "feat/login", but specified "feat/dashboard".
# hint: Switch branches first:
# hint:   git checkout feat/dashboard
# hint:   gx sync
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
| Target branch mismatch         | If `<target_branch>` does not match the currently checked-out branch, refuses immediately with a hint to `git checkout <target_branch>`.                                                    |
| Incompatible flags             | Passing conflicting flags (e.g. `--abort` and `--continue`, `--rebase` and `--merge`, or `<target_branch>` with `--continue`/`--abort`) exits with an immediate usage error.                 |
