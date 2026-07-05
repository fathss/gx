# Commit Domain

**File:** `internal/git/commit.go`

## Commit

```
git commit -m <msg>
```

Create a commit with the provided message. No editor is opened.

---

## CommitEditor

```
git commit
```

Create a commit by opening the configured `$EDITOR`. The editor shows the
default commit message template (commented-out status, diff summary).

---

## CommitAllowEmpty

```
git commit --allow-empty -m <msg>
```

Create an empty commit (no files changed) with the provided message. Used
for CI triggers, placeholder commits, or workflow markers.

---

## CommitAllowEmptyEditor

```
git commit --allow-empty
```

Create an empty commit by opening `$EDITOR`. Used when `--allow-empty` is
active and no `-m` flag is provided.
