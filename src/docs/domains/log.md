# Log Domain

**File:** `internal/git/log.go`

## Overview

Commit history access for the `gx status` recent-commits section (5 most
recent commits) and the planned `gx log` command.

---

## CommitInfo

```go
type CommitInfo struct {
    ShortHash    string // abbreviated hash (7 chars)
    RelativeDate string // e.g. "2 hours ago"
    Subject      string // first line of commit message
}
```

Holds information about a single commit.

---

## RecentCommits

```go
func RecentCommits(run *runner.Runner, n int) ([]CommitInfo, error)
```

Returns the last `n` commits on the current branch:

```
git log -n <n> --format=%h|||%ar|||%s
```

**Delimiter choice:** `%h`, `%ar`, and `%s` are joined with `|||` — a delimiter
that cannot appear in a subject — so subjects containing spaces survive the
field split intact. A space or tab-based format would break on multi-word
subjects.

Each line is parsed with `SplitN(line, "|||", 3)`:

| Field | Source | Example |
|---|---|---|
| `ShortHash` | `%h` | `3f9c2ab` |
| `RelativeDate` | `%ar` | `2 hours ago` |
| `Subject` | `%s` | first line of commit message |

### Behaviour

- **No commits** — returns an empty slice.
- **Malformed lines** — lines that don't split into exactly 3 parts are skipped.
- **Empty fields** — commits with an empty hash or subject are skipped.

### Consumers

- `gx status` — recent-commits section (`RecentCommits(run, 5)`)
- `gx log` — planned command (shared formatting)