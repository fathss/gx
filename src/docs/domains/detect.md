# Detect Domain

**File:** `internal/git/detect.go`

## DetectRemote

```
git config branch.<current>.remote   (fallback: git remote)
```

Returns the remote tracking the current branch, or the first configured
remote. Returns an error if no remotes are configured.

**Detection order:**
1. Current branch's tracking remote (`git config branch.<name>.remote`)
2. First entry from `git remote`

---

## DetectDefaultBranch

```
git rev-parse --abbrev-ref <remote>/HEAD   (fallback: local branch check)
```

Detects the remote's HEAD branch. Returns an error if detection fails.

**Detection order:**
1. `<remote>/HEAD` symbolic ref (set by `git clone`)
2. Local branches: checks `main`, `master`, `develop` in order

| Edge case | Behaviour |
|---|---|
| No remote configured | Error. |
| Remote has no HEAD | Falls back to local branch names. |
| No matching local branch | Error — user must specify explicitly. |
