# Structured CLI output types (`cli.Success`, `cli.Info`)

## What

Add `cli.Success` and `cli.Info` types (and a runner handler) alongside `cli.Warn`, so workflow code emits structured notifications instead of raw `fmt.Println`:

```go
type Success struct {
    Message string
}

type Info struct {
    Message string
}
```

The runner would get a `successHandler` / `infoHandler` (same pattern as `warnHandler`). The cmd layer wires them to stdout:

```go
run.Success("✔ Pushed feature/x → origin")
run.Info("7 files changed, 42 insertions(+)")
```

When the TUI arrives, these become renderable notification types (green text, blue text, icons) rather than plain stdout lines.

## Why not today

- `cli.Warn` solves an active problem (non-fatal stderr writes violating the architecture). `cli.Success`/`cli.Info` don't — today's `fmt.Println` calls are fine and don't cross any layer boundary.
- Defining types before they're consumed adds ceremony without benefit. The TUI renderer isn't built yet, so there's no clear contract to design against.
- Premature abstraction risks getting the shape wrong. A future TUI might want `Success{Message, Detail, Action}` or `Info{Message, Verbose, Meta}` — we can't know yet.

## When to implement

**Alongside or just before TUI work starts.** When you begin building the TUI renderer, add these types then — not before. The TUI will inform the exact shape they need. `cli.Warn` is the exception because it solves a concrete problem today.
