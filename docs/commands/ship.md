# gx ship

Push the current branch to the remote and print the Pull Request URL.

---

## Usage

```bash
gx ship [--force] [--no-pr]
```

`gx ship` synchronizes your feature branch with the remote repository:

1. **Safety Pre-flights**: Verifies no rebase/merge is in progress and ensures HEAD is not detached.
2. **Protected Branch Guard**: Refuses to push directly to protected branches (`main`, `master`, `develop`).
3. **Upstream & Divergence Check**: Fetches latest remote refs and evaluates ahead/behind counts against `<remote>/<branch>`.
4. **Push Execution**:
   - First push: Sets tracking via `git push -u <remote> <branch>`.
   - Normal push: Fast-forwards via `git push <remote> <branch>`.
   - Diverged push (with `--force`): Pushes safely via `git push --force-with-lease <remote> <branch>`.
5. **Pull Request URL**: Inspects remote forge (GitHub, GitLab, Bitbucket) and outputs the link to create or view the PR.

---

## Flags

| Flag              | Description                                                                            |
| ----------------- | -------------------------------------------------------------------------------------- |
| `--force`         | Allow pushing when local and remote history have diverged (uses `--force-with-lease`). |
| `--no-pr`         | Skip PR URL lookup and printing after push.                                            |
| `--verbose`, `-v` | Show underlying git plumbing commands.                                                 |

---

## Examples

### Standard push and PR link generation

```bash
gx ship
# ▸ git fetch origin
# ▸ git push origin feat/login
# ✔ Pushed feat/login → origin
# → https://github.com/org/repo/compare/main...feat/login?expand=1
```

### First-time branch push

```bash
gx ship
# ▸ git fetch origin
# ▸ git push -u origin feat/new-feature
# ✔ Pushed feat/new-feature → origin
# → https://github.com/org/repo/compare/main...feat/new-feature?expand=1
```

### Force push after rebase or commit amend

```bash
gx ship --force
# ▸ git fetch origin
# ▸ git push --force-with-lease origin feat/login
# ✔ Pushed feat/login → origin
# → https://github.com/org/repo/pull/42
```

---

## Edge Cases & Recovery

| Scenario                           | Behavior & Recovery                                                                                                                  |
| ---------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| Detached HEAD                      | Blocked: `Cannot ship from a detached HEAD. Create a branch first: git checkout -b <name>.`                                          |
| Protected branch                   | Hard refusal: `Refusing to push directly to protected branch "<branch>".` `--force` does not override this.                          |
| Rebase or merge in progress        | Blocked: `A rebase is already in progress.` Resolve or abort with `gx sync --continue`.                                              |
| Local is behind remote             | Refuses: `Remote has commits not on local... Pull or rebase first (gx sync).`                                                        |
| Diverged history without `--force` | Refuses with ahead/behind counts: `Local and remote history have diverged. Rebase or merge first (gx sync), or re-run with --force.` |
| Rejected non-fast-forward          | Error with hint: `Push rejected by remote. Someone may have pushed since your last fetch — run gx sync and retry.`                   |
| Unrecognized git forge host        | Push succeeds; a notice prints indicating host cannot be identified for PR creation.                                                 |
| PR lookup timeout or failure       | Push succeeds; fallback generates deterministic compare/PR URL.                                                                      |
