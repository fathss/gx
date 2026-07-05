# Error Handling

Errors flow through the layers — each layer only wraps errors from the layer below.

```
runner  →  raw exec errors (never wrapped)

  ↓

git     →  raw git errors (never wrapped, never creates cli.Error)

  ↓

workflow →  cli.Error{Message, Hint}  ← every external failure becomes this

  ↓

cmd     →  passes error up to main

  ↓

main    →  prints the error (only main.go renders errors)
```

---

## Rules by layer

### runner — never wrap errors

```go
// ✅ Good
return cmd.Run()

// ❌ Bad
return fmt.Errorf("git failed: %w", err)
```

### git — never create cli.Error

```go
// ✅ Good
func Fetch(run *runner.Runner, remote string) error {
    return run.Run("fetch", remote)
}

// ❌ Bad
return &cli.Error{Message: "fetch failed"}
```

### workflow — every external failure becomes cli.Error

```go
if err := git.Fetch(run, cfg.Remote); err != nil {
    return &cli.Error{
        Message: "Failed to fetch remote.",
        Hint:    "Check network connection or remote configuration.",
    }
}
```

`Hint` gives the user an actionable next step.

### main — only this file prints errors

```go
func printError(err error) {
    var cliErr *cli.Error
    if errors.As(err, &cliErr) {
        fmt.Fprintf(os.Stderr, "✗ %s\n", cliErr.Message)
        if cliErr.Hint != "" {
            for _, line := range strings.Split(cliErr.Hint, "\n") {
                fmt.Fprintf(os.Stderr, "hint: %s\n", line)
            }
        }
        return
    }
    fmt.Fprintf(os.Stderr, "✗ %v\n", err)
}
```

Multi-line hints are split into separate `hint:` lines (git-style). No other package prints failures to the terminal.
