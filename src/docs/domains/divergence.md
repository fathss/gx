# Divergence Domain

**File:** `internal/git/divergence.go`

## Overview

Computes how far a local branch has diverged from its remote-tracking
counterpart. Consumed by `gx status` (ahead/behind section) and the `gx ship`
divergence check.

---

## AheadBehind

```go
func AheadBehind(run *runner.Runner, remote, branch string) (ahead, behind int, err error)
```

Returns the number of commits the local branch is ahead of and behind its
remote-tracking counterpart:

```
git rev-list --count --left-right <remote>/<branch>...HEAD
```

`--left-right` prefixes each counted commit with `<` (left side of the `...`
range — the remote) or `>` (right side — HEAD), and `--count` emits one
`<left>\t<right>` line:

```
<behind>	<ahead>
```

- **Left** = commits the remote has that local doesn't = **behind**
- **Right** = commits local has that the remote doesn't = **ahead**

### Parsing

The output is split with `strings.Fields` (whitespace-split), **not**
fixed-width substrings — `runner.Output` trims leading whitespace of the full
output, so a fixed-width slice of the left column would be corrupted.

| Edge case | Behaviour |
|---|---|
| Remote-tracking ref doesn't exist (no upstream yet) | Returns `-1, -1, nil` |
| Output doesn't split into exactly 2 fields | Returns `0, 0, nil` |
| Fields present but non-numeric | Returns `0, 0, error` with the raw output |

### Consumers

- `gx status` — ahead/behind section
- `gx ship` — divergence check before push