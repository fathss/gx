# gx ship

Push the current branch, set upstream if needed, and print the PR creation URL.

## Usage

```
gx ship [--force] [--no-pr]
```

`gx ship` pushes the current branch to the configured remote, refusing to
push directly to protected branches. It detects diverged history and
requires an explicit `--force` before force-pushing. On success it prints
the PR creation URL for the detected git host (or the existing PR's URL, if
one is already open for this branch).

```bash
gx ship            # push branch, print PR url
gx ship --force    # force-push after a rebase/amend
gx ship --no-pr    # push only, skip PR url lookup/printing
```

---

## Flags

| Flag      | Purpose                                                    |
| --------- | ---------------------------------------------------------- |
| `--force` | Allow pushing when local and remote history have diverged. |
| `--no-pr` | Skip PR URL lookup/printing after push.                    |

The global `--verbose` / `-v` flag applies as usual to show plumbing commands.

---

## Flow

### Pre-flight

Before anything else, the central guard checks apply (not a Git repo, no
`.gx/config`). `gx ship` then checks that no rebase or merge is in
progress — same as `gx save`:

```
✗ A rebase is already in progress.
hint: Resolve or abort the rebase first.
hint: Use gx sync --continue or git rebase --abort/--continue.
```

Detached HEAD is blocked (there's no branch to ship):

```
✗ Cannot ship from a detached HEAD.
hint: Create a branch first: git checkout -b <name>.
```

### Protected branch check

The current branch name is checked against `protectedBranches` in
`.gx/config` (defaults to `main`, `master`, `develop` if unset). If it
matches, `gx ship` refuses unconditionally — `--force` does **not**
override this, since the fix here is a different workflow, not a flag:

```
✗ Refusing to push directly to protected branch "main".
hint: Open a feature branch with git checkout -b <name> or cut a release with gx tag.
```

### Divergence check

`gx ship` fetches the latest remote-tracking ref for the current branch
(no merge/rebase — just `git fetch <remote> <branch>` to update the ref)
and compares local vs. remote:

| Local vs. remote                              | Behavior                                                                                                                                                  |
| --------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| No upstream configured yet                    | Proceed to first-push path (see below).                                                                                                                   |
| Remote is an ancestor of local (fast-forward) | Proceed with a normal `push`.                                                                                                                             |
| Local is an ancestor of remote                | Block — pulling is needed, pushing would lose nothing but is pointless: `git push` would simply reject it, so `gx ship` surfaces a clearer message first. |
| Histories diverged                            | Block unless `--force` is passed.                                                                                                                         |

Diverged, no `--force`:

```
✗ Local and remote history have diverged for "feat/login".
  local:  3 commit(s) not on remote
  remote: 2 commit(s) not on local
hint: Rebase or merge first (gx sync), or re-run with --force if you intend
      to overwrite remote history.
```

With `--force`, the push uses `--force-with-lease` rather than a bare
`--force`, so a push is still rejected if someone else pushed to the branch
between the fetch and the push (protects against clobbering a teammate's
work you haven't seen):

```
▸ git push --force-with-lease origin feat/login
```

### Push

**First push (no upstream tracking yet):**

```
▸ git push -u origin feat/login
```

**Subsequent push (upstream already set, fast-forward):**

```
▸ git push origin feat/login
```

**Diverged + `--force`:**

```
▸ git push --force-with-lease origin feat/login
```

If the push itself fails (auth, hook rejection, `--force-with-lease` stale
lease because someone pushed in between), `gx ship` exits with the git
error surfaced as-is plus a hint:

```
✗ Push rejected by remote.
hint: Someone may have pushed since your last fetch — run gx sync and retry.
```

### PR URL

Unless `--no-pr` is passed, after a successful push `gx ship` detects the
host from the remote URL (GitHub, GitLab, or Bitbucket) via the `forge`
package and either:

- constructs the PR/MR _creation_ URL for this branch → `base_branch`, or
- if a PR/MR already exists for this branch, prints that PR's URL instead
  (same lookup `gx pr` uses)

```
✔ Pushed feat/login → origin
→ https://github.com/acme/widgets/compare/main...feat/login?expand=1
```

If the host can't be detected (unknown remote, e.g. a bare local path or
self-hosted git server not matching known patterns), this step is skipped
with a one-line note rather than an error — the push already succeeded:

```
✔ Pushed feat/login → origin
(could not detect a known git host for PR URL — remote: /srv/git/widgets.git)
```

---

## Edge cases

| Condition                                   | Behaviour                                                                                                                                         |
| ------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| Not a Git repository                        | Central guard blocks: "Not inside a Git repository."                                                                                              |
| No `.gx/config`                             | Central guard blocks: "run gx init to start using gx"                                                                                             |
| Rebase in progress                          | Block: "A rebase is already in progress." Hint to abort or continue.                                                                              |
| Merge in progress                           | Block: "A merge is already in progress." Hint to abort or continue.                                                                               |
| Detached HEAD                               | Block: "Cannot ship from a detached HEAD."                                                                                                        |
| Current branch is protected                 | Block, unconditionally — `--force` does not override.                                                                                             |
| No commits ahead of base at all             | `git push` runs anyway (idempotent); if remote already matches, git reports "Everything up-to-date" and `gx ship` still attempts the PR URL step. |
| No upstream set yet                         | Push with `-u origin <branch>`, no divergence check needed.                                                                                       |
| Fast-forward push possible                  | Normal push, no `--force` needed.                                                                                                                 |
| Diverged, no `--force`                      | Block with ahead/behind counts and a hint.                                                                                                        |
| Diverged, `--force`                         | Push with `--force-with-lease` (never a bare `--force`).                                                                                          |
| `--force` with nothing diverged             | No-op flag — normal push proceeds, no warning needed.                                                                                             |
| `--force-with-lease` rejected (stale lease) | Push error surfaced with hint to `gx sync` and retry.                                                                                             |
| Push fails (auth, hook)                     | Git error surfaced as-is plus a generic retry hint.                                                                                               |
| Push succeeds, `--no-pr` passed             | Skip PR URL step entirely.                                                                                                                        |
| Push succeeds, PR already exists for branch | Print the existing PR's URL instead of a creation link.                                                                                           |
| Push succeeds, host undetected              | Skip PR URL step with a non-fatal note; exit code still 0.                                                                                        |
| Push succeeds, PR lookup API call fails     | Non-fatal — print push success, note that PR URL lookup failed.                                                                                   |

---

## User-visible output

Every git command prints `▸ git <args>` followed by git's own output.
Plumbing used for the divergence check (`rev-list --count`, `ls-remote`)
is hidden by default; pass `--verbose` to show it.

gx-specific messages:

| Message                                                        | Trigger                                    |
| -------------------------------------------------------------- | ------------------------------------------ |
| `✗ Refusing to push directly to protected branch "<branch>".`  | Current branch matches `protectedBranches` |
| `✗ Cannot ship from a detached HEAD.`                          | HEAD is detached                           |
| `✗ Local and remote history have diverged for "<branch>".`     | Divergence detected, no `--force`          |
| `✗ Push rejected by remote.`                                   | `git push` returned non-zero               |
| `✔ Pushed <branch> → <remote>`                                 | Push succeeded                             |
| `→ <pr-url>`                                                   | PR URL resolved                            |
| `(could not detect a known git host for PR URL — remote: ...)` | Host detection failed (non-fatal)          |

### Normal success (first push)

```
▸ git push -u origin feat/login
✔ Pushed feat/login → origin
→ https://github.com/acme/widgets/compare/main...feat/login?expand=1
```

### Protected branch blocked

```
✗ Refusing to push directly to protected branch "main".
hint: Open a feature branch with git checkout -b <name> or cut a release with gx tag.
```

### Diverged, blocked

```
✗ Local and remote history have diverged for "feat/login".
  local:  3 commit(s) not on remote
  remote: 2 commit(s) not on local
hint: Rebase or merge first (gx sync), or re-run with --force if you intend
      to overwrite remote history.
```

### Diverged, forced

```
▸ git push --force-with-lease origin feat/login
✔ Pushed feat/login → origin
→ https://github.com/acme/widgets/pull/42
```

---

## For developers

| File                          | Contents                                                                              |
| ----------------------------- | ------------------------------------------------------------------------------------- |
| `cmd/ship.go`                 | Cobra command, `--force`/`--no-pr` flags, config loading                              |
| `internal/workflow/ship.go`   | Protected-branch check, divergence check, push, PR-url orchestration                  |
| `internal/git/branch.go`      | `CurrentBranch()` (empty string = detached HEAD)                                      |
| `internal/git/remote.go`      | `Push()`, `PushSetUpstream()`, `PushForceWithLease()`, `HasUpstream()`, `RemoteURL()` |
| `internal/git/divergence.go`  | `AheadBehind(remote, branch) (ahead, behind int, err error)`                          |
| `internal/config/config.go`   | `Load()`, `ProtectedBranches` field (defaults to `main,master,develop`)               |
| `internal/forge/forge.go`     | Interface: `PRCreateURL(remote, base, branch) string`, `ExistingPRURL(...)`           |
| `internal/forge/github.go`    | GitHub URL construction + existing-PR lookup                                          |
| `internal/forge/gitlab.go`    | GitLab URL construction + existing-MR lookup                                          |
| `internal/forge/bitbucket.go` | Bitbucket URL construction + existing-PR lookup                                       |
| `tests/gx-ship-test.sh`       | Bash integration tests                                                                |
