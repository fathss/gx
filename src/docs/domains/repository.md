# Repository Domain

**File:** `internal/git/repository.go`

## IsRepository

```
git rev-parse --git-dir
```

Check whether the current directory is inside a git repository.

Returns `true` if the command succeeds (we're in a repo), `false` otherwise.

| Failure | Workflow response |
|---|---|
| Not a git repository | `cli.Error{Message: "Not inside a Git repository.", Hint: "Run gx inside a cloned repository."}` |
| Git executable unavailable | Error from runner. |
