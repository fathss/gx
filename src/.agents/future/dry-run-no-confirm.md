# Global `--dry-run` and `--no-confirm` flags

## What

Add two global flags to `gx` for scripting and preview use:

**`--dry-run`** — walk through the full workflow logic (checks, validation, planning) but skip every `git` command. Print what *would* happen instead:

```
▸ git fetch origin
▸ git merge --ff-only origin/develop
▸ git push origin feature/x
```

The workflow still validates preconditions (clean state, branch, etc.) so the dry run reflects a realistic outcome. Runner methods (`Run`, `Output`, `CombinedOutput`) become no-ops that return empty/fake success when `--dry-run` is set.

**`--no-confirm`** — skip all interactive `bufio.Scanner` prompts and proceed with default/safe choices. Makes `gx` scriptable without `yes | gx foo`.

Implementation sketch:

```go
// cmd/root.go
var DryRun bool
var NoConfirm bool

// In PersistentPreRunE or each command:
//   runner.SetDryRun(DryRun)
//   workflow.SetNoConfirm(NoConfirm)

// runner/runner.go — DryRunRunner wraps Runner, prints headers but skips exec
type DryRunRunner struct{ Runner }
func (d *DryRunRunner) Run(args ...string) error {
    fmt.Printf("▸ git %s\n", strings.Join(args, " "))
    return nil
}
```

## Why not today

- Every workflow needs auditing or minor changes to honor these flags — not zero cost.
- No user has asked for these yet; no scripting pain point has surfaced.
- `--dry-run` semantics vary per command (some need shell preview, some need structured output) — the right abstraction isn't clear without usage.
- `--no-confirm` only matters for `gx ship` today (no other command prompts interactively). Premature generalization.
- Could slow down command dispatch if wired globally without being needed.

## When to implement

- When a user explicitly asks for scripting support.
- When gx has 3+ commands with confirmation prompts.
- When writing CI scripts that need gx (the concrete trigger: someone wants to run `gx ship --no-confirm` in CI).
- Or as a natural addition when adding a new command that needs `--dry-run` to be safe (e.g. `gx reset` or `gx prune`).
