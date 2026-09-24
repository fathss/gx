# Remote Domain

**File:** `internal/git/remote.go`

## Fetch

```
git fetch <remote>
```

Update remote-tracking refs without modifying the working tree.

| Failure | Common cause |
|---|---|
| Offline | No network connection. |
| Authentication failed | Invalid credentials or SSH key. |
| Remote missing | Remote URL is wrong or the repo was deleted. |
| DNS failure | Hostname can't be resolved. |

**Workflow message:** "Failed to fetch remote."  
**Hint:** "Check network connection or remote configuration."

---

## FastForward

```
git merge --ff-only <remote>/<branch>
```

Update a local branch to match its remote-tracking counterpart, but only when a fast-forward is possible (no divergent local commits).

| Edge case | Behaviour |
|---|---|
| Local branch has diverged | Fails — manual inspection needed. Workflow should suggest the user inspect the base branch manually. |

---

## RemoteExists

```
git remote get-url <name>
```

Returns `true` if the given remote name is configured. Used by the sync workflow to validate remote before fetching.

---

## RemoteBranchExists

```
git rev-parse --verify <remote>/<branch>
```

Returns `true` if the remote-tracking branch ref exists. Used by the sync workflow to check whether a remote branch exists before fast-forwarding.

---

## RemoteHEAD

```
git symbolic-ref refs/remotes/<remote>/HEAD
```

Returns the branch that the remote's HEAD symbolic-ref points to (e.g. `"main"`). Returns `""` if the remote HEAD cannot be resolved. Used by the sync workflow to detect stale remote default branches.

---

## Push

```
git push <remote> <branch>
```

Push the current branch to remote without setting upstream. Used by `gx ship` for subsequent (non-first) pushes when histories haven't diverged.

---

## PushSetUpstream

```
git push -u <remote> <branch>
```

Push and set upstream tracking (`-u`). Used by `gx ship` for first-time pushes (no upstream tracking yet).

---

## PushForceWithLease

```
git push --force-with-lease <remote> <branch>
```

Force-push with lease (safe force — rejects if someone else pushed in between). Used by `gx ship --force`. Never uses a bare `--force`.

---

## HasUpstream

```
git rev-parse --abbrev-ref <branch>@{upstream}
```

Returns `true` if the branch has an upstream tracking branch configured. Used by `gx ship` to decide whether to run the divergence check.

---

## RemoteURL

```
git remote get-url <remote>
```

Returns the URL of the given remote. Used by `gx ship` after a successful push to feed the forge URL detection.

---

## DeleteRemoteBranch

```
git push <remote> --delete <branch>
```

Deletes a branch on the remote. Affects every collaborator — the branch is
removed from the shared remote, not just local refs.
