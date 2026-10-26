# gx Documentation

Welcome to the gx documentation.

## User Guide

User-facing reference for each gx command:

- [gx init](commands/init.md) — Initialize `.gx/config` for a repository with auto-detection
- [gx sync](commands/sync.md) — Sync feature branch with base branch (stash, fetch, rebase/merge, restore)
- [gx save](commands/save.md) — Stage changes and commit with sensitive pattern and empty commit guards
- [gx ship](commands/ship.md) — Push feature branch and output PR creation URL
- [gx status](commands/status.md) — Human-readable repository state snapshot
- [gx clean](commands/clean.md) — Prune branches merged into the base branch
- [gx config](commands/config.md) — Inspect and update `.gx/config` values

## Concepts & Specifications

Cross-cutting behavioral models and invariants:

- [Configuration](concepts/configuration.md) — `.gx/config` schema, default values, and precedence rules
- [Sync State Machine](concepts/sync-machine.md) — Q1–Q6 state classification, pause recovery, and stash isolation

## Architecture & Contributor Guide

Internal design documentation for gx developers:

- [Architecture & Layering](internal/architecture.md) — Strict layer boundaries (`cmd` → `workflow` → `git` → `runner`)
- [Error Handling](internal/error-handling.md) — Error propagation rules and types
- [Git Domains Reference](internal/git-domains.md) — Low-level git wrappers and operations
