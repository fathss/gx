# gx clean

Prune branches that are already merged into the base branch.

---

## Usage

```bash
gx clean [--remote] [--yes]
```

`gx clean` fetches with `--prune` from the configured remote so merge decisions reflect current remote history and stale remote-tracking refs are removed. It identifies branches fully merged into the base branch (`defaultBranch` in `.gx/config`) and prompts before deleting:

1. **Local branches & remote-tracking refs**: `git branch -d <branch>` and `git branch -d -r <remote>/<branch>`.
2. **Remote branches** (only with `--remote`): `git push <remote> --delete <branch>`.

Protected branches (`main`, `master`, `develop`), the default branch itself, the currently active branch, and branches checked out in any worktree are never deleted. Worktree exclusions are reported as a warning. Deleting a merged branch is safe and never loses commits.

---

## Flags

| Flag        | Shorthand | Description                                          |
| ----------- | --------- | ---------------------------------------------------- |
| `--remote`  |           | Also prompt to delete merged branches on the remote. |
| `--yes`     | `-y`      | Automatically answer yes to confirmation prompts.    |
| `--verbose` | `-v`      | Show underlying plumbing git commands.               |

Confirmation prompts default to `No`: pressing Enter, typing `n`, or sending EOF declines. Pass `--yes` for non-interactive scripting and CI environments.

---

## Examples

### Prune local merged branches

```bash
gx clean
# ▸ git fetch --prune origin
# Prune 1 merged branch(es) from this clone?
#   feature/old-login [y/N]: y
# ▸ git branch -d feature/old-login
# Deleted branch feature/old-login (was abc1234).
# Pruned 1 branches.
```

### Prune local and remote branches

```bash
gx clean --remote
# ▸ git fetch --prune origin
# Prune 2 merged branch(es) from this clone?
#   origin/feature/old-a
#   origin/feature/old-b [y/N]: y
# Delete 2 remote branch(es) on origin?
#   origin/feature/old-a
#   origin/feature/old-b [y/N]: y
# ▸ git branch -d -r origin/feature/old-a
# ▸ git branch -d -r origin/feature/old-b
# ▸ git push origin --delete feature/old-a
# ▸ git push origin --delete feature/old-b
# Pruned 4 branches.
```

### Non-interactive cleanup (CI / automation)

```bash
gx clean --yes
# ▸ git fetch --prune origin
# ▸ git branch -d feature/old-login
# Deleted branch feature/old-login (was abc1234).
# Pruned 1 branches.
```

---

## Edge Cases & Recovery

| Scenario                               | Behavior & Recovery                                                                                                                                                                   |
| -------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Not a Git repo                         | Blocked by pre-flight guard: `Not inside a Git repository.`                                                                                                                           |
| Missing configuration                  | Blocked by pre-flight guard: run `gx init` first.                                                                                                                                     |
| Dirty working tree                     | Allowed. Branch pruning does not affect uncommitted working tree modifications.                                                                                                       |
| Protected, current, or worktree branch | Protected branches (`protectedBranches`), the checked-out branch, and branches checked out in any worktree are excluded from deletion candidates. Worktree exclusions emit a warning. |
| No merged branches found               | Outputs `Pruned 0 branches.` without prompting. Exits with code 0.                                                                                                                    |
| Declining prompt                       | Entering `n` or hitting Enter leaves target branches untouched. Exits with code 0.                                                                                                    |
| Branch already deleted remotely        | `fetch --prune` removes the stale remote-tracking ref, so no remote deletion is attempted and the run exits 0.                                                                        |
| Concurrent remote deletion             | If a branch disappears between fetch and `push --delete`, the missing remote ref is treated as already pruned.                                                                        |
| Network or fetch failure               | Exits with error: `Failed to fetch from "<remote>".` No branches are deleted.                                                                                                         |
| Remote deletion error                  | If remote hook rejects deletion, git error is surfaced and remaining commits stay on base branch.                                                                                     |
