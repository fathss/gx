# gx clean

Prune branches that are already merged into the base branch.

---

## Usage

```bash
gx clean [--remote] [--yes]
```

`gx clean` fetches from the configured remote so merge decisions reflect current remote history. It identifies branches fully merged into the base branch (`defaultBranch` in `.gx/config`) and prompts before deleting:

1. **Local branches & remote-tracking refs**: `git branch -d <branch>` and `git branch -d -r <remote>/<branch>`.
2. **Remote branches** (only with `--remote`): `git push <remote> --delete <branch>`.

Protected branches (`main`, `master`, `develop`), the default branch itself, and the currently active branch are never deleted. Deleting a merged branch is safe and never loses commits.

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
# ▸ git fetch origin
# Prune 1 merged branch(es) from this clone?
#   feature/old-login [y/N]: y
# ▸ git branch -d feature/old-login
# Deleted branch feature/old-login (was abc1234).
# Pruned 1 branches.
```

### Prune local and remote branches

```bash
gx clean --remote
# ▸ git fetch origin
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
# ▸ git fetch origin
# ▸ git branch -d feature/old-login
# Deleted branch feature/old-login (was abc1234).
# Pruned 1 branches.
```

---

## Edge Cases & Recovery

| Scenario                    | Behavior & Recovery                                                                                                      |
| --------------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| Not a Git repo              | Blocked by pre-flight guard: `Not inside a Git repository.`                                                              |
| Missing configuration       | Blocked by pre-flight guard: run `gx init` first.                                                                        |
| Dirty working tree          | Allowed. Branch pruning does not affect uncommitted working tree modifications.                                          |
| Protected or current branch | Protected branches (`protectedBranches`) and the checked-out branch are automatically excluded from deletion candidates. |
| No merged branches found    | Outputs `Pruned 0 branches.` without prompting. Exits with code 0.                                                       |
| Declining prompt            | Entering `n` or hitting Enter leaves target branches untouched. Exits with code 0.                                       |
| Network or fetch failure    | Exits with error: `Failed to fetch from "<remote>".` No branches are deleted.                                            |
| Remote deletion error       | If remote hook rejects deletion, git error is surfaced and remaining commits stay on base branch.                        |
