# Sync State Machine

`gx sync` operates as a deterministic state machine that inspects git repository state before executing actions. This guarantees that gx never corrupts or clobbers in-progress operations initiated manually by the user, and safely tracks its own stashed changes across paused operations.

---

## State Classification (Q-Table)

Before executing or resuming a sync, `gx` classifies repository state into one of six states (Q1–Q6) based on three signals:

| State  | Rebase Active? | Merge Active? | GX Stash Exists? | Description & Behavior                                                                        |
| ------ | -------------- | ------------- | ---------------- | --------------------------------------------------------------------------------------------- |
| **Q1** | No             | No            | No               | **Normal sync**: Proceed with full sync workflow.                                             |
| **Q2** | No             | No            | Yes              | **Orphaned stash**: Restore lingering `gx-sync/` stash, then proceed normally.                |
| **Q3** | Yes            | No            | No               | **Manual rebase**: Block. Rebase was started outside gx; user must use `git rebase` commands. |
| **Q4** | Yes            | No            | Yes              | **Paused gx rebase**: Block on plain sync. Accept `--continue`, `--skip`, or `--abort`.       |
| **Q5** | No             | Yes           | No               | **Manual merge**: Block. Merge was started outside gx; user must use `git merge` commands.    |
| **Q6** | No             | Yes           | Yes              | **Paused gx merge**: Block on plain sync. Accept `--continue` or `--abort`.                   |

### Detection Signals

1. **Rebase in Progress**:
   - Detected by presence of `.git/rebase-merge` or `.git/rebase-apply`.
   - Avoids false-negatives of `git rebase --show-current-patch` during interactive pauses.
2. **Merge in Progress**:
   - Detected via `git rev-parse -q --verify MERGE_HEAD`.
3. **GX Stash Existence**:
   - `git stash list` is inspected for labels starting with `gx-sync/` (or scoped to `gx-sync/<branch>/`).

---

## Stash Lifecycle & Isolation

The git stash stack is shared across all tools and user commands. Using fixed indices like `stash@{0}` is prone to index-shifting if other stashes are pushed or popped while a sync is paused.

gx avoids index-shifting bugs by:

1. Labeling each stash with a distinct prefix: `gx-sync/<branch>/<timestamp>`.
2. Locating stashes by prefix parsing rather than position index.
3. Scoping lookups to the current branch to avoid cross-branch stash conflicts.
4. Popping or dropping only matching entries via `stash@{N}` resolved from the search.

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
- The `gx-sync/` stash is **preserved**.
- gx outputs the conflicted commit information and files.
- State transitions to **Q4** (rebase) or **Q6** (merge).

### 3. Resuming Paused Operations (Q4 / Q6)

After resolving conflicts and staging files (`git add <files>`):

- `gx sync --continue`: Continues the in-progress rebase or merge (`GIT_EDITOR=true` suppresses message prompt). Once complete, automatically pops the preserved stash.
- `gx sync --skip` (rebase only): Skips the current commit and continues rebasing.
- `gx sync --abort`: Cancels the rebase (`git rebase --abort`) or merge (`git merge --abort`) and restores the working directory stash.

### 4. Non-Interference Policy (Q3 / Q5)

If a rebase or merge is in progress but no `gx-sync/` stash exists, gx determines the operation was started manually with `git rebase` or `git merge`. gx refuses to manipulate the operation to avoid clobbering user intent.
