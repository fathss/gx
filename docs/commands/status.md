# gx status

Display a clean, structured snapshot of repository state.

---

## Usage

```bash
gx status
```

`gx status` is a read-only command that summarizes repository state in six fixed sections:

1. **Branch / HEAD State**: Active branch name, detached HEAD status, and any active rebase or merge operations.
2. **Conflicted Files**: Unresolved merge conflicts surfaced under a `Conflicted:` section with `C` status badge.
3. **Staged Files**: Changes ready to commit, displayed with colored status badges (`A` green, `M` yellow, `D` red, `R` white) and line diffstats (`+ins -del`).
4. **Unstaged & Untracked Files**: Modified tracked files (badged `M` or `D` with diffstats) and untracked files (badged `U` green) unified under the `Unstaged:` section (omitted if clean).
5. **Stash Count**: Total number of entries in the git stash stack.
6. **Ahead / Behind Counts**: Distance from `<remote>/<branch>` (gx's single definition of upstream).
7. **Recent Commits**: Last 5 commits on the current branch with hash, relative timestamp, and subject.

---

## Flags

| Flag              | Description                                                                               |
| ----------------- | ----------------------------------------------------------------------------------------- |
| `--verbose`, `-v` | Show underlying git plumbing commands (`git status --porcelain`, `git stash list`, etc.). |
| `--no-pager`      | Suppress the pager when output exceeds terminal height.                                   |

---

## Examples

### Standard repository snapshot

```bash
gx status
# On branch feat/login
#
# Staged:
#   M  src/auth.go +12 -3
#   A  src/auth_test.go +45
#
# Unstaged:
#   M  src/main.go +5 -1
#   U  notes.md
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

### Repository snapshot with conflicts

```bash
gx status
# On branch feat/login (rebase in progress)
#
# Conflicted:
#   C  both modified: src/auth.go
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
| Unresolved merge conflicts    | Surfaced under a distinct `Conflicted:` group before the Staged section with `C` badge.                     |
| No upstream configured        | Ahead/behind displays: `(no upstream configured)`.                                                          |
| Empty repository (no commits) | Recent commits displays: `(none — no commits yet)`.                                                         |
| Fewer than 5 commits          | Displays all available commits without artificial padding lines.                                            |
