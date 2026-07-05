---
name: capture-future-idea
description: >
  Capture a future idea into `.agents/future/` as a structured markdown file.
  Use when you have an idea that's not ready to implement today but should be
  documented for later — a feature, refactor, or design change that the project
  should revisit when it matures.
---

# Capture future idea

Save an idea for later into `.agents/future/<short-name>.md` so it's not lost
but doesn't clutter the active issue tracker or codebase.

## Structure

Each future-idea file has three sections:

```markdown
# Title — short name for the idea

## What

What the idea is — enough context to understand it months later. Include
sketches, type signatures, code snippets, whatever makes it concrete.

## Why not today

Why this isn't being implemented now. Honest reasons:
- Prerequisite work not done yet
- Doesn't solve a current problem
- Would be premature abstraction
- Depends on external changes
- Need more real-world usage to validate the shape

## When to implement

The trigger condition — what needs to be true before this is worth doing.
Specific and actionable (e.g. "when TUI work starts", "when gx has 10+
commands", "when someone asks for this feature three times").
```

## Workflow

1. Check `.agents/future/` for existing entries (avoid duplicates).
2. Create a new `.md` file with a short kebab-case name.
3. Fill in the three sections.
4. Move on — the idea is captured, no further action needed.
