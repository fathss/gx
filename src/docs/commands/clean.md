# gx clean

Prune branches that are already merged into the base branch.

## Usage

```
gx clean [--remote] [--yes/--y]
```

`gx clean` fetches from the configured remote first so the merged-vs-not decision reflects current remote history rather than a stale local snapshot. It then lists the branches that are fully merged into the base branch (`defaultBranch` from `.gx/config`) and asks for confirmation before deleting them (skipped under `--yes`). Deleting a merged branch never loses commits: everything on a `--merged <base>` branch is reachable from `<base>`.

```bash
gx clean                  # prompt, then prune local + remote-tracking refs
gx clean --remote         # additionally prompt, then delete remote branches
gx clean --yes            # prune without prompting
gx clean --yes --remote   # delete remote branches without prompting
```

---

## Flags

| Flag           | Purpose                                              |
| -------------- | ---------------------------------------------------- |
| `--remote`     | Also offer to delete remote branches (second prompt) |
| `--yes` / `-y` | Skip both prompts and delete every candidate         |

Prompts list the exact refs at stake and default to **No**: an empty answer,
EOF, or any answer other than `y`/`yes` declines. `--yes` is the way to make
`gx clean` work non-interactively (pipes, CI, scripts); without it a
non-interactive run declines every prompt and prunes nothing.

`--yes` only affects prompting — the remote scope still requires `--remote`,
so `gx clean --yes` never touches the remote.

The global `--verbose` / `-v` flag applies as usual to show plumbing commands.

---

## What it deletes (three scopes)

| Scope                | Prompted?                            | Command                               | Affects                |
| -------------------- | ------------------------------------ | ------------------------------------- | ---------------------- |
| Local branches       | Yes (first prompt)                   | `git branch -d <branch>`              | Only your clone        |
| Remote-tracking refs | Yes (first prompt)                   | `git branch -d -r <remote>/<branch>`  | Only your clone's view |
| Remote branches      | Yes (second prompt, `--remote` only) | `git push <remote> --delete <branch>` | Everyone on the remote |

The first prompt covers both clone-local scopes together — declining it skips
local branches _and_ remote-tracking refs. The second prompt is independent:
declining it still allows the first scope to be pruned, and confirming it while
declining the first deletes only the remote branches (git then drops the matching
remote-tracking refs itself, as `push --delete` prunes them).

Without `--remote`, `gx clean` never mutates shared state on the remote.

---

## Candidate selection

Candidates come from the merged-into-base check. The merge-base target differs by scope because `git fetch <remote>` advances only `refs/remotes/<remote>/*`, never the local `defaultBranch`:

- **Local branches:** `git branch --merged <defaultBranch>` — checked against local `defaultBranch`.
- **Remote-tracking refs:** `git branch -r --merged <remote>/<defaultBranch>` — checked against freshly-fetched remote base.
- **Remote branches** (`--remote`): same as remote-tracking — against `<remote>/<defaultBranch>`. Using the local base for remote deletion would be unsafe: if local `defaultBranch` is ahead of the remote (unpushed local merges), a branch merged only via a local-only commit could be deleted from the remote while its commits are not reachable from the remote's actual base.

The filter removes (matching the **branch-name portion**, stripping `<remote>/` prefix for remote-tracking refs so `protectedBranches: ["main"]` matches `origin/main`):

- Protected branches (`protectedBranches` config, defaults to `main`, `master`, `develop`)
- The currently-checked-out branch
- The default branch itself (and its `<remote>/...` counterpart)
- The remote HEAD symbolic ref (`<remote>/HEAD` and `origin/HEAD -> origin/main` lines)

Remote deletion candidates come from the remote-tracking list independently, so a collaborator with no local branch can still prune the remote.

---

## Configuration

| Field               | Used for                      | Default                       |
| ------------------- | ----------------------------- | ----------------------------- |
| `remote`            | Remote to fetch/push          | `origin`                      |
| `defaultBranch`     | Which branch to check against | `develop`                     |
| `protectedBranches` | Branches never deleted        | `["main","master","develop"]` |

---

## Flow

### Pre-flight

Central guard checks apply (same as all gx commands):

- Not a git repository → `✗ Not inside a Git repository.`
- No `.gx/config` → `✗ No existing configuration` + `hint: Run gx init to start using gx`

`gx clean` requires both. A dirty working tree is **not** blocked — branch deletion does not conflict with uncommitted changes.

### Normal flow

```
1. git fetch <remote>
2. List local candidates: git branch --merged <defaultBranch>
3. List remote-tracking candidates: git branch -r --merged <remote>/<defaultBranch>
4. Filter out protected, current, default, and <remote>/HEAD
5. Prompt: prune the local candidates (branches + remote-tracking) [y/N] (default N)
6. If --remote: prompt with exact remote branches to delete [y/N] (default N)
   (steps 5-6 are skipped entirely under --yes, which answers both with yes)
7. git branch -d <branch>             (each remaining local, if confirmed)
8. git branch -d -r <remote>/<branch> (each remaining remote-tracking, if confirmed)
9. git push <remote> --delete <branch> (each confirmed remote, if any)
10. Pruned N branches.  (or failure variant)
```

