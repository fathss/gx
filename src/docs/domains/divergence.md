# Divergence Domain

**File:** `internal/git/divergence.go`

## Overview

Computes how far a local branch has diverged from its counterpart at
`<remote>/<branch>` — gx's **one notion of upstream**: the configured
remote's copy of the branch, never `@{upstream}` (which can point at a
different remote entirely). Consumed by `gx status` (ahead/behind section)
and the `gx ship` divergence check.

---

## Divergence

```go
type Divergence struct {
	Ahead    int
	Behind   int
	Tracking bool
}
```

`Tracking` reports whether the remote-tracking ref `<remote>/<branch>`
exists at all. `Ahead`/`Behind` are only meaningful when `Tracking` is true.

---

## CheckDivergence

```go
func CheckDivergence(run runner.Executor, remote, branch string) (Divergence, error)
```

Two queries, in order:

1. `git rev-parse --verify <remote>/<branch>` — establishes `Tracking`
2. `git rev-list --count --left-right <remote>/<branch>...HEAD` — only run
   when the ref exists

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
fixed-width substrings — the runner right-trims trailing line breaks only,
so `Fields` is the robust choice regardless of tab/space separation.

### Edge cases

| Edge case | Behaviour |
|---|---|
| Remote-tracking ref doesn't exist (never pushed) | Returns `Divergence{Tracking: false}, nil` — a normal state, not an error; `rev-list` is skipped |
| `rev-list` fails (broken repo, bad revision) | Returns the error — callers surface their own failure message |
| Output doesn't split into exactly 2 fields | Returns an error with the raw output |
| Fields present but non-numeric | Returns an error with the raw output |

There is deliberately **no sentinel value**: a real git failure can never be
mistaken for "no upstream", and "no upstream" can never be mistaken for a
count.

### Consumers

- `gx status` — ahead/behind section; `Tracking == false` prints
  `(no upstream configured)`
- `gx ship` — decides first push (`!Tracking`) vs. divergence check before
  push
