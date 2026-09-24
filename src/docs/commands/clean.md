# gx clean

Prune branches that are already merged into the base branch.

## Usage

```
gx clean [--remote]
```

`gx clean` fetches from the configured remote first so the merged-vs-not decision reflects current remote history rather than a stale local snapshot. It then deletes branches that are fully merged into the base branch (`defaultBranch` from `.gx/config`). Deleting a merged branch never loses commits: everything on a `--merged <base>` branch is reachable from `<base>`.

```bash
gx clean              # prune local + remote-tracking refs
gx clean --remote     # also delete remote branches (prompts)
```

---

## Flags

| Flag       | Purpose                                              |
| ---------- | ---------------------------------------------------- |
| `--remote` | Also delete remote branches (prompts for confirmation) |

The global `--verbose` / `-v` flag applies as usual to show plumbing commands.

---

## What it deletes (three scopes)

| Scope                | Default?          | Command                            | Affects              |
| -------------------- | ----------------- | ---------------------------------- | -------------------- |
| Local branches       | Always            | `git branch -d <branch>`           | Only your clone      |
| Remote-tracking refs | Always            | `git branch -d -r <remote>/<branch>` | Only your clone's view |
| Remote branches      | Only `--remote` + confirm | `git push <remote> --delete <branch>` | Everyone on the remote |

Without `--remote`, `gx clean` never mutates shared state on the remote.

---

## Candidate selection

Candidates come from the merged-into-base check. The merge-base target differs by scope because `git fetch <remote>` advances only `refs/remotes/<remote>/*`, never the local `defaultBranch`:

* **Local branches:** `git branch --merged <defaultBranch>` — checked against local `defaultBranch`.
* **Remote-tracking refs:** `git branch -r --merged <remote>/<defaultBranch>` — checked against freshly-fetched remote base.
* **Remote branches** (`--remote`): same as remote-tracking — against `<remote>/<defaultBranch>`. Using the local base for remote deletion would be unsafe: if local `defaultBranch` is ahead of the remote (unpushed local merges), a branch merged only via a local-only commit could be deleted from the remote while its commits are not reachable from the remote's actual base.

The filter removes (matching the **branch-name portion**, stripping `<remote>/` prefix for remote-tracking refs so `protectedBranches: ["main"]` matches `origin/main`):

* Protected branches (`protectedBranches` config, defaults to `main`, `master`, `develop`)
* The currently-checked-out branch
* The default branch itself (and its `<remote>/...` counterpart)
* The remote HEAD symbolic ref (`<remote>/HEAD` and `origin/HEAD -> origin/main` lines)

Remote deletion candidates come from the remote-tracking list independently, so a collaborator with no local branch can still prune the remote.

---

## Configuration

| Field              | Used for                  | Default                         |
| ------------------ | ------------------------- | ------------------------------- |
| `remote`           | Remote to fetch/push      | `origin`                        |
| `defaultBranch`    | Which branch to check against | `develop`                   |
| `protectedBranches`| Branches never deleted   | `["main","master","develop"]`   |

---

## Flow

### Pre-flight

Central guard checks apply (same as all gx commands):

* Not a git repository → `✗ Not inside a Git repository.`
* No `.gx/config` → `✗ No existing configuration` + `hint: Run gx init to start using gx`

`gx clean` requires both. A dirty working tree is **not** blocked — branch deletion does not conflict with uncommitted changes.

### Normal flow

```
1. git fetch <remote>
2. List local candidates: git branch --merged <defaultBranch>
3. List remote-tracking candidates: git branch -r --merged <remote>/<defaultBranch>
4. Filter out protected, current, default, and <remote>/HEAD
5. If --remote: prompt with exact remote branches to delete [y/N] (default N)
6. git branch -d <branch>             (each remaining local)
7. git branch -d -r <remote>/<branch> (each remaining remote-tracking)
8. git push <remote> --delete <branch> (each confirmed remote, if any)
9. Pruned N branches.  (or failure variant)
```

Remote deletion is gated: default off, requires affirmative `[y/N]` listing exact branches. If the user declines, local + remote-tracking pruning still happens and `gx clean` exits 0.

### Output

Git's own runner output already reports each delete (`Deleted branch ...`, `Deleted remote-tracking branch ...`). gx adds a single summary line:

* Success: `Pruned N branches.`
* Partial failure: `Pruned N branches — their commits remain on <base>.` — the "commits remain on base" clause appears only on failure, where the user is worried something was lost. The invariant is that every deleted branch was merged, so nothing is ever lost.

Every git command prints `▸ git <args>` followed by git's own stdout/stderr. Plumbing commands are hidden unless `--verbose`.

---

## Edge cases

| Condition | Behaviour |
| --------- | --------- |
| Protected branch (`main`/`master`/`develop`) | Never deleted (local or remote-tracking) |
| Current branch | Never deleted (local only) |
| Default branch itself | Never deleted (local and `<remote>/...`) |
| `<remote>/HEAD` symbolic ref | Never deleted |
| Unmerged branch | Not a candidate (`--merged` excludes it) — left intact |
| Nothing to prune | `Pruned 0 branches.` — exit 0 |
| Dirty working tree | Still runs — dirty files untouched |
| Outside git repo | Blocked: `Git repository` |
| No `.gx/config` | Blocked: `gx init` |
| `--remote` without confirming (n / empty) | Remote branches left intact — exit 0, summary counts only local/remote-tracking |
| `--remote` with confirming (y) | Remote branches deleted for everyone |
| Collaborator with no local branch | Still prunes remote-tracking (plain) and remote (`--remote` + confirm) via remote-tracking list |
| Remote base ahead vs local base ahead | Remote candidates use `<remote>/<defaultBranch>` — prevents deleting a branch merged only locally |
| Remote update hook rejects delete | Git error surfaced as-is; summary shows `their commits remain on <base>` |
| Fetch fails | `Failed to fetch from "<remote>".` |

---

## Examples

### Clean local clutter

```bash
gx clean
# ▸ git fetch origin
# ▸ git branch --merged develop
# ▸ git branch -d feature/old-login
# Deleted branch feature/old-login (was abc1234).
# Pruned 1 branches.
```

### Clean including remote

```bash
gx clean --remote
# ▸ git fetch origin
# ▸ git branch -d -r origin/feature/old-a
# ▸ git branch -d -r origin/feature/old-b
# Delete 2 remote branch(es) on origin?
#   origin/feature/old-a
#   origin/feature/old-b [y/N]: y
# ▸ git push origin --delete feature/old-a
# ▸ git push origin --delete feature/old-b
# Pruned 4 branches.
```

Here neither branch has a local copy — a collaborator who never checked them out still
prunes their remote-tracking refs and (on confirm) the branches on the remote: 2 + 2 = 4.


### Nothing to do

```bash
gx clean
# ▸ git fetch origin
# Pruned 0 branches.
```

---

## For developers

| File | Contents |
| ---- | -------- |
| `cmd/clean.go` | Cobra command, `--remote` flag, config loading |
| `internal/workflow/clean.go` | Orchestration: fetch → select → filter → prompt → delete → summarize; filter helper |
| `internal/git/branch.go` | `MergedBranches()`, `MergedRemoteBranches()`, `DeleteLocalBranch()`, `DeleteRemoteTrackingBranch()` |
| `internal/git/remote.go` | `Fetch()`, `DeleteRemoteBranch()` |
| `tests/gx-clean-test.sh` | Bash integration tests |
