# gx

An opinionated git CLI that encodes common workflows into single commands —
sync, save, and ship a branch without stitching together `stash` / `fetch` /
`rebase` / `push` by hand every time.

gx is not a replacement for `git`. It wraps a handful of multi-step rituals
into safe, single commands; for anything else, use `git` directly.

**Status: in development.** Core commands are implemented and usable; several
planned commands are not yet built (see below).

## Install

Requires Go installed locally. Not yet published as a release binary or via
a package manager — build from source:

```bash
git clone https://github.com/fathss/gx.git
cd gx/src            # go.mod lives here, not the repo root
go build -o gx .
```

This produces a `gx` binary in `gx/src/`. Optionally move it onto your `PATH`:

```bash
mv gx /usr/local/bin/gx
```

or run it directly from `gx/src/` (e.g. `./gx status`) while developing.

## Getting started

```bash
cd your-repo
gx init      # writes .gx/config
gx status    # see where you stand
```

## Commands

### Implemented

| Command     | What it does                                                          |
| ----------- | --------------------------------------------------------------------- |
| `gx init`   | Set up `.gx/config` for this repo                                     |
| `gx sync`   | Stash, fetch, pull the base branch, rebase/merge, restore the stash   |
| `gx save`   | Stage changes and commit, with sensitive-file and empty-commit guards |
| `gx ship`   | Push the current branch and print the PR URL                          |
| `gx status` | Branch, staged/unstaged files, stash count, ahead/behind, recent log  |
| `gx config` | Read/write `.gx/config` values                                        |

Full behavior and flags for each are documented under `src/docs/commands/`.

### Planned, not yet implemented

| Command      | What it will do                                       |
| ------------ | ----------------------------------------------------- |
| `gx resolve` | Conflict resolution with per-file context and actions |
| `gx start`   | Sync base, then create (and later push) a new branch  |
| `gx tag`     | Cut and push a semver release tag                     |
| `gx undo`    | Safely undo the last (unpushed) commit                |
| `gx pr`      | Open the PR page for the current branch               |
| `gx clean`   | Prune local branches already merged into base         |
| `gx log`     | Opinionated, branch-scoped commit history             |
| `gx stash`   | Named stash management (list/pop/drop by index)       |

## Configuration

gx reads `.gx/config` (JSON), created by `gx init`:

```json
{
  "remote": "origin",
  "defaultBranch": "main",
  "syncStrategy": "rebase",
  "sensitivePatterns": [".env", "*.pem", "*secret*", "*.key"],
  "protectedBranches": ["main", "master", "develop"]
}
```

Edit values directly or via `gx config <key> <value>`.

## Global flags

`--verbose` — print the underlying git commands as they run

## Contributing / internals

Architecture, layering rules, and per-command specs live in `src/AGENTS.md`
and `src/docs/`.
