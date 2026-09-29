# Ref Domain

**File:** `internal/git/ref.go`

## Overview

The ref vocabulary shared by every git-layer function: parse, qualify, and
strip remote-tracking refs in one place so ref-format bugs land in exactly
one module. Workflows never manipulate ref strings themselves.

---

## Qualify

```go
func Qualify(remote, branch string) string
```

Returns `branch` qualified with `remote` (e.g. `origin/main`). **Idempotent:**
an already-qualified ref is returned unchanged, so callers never need a
pre-check. Returns `branch` unchanged when `remote` is empty.

Used by `MergedRemoteBranches`, `DeleteRemoteTrackingBranch`, `FastForward`,
`RemoteBranchExists`, `CheckDivergence`, and workflow display strings.

---

## StripRemote

```go
func StripRemote(remote, ref string) string
```

Removes the `<remote>/` prefix from a remote-tracking ref and returns the
bare branch name. Refs not carrying the prefix are returned unchanged.

Used by `MergedRemoteBranches` and `DetectDefaultBranch`.