Both prompts are collected **before** any deletion, so the first prompt always
runs before the repository is mutated. Each prompt defaults to No and lists the
exact refs: declining the first leaves local + remote-tracking refs untouched,
declining the second leaves remote branches untouched. Either way `gx clean`
exits 0.

### Output

Git's own runner output already reports each delete (`Deleted branch ...`, `Deleted remote-tracking branch ...`). gx adds a single summary line:

- Success: `Pruned N branches.`
- Partial failure: `Pruned N branches — their commits remain on <base>.` — the "commits remain on base" clause appears only on failure, where the user is worried something was lost. The invariant is that every deleted branch was merged, so nothing is ever lost.

Every git command prints `▸ git <args>` followed by git's own stdout/stderr. Plumbing commands are hidden unless `--verbose`.

---

## Edge cases

| Condition                                    | Behaviour                                                                                                                          |
| -------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| Protected branch (`main`/`master`/`develop`) | Never deleted (local or remote-tracking)                                                                                           |
| Current branch                               | Never deleted (local only)                                                                                                         |
| Default branch itself                        | Never deleted (local and `<remote>/...`)                                                                                           |
| `<remote>/HEAD` symbolic ref                 | Never deleted                                                                                                                      |
| Unmerged branch                              | Not a candidate (`--merged` excludes it) — left intact                                                                             |
| Nothing to prune                             | `Pruned 0 branches.` — no prompt is shown, exit 0                                                                                  |
| `--yes`                                      | No prompt is printed; every candidate is deleted                                                                                   |
| `--yes` with nothing to prune                | `Pruned 0 branches.` — exit 0                                                                                                      |
| No candidates in one scope                   | That scope's prompt is skipped entirely                                                                                            |
| Local prompt declined (n / empty)            | Local branches and remote-tracking refs both left intact — exit 0                                                                  |
| Local prompt confirmed (y)                   | Local branches and remote-tracking refs pruned                                                                                     |
| Dirty working tree                           | Still runs — dirty files untouched                                                                                                 |
| Outside git repo                             | Blocked: `Git repository`                                                                                                          |
| No `.gx/config`                              | Blocked: `gx init`                                                                                                                 |
| `--remote` without confirming (n / empty)    | Remote branches left intact — exit 0, summary counts only the confirmed local/remote-tracking work                                 |
| `--remote` with confirming (y)               | Remote branches deleted for everyone                                                                                               |
| `--yes` without `--remote`                   | Prompts skipped for local scope only; remote branches untouched                                                                    |
| Local declined but `--remote` confirmed      | Remote branches deleted; local branches kept (git's `push --delete` also drops their remote-tracking refs as a side effect)        |
| Collaborator with no local branch            | Still offers remote-tracking (first prompt) and, with `--remote`, the remote branches (second prompt) via the remote-tracking list |
| Remote base ahead vs local base ahead        | Remote candidates use `<remote>/<defaultBranch>` — prevents deleting a branch merged only locally                                  |
| Remote update hook rejects delete            | Git error surfaced as-is; summary shows `their commits remain on <base>`                                                           |
| Fetch fails                                  | `Failed to fetch from "<remote>".`                                                                                                 |

---

## Examples

### Clean local clutter

```bash
gx clean
# ▸ git fetch origin
# Prune 1 merged branch(es) from this clone?
#   feature/old-login [y/N]: y
# ▸ git branch -d feature/old-login
# Deleted branch feature/old-login (was abc1234).
# Pruned 1 branches.
```

### Clean including remote

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

Here neither branch has a local copy — a collaborator who never checked them out still
prunes their remote-tracking refs and (on confirm) the branches on the remote: 2 + 2 = 4.

### Nothing to do

```bash
gx clean
# ▸ git fetch origin
# Pruned 0 branches.
```

Nothing is shown here because there is nothing to confirm — the prompts only
appear when at least one ref is a candidate.

### Non-interactive (CI, scripts)

```bash
gx clean --yes
# ▸ git fetch origin
# ▸ git branch -d feature/old-login
# Deleted branch feature/old-login (was abc1234).
# Pruned 1 branches.
```

No `[y/N]` prompt is printed.

### Declining everything

```bash
gx clean
# ▸ git fetch origin
# Prune 1 merged branch(es) from this clone?
#   feature/old-login [y/N]:
# Pruned 0 branches.
```

---

## For developers

| File                         | Contents                                                                                                 |
| ---------------------------- | -------------------------------------------------------------------------------------------------------- |
| `cmd/clean.go`               | Cobra command, `--remote` / `--yes` flags, config loading                                                |
| `internal/workflow/clean.go` | Orchestration: fetch → select → filter → prompt (local, then remote) → delete → summarize; filter helper |
| `internal/git/branch.go`     | `MergedBranches()`, `MergedRemoteBranches()`, `DeleteLocalBranch()`, `DeleteRemoteTrackingBranch()`      |
| `internal/git/remote.go`     | `Fetch()`, `DeleteRemoteBranch()`                                                                        |
| `tests/gx-clean-test.sh`     | Bash integration tests                                                                                   |
