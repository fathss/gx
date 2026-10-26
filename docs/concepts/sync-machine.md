# Sync State Machine

`gx sync` operates as a deterministic state machine that inspects git repository state before executing actions. This guarantees that gx never corrupts or clobbers in-progress operations initiated manually by the user, and safely tracks its own stashed changes across paused operations.

---

## State Classification (Q-Table)

Before executing or resuming a sync, `gx` classifies repository state into one of six states (Q1–Q6) based on three signals:

| State  | Rebase Active? | Merge Active? | GX Owned? | Description & Behavior                                                                        |
| ------ | -------------- | ------------- | --------- | --------------------------------------------------------------------------------------------- |
| **Q1** | No             | No            | No        | **Normal sync**: Proceed with full sync workflow.                                             |
| **Q2** | No             | No            | Yes       | **Orphaned state**: Restore a lingering gx stash / clear a stale marker, then proceed.        |
| **Q3** | Yes            | No            | No        | **Manual rebase**: Block. Rebase was started outside gx; user must use `git rebase` commands. |
| **Q4** | Yes            | No            | Yes       | **Paused gx rebase**: Block on plain sync. Accept `--continue`, `--skip`, or `--abort`.       |
| **Q5** | No             | Yes           | No        | **Manual merge**: Block. Merge was started outside gx; user must use `git merge` commands.    |
| **Q6** | No             | Yes           | Yes       | **Paused gx merge**: Block on plain sync. Accept `--continue` or `--abort`.                   |

### Detection Signals

1. **Rebase in Progress**:
   - Detected by presence of `.git/rebase-merge` or `.git/rebase-apply`.
   - Avoids false-negatives of `git rebase --show-current-patch` during interactive pauses.
2. **Merge in Progress**:
   - Detected via `git rev-parse -q --verify MERGE_HEAD`.
3. **GX Ownership (marker, then stash fallback)**:
   - `.git/gx-sync-state` is written immediately before gx starts a rebase or merge and contains the owning branch. It is preserved on a conflict pause and cleared on every other settle. This is the authoritative signal, and it is what makes a conflict from a **clean** tree (no stash created) resumable.
   - For operations paused by an older gx (no marker), a branch-scoped `gx-sync/<branch>/<timestamp>` stash is accepted as the ownership signal.

While HEAD is detached during a rebase, the owning branch comes from the marker (or `.git/rebase-merge/head-name` for legacy state) — never from `git branch --show-current`, which is empty.

---

## Stash Lifecycle & Isolation

The git stash stack is shared across all tools and user commands. Using fixed indices like `stash@{0}` is prone to index-shifting if other stashes are pushed or popped while a sync is paused.

gx avoids index-shifting bugs by:

1. Labeling each stash with a distinct prefix: `gx-sync/<branch>/<timestamp>`.
2. Locating stashes by parsing labels rather than position index.
3. Matching the **exact** branch segment (the suffix must be the numeric timestamp), so `feature/a` never matches `feature/a/b`'s stash.
4. Popping or dropping only matching entries via `stash@{N}` resolved from the search.
5. Scoping cleanup to the owning branch, so another branch's orphaned stash is never applied or dropped.

If the tree was clean, no stash exists; the ownership marker alone carries the branch across the pause. If a stale marker names a different branch (for example after aborting a gx rebase with `git rebase --abort` and switching branches), gx clears only the marker and leaves that branch's stash untouched.

---

## Action Workflows

### 1. Normal Flow (Q1 / Q2)

```
1. Verify working tree and ensure HEAD is not detached
2. Stash dirty working tree (gx-sync/<branch>/<timestamp>) if uncommitted changes exist
3. git fetch <remote>
4. If current branch == defaultBranch:
     git merge --ff-only <remote>/<defaultBranch>
     Restore stash (if created)
5. If on feature branch:
     git checkout <defaultBranch>
     git merge --ff-only <remote>/<defaultBranch>
     git checkout <currentBranch>
     Apply strategy: git rebase <defaultBranch> OR git merge --no-edit <defaultBranch>
     Restore stash (if created)
```

### 2. Conflict Handling

When a conflict occurs during rebase or merge:

- Rebase or merge is **left in progress**.
- The ownership marker is **preserved**.
- The `gx-sync/` stash is **preserved** when one was created (dirty tree).
- gx outputs the conflicted commit information and files.
- State transitions to **Q4** (rebase) or **Q6** (merge).

This holds for a clean tree too: no stash is created, but the marker still makes the paused operation gx-owned and resumable.

### 3. Resuming Paused Operations (Q4 / Q6)

After resolving conflicts and staging files (`git add <files>`):

- `gx sync --continue`: Continues the in-progress rebase or merge (`GIT_EDITOR=true` suppresses message prompt). Once complete, automatically pops the preserved stash.
- `gx sync --skip` (rebase only): Skips the current commit and continues rebasing.
- `gx sync --abort`: Cancels the rebase (`git rebase --abort`) or merge (`git merge --abort`) and restores the working directory stash.

### 4. Non-Interference Policy (Q3 / Q5)

If a rebase or merge is in progress but there is no ownership marker and no branch-scoped `gx-sync/` stash, gx determines the operation was started manually with `git rebase` or `git merge`. gx refuses to manipulate the operation to avoid clobbering user intent.
