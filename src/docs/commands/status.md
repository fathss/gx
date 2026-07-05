# gx status

Print a human-readable snapshot of the repo's current state.

## Usage

```
gx status
```

`gx status` has no mutating behavior and no flags of its own — it only
reads. Per the root spec it reports: branch name, staged files, unstaged
files, stash count, ahead/behind count, and the last 5 commits.

```bash
gx status
```

---

## Flags

None beyond the global flag `--verbose` / `-v`, which controls whether the
underlying plumbing commands are echoed.

---

## Flow

### Pre-flight

Central guard checks apply (not a Git repo, no `.gx/config`). Unlike `save`
or `ship`, `gx status` does **not** block on an in-progress rebase or
merge — that's exactly the kind of state a developer runs `gx status` to
investigate. Instead, it surfaces it as the first line of output.

Detached HEAD is allowed and shown explicitly rather than treated as an
error.

### Sections

`gx status` gathers each section independently and prints them in a fixed
order, even if one section is empty:

**1. Branch / HEAD state**

```
On branch feat/login
```

or, if mid-rebase/merge/detached:

```
On branch feat/login (rebase in progress)
HEAD detached at a1b2c3d
```

**2. Staged files**

Tracked changes already in the index, grouped by status (`M`/`A`/`D`/`R`):

```
Staged:
  modified:  src/auth.go
  new file:  src/login.go
```

If nothing is staged:

```
Staged:
  (none)
```

**3. Unstaged files**

Modified-but-not-staged tracked files, plus untracked files, shown in
separate sub-groups (matching `gx save`'s categorization, for consistency
across commands):

```
Unstaged:
  modified:  src/main.go
Untracked:
  notes.md
```

Either sub-group is omitted (not printed as empty) if there's nothing in
it, to keep the common case compact.

**4. Stash count**

```
Stashes: 2
```

or, if empty:

```
Stashes: (none)
```

This is a count only — `gx status` doesn't list stash contents; that's
`gx stash list`'s job.

**5. Ahead/behind**

Compares the current branch against its upstream, if one is configured:

```
Ahead/behind: 2 ahead, 1 behind origin/feat/login
```

Variants:

```
Ahead/behind: up to date with origin/feat/login
Ahead/behind: 3 ahead of origin/feat/login
Ahead/behind: 1 behind origin/feat/login
Ahead/behind: (no upstream configured)
```

If there's an upstream configured but the remote-tracking ref is stale
(no fetch has happened recently), `gx status` does **not** fetch on its
own — it reports against whatever the local remote-tracking ref currently
holds, and notes this:

```
Ahead/behind: 2 ahead, 1 behind origin/feat/login (may be stale — last fetched 3h ago)
```

**6. Recent commits**

Last 5 commits on the current branch, one line each, same format as
`gx log`'s default (relative date, short hash, subject):

```
Recent commits:
  a1b2c3d  (2 hours ago)  fix: login redirect loop
  9f8e7d6  (1 day ago)    feat: add login page
  3c4d5e6  (2 days ago)   chore: bump deps
  7a8b9c0  (3 days ago)   test: cover auth middleware
  0d1e2f3  (4 days ago)   Initial commit
```

If the branch has fewer than 5 commits (e.g. right after `gx start`), only
the available commits are shown — no padding or placeholder lines.

---

## Edge cases

| Condition                               | Behaviour                                                                                                       |
| --------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| Not a Git repository                    | Central guard blocks: "Not inside a Git repository."                                                            |
| No `.gx/config`                         | Central guard blocks: "run gx init to start using gx"                                                           |
| Rebase in progress                      | Shown inline in branch line, not blocked.                                                                       |
| Merge in progress                       | Shown inline in branch line, not blocked.                                                                       |
| Detached HEAD                           | Shown as "HEAD detached at <hash>", not blocked.                                                                |
| Nothing staged                          | "Staged: (none)"                                                                                                |
| Nothing unstaged, no untracked files    | "Unstaged:" section and "Untracked:" section both omitted entirely.                                             |
| No stashes                              | "Stashes: (none)"                                                                                               |
| No upstream configured                  | "Ahead/behind: (no upstream configured)"                                                                        |
| Upstream configured, fully in sync      | "Ahead/behind: up to date with <remote>/<branch>"                                                               |
| Fewer than 5 commits on branch          | Prints only what exists, no padding.                                                                            |
| Empty repo (no commits at all)          | "Recent commits: (none — no commits yet)"                                                                       |
| `gx status` run mid-conflict (unmerged) | Staged/unstaged sections show `both modified:` entries under a third `Conflicted:` group, listed before Staged. |

---

## User-visible output

`gx status` is read-only, so unlike `save`/`ship` it does not print
`▸ git <args>` lines by default — the whole point is a clean, synthesized
summary. Pass `--verbose` to additionally show the raw plumbing commands
used to build each section (`git status --porcelain`, `git stash list`,
`git rev-list --left-right --count`, `git log -5`).

### Normal output

```
On branch feat/login

Staged:
  modified:  src/auth.go

Unstaged:
  modified:  src/main.go
Untracked:
  notes.md

Stashes: 1

Ahead/behind: 2 ahead, 1 behind origin/feat/login

Recent commits:
  a1b2c3d  (2 hours ago)  fix: login redirect loop
  9f8e7d6  (1 day ago)    feat: add login page
  3c4d5e6  (2 days ago)   chore: bump deps
  7a8b9c0  (3 days ago)   test: cover auth middleware
  0d1e2f3  (4 days ago)   Initial commit
```

### Clean tree, mid-rebase

```
On branch feat/login (rebase in progress)

Staged:
  (none)

Stashes: (none)

Ahead/behind: (no upstream configured)

Recent commits:
  a1b2c3d  (2 hours ago)  fix: login redirect loop
  9f8e7d6  (1 day ago)    feat: add login page
```

---

## For developers

| File                          | Contents                                                           |
| ----------------------------- | ------------------------------------------------------------------ |
| `cmd/status.go`               | Cobra command, no flags beyond global ones, config loading         |
| `internal/workflow/status.go` | Orchestrates the six sections in order, assembles final output     |
| `internal/git/status.go`      | `Status()`, `NonEmptyStatus()`, `GetParsedStatus()`, `FormatStagedLabel()` |
| `internal/git/branch.go`      | `GetHEADState()` (HEAD state + short hash)                       |
| `internal/git/stash.go`       | `StashList()` (reused from `stash`)                                 |
| `internal/git/divergence.go`  | `AheadBehind(remote, branch)` (reused from `ship`)                 |
| `internal/git/log.go`         | `RecentCommits(n int)` (shared with `gx log`'s formatting)         |
| `internal/config/config.go`   | `Load()`                                                           |
| `tests/gx-status-test.sh`     | Bash integration tests                                             |
