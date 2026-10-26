# gx status

Display a clean, structured snapshot of repository state.

---

## Usage

```bash
gx status
```

`gx status` is a read-only command that summarizes repository state in six fixed sections:

1. **Branch / HEAD State**: Active branch name, detached HEAD status, and any active rebase or merge operations.
2. **Staged Files**: Changes ready to commit, categorized by porcelain status (`modified`, `new file`, `deleted`, `renamed`).
3. **Unstaged & Untracked Files**: Modified tracked files and untracked files (omitted if clean).
4. **Stash Count**: Total number of entries in the git stash stack.
5. **Ahead / Behind Counts**: Distance from `<remote>/<branch>` (gx's single definition of upstream).
6. **Recent Commits**: Last 5 commits on the current branch with hash, relative timestamp, and subject.

---

## Flags

| Flag              | Description                                                                               |
| ----------------- | ----------------------------------------------------------------------------------------- |
| `--verbose`, `-v` | Show underlying git plumbing commands (`git status --porcelain`, `git stash list`, etc.). |

---

## Examples

### Standard repository snapshot

```bash
gx status
# On branch feat/login
#
# Staged:
#   modified:  src/auth.go
#
# Unstaged:
#   modified:  src/main.go
# Untracked:
#   notes.md
#
# Stashes: 1
#
# Ahead/behind: 2 ahead, 1 behind origin/feat/login
#
# Recent commits:
#   a1b2c3d  (2 hours ago)  fix: login redirect loop
#   9f8e7d6  (1 day ago)    feat: add login page
#   3c4d5e6  (2 days ago)   chore: bump deps
#   7a8b9c0  (3 days ago)   test: cover auth middleware
#   0d1e2f3  (4 days ago)   Initial commit
```

### Clean working tree mid-rebase

```bash
gx status
# On branch feat/login (rebase in progress)
#
# Staged:
#   (none)
#
# Stashes: (none)
#
# Ahead/behind: (no upstream configured)
#
# Recent commits:
#   a1b2c3d  (2 hours ago)  fix: login redirect loop
```

---

## Edge Cases & Recovery

| Scenario                      | Behavior & Recovery                                                                                         |
| ----------------------------- | ----------------------------------------------------------------------------------------------------------- |
| Outside a Git repository      | Blocked by pre-flight guard: `Not inside a Git repository.`                                                 |
| Missing configuration         | Blocked by pre-flight guard: run `gx init` first.                                                           |
| Rebase or merge in progress   | Non-blocking. Surfaced on the first line: `On branch <name> (rebase in progress)` or `(merge in progress)`. |
| Detached HEAD                 | Non-blocking. Displayed as `HEAD detached at <shortHash>`.                                                  |
| Unresolved merge conflicts    | Surfaced under a distinct `Conflicted:` group before the Staged section.                                    |
| No upstream configured        | Ahead/behind displays: `(no upstream configured)`.                                                          |
| Empty repository (no commits) | Recent commits displays: `(none — no commits yet)`.                                                         |
| Fewer than 5 commits          | Displays all available commits without artificial padding lines.                                            |
