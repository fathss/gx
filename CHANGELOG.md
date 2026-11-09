# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

## 0.2.0 - 2026-11-09

### Added

- **`gx stage`**: Dedicated staging command (aliased as `gx add`) with sensitive pattern guardrails:
  - Categorized preview of unstaged tracked files and new untracked files with status badges and diffstats.
  - Interactive confirmation prompt for untracked files (`Stage X untracked file(s)?`).
  - Flags: `-a, --all` to stage all files without prompting, `-p, --patch` for interactive hunk staging (`git add -p`), `--allow-sensitive` to bypass sensitive file blocking, and `--exclude` to filter paths.
  - Summary report of files staged during the command (`Staged X file(s) (+ins | -del)`).
- **`gx save --all` (`-a`)**: Stage all modified and untracked files before committing without confirmation prompts.
- **Pre-staged Commit Support in `gx save`**: Directly commits already-staged changes without re-staging remaining working tree modifications or prompting.
- **Commit Summary Card**: `gx save` renders a formatted commit summary card displaying commit hash, branch name, commit message, categorized changes (`New files:`, `Modified:`, `Deleted:`, `Renamed:`) with diffstats, and overall file/line counts.
- **Terminal Pager Support**: Integrated cross-platform terminal pagination via `internal/pager` (`pager.Print`), respecting `$PAGER` or defaulting to `less -FIRX`, with global `--no-pager` flag support across commands (`status`, `stage`, `save`, `clean`).
- **Terminal UI Utilities & Spinners**: Added zero-dependency ANSI styling package (`internal/ui`) with color helpers (`White`, `Dim`, `Success`, `Danger`, `Warning`), status badges, and an ASCII slash spinner component with TTY and `NO_COLOR`/`TERM=dumb` detection.
- **Structured Runner Feedback**: Extended `runner.Executor` with structured notification methods (`Success`, `Info`, `State`, `StopSpinner`, `StartProgress`, `UpdateProgress`, `EndProgress`) to streamline interactive CLI feedback and suppress plumbing output in non-verbose mode.

### Changed

- **`gx status` Output Formatting**:
  - Unified modified tracked files and untracked files under a single `Unstaged:` section.
  - Added colored status badges (`A`, `M`, `D`, `R`, `C`, `U`) and line diffstats (`+ins -del`).
  - Surfaced unresolved merge conflicts under a dedicated `Conflicted:` section with `C` badge.
- **`gx clean` Confirmation Prompts**: Enhanced confirmation prompts to cleanly distinguish between local tracking refs and remote branches with explicit counts and target descriptions.
- **Staging Area Sensitive File Protection**: `gx save` now checks both working tree files and the staging area before committing, preventing pre-staged sensitive files from being committed without `--allow-sensitive`.

## 0.1.0 - 2026-10-26

Initial release of `gx` — an opinionated Git CLI automating common developer workflows.

### Added

- **`gx init`**: Initialize `.gx/config` with auto-detection for tracking remotes and trunk branches.
- **`gx sync`**: Synchronize feature branches with base branches:
  - Transparent working-tree stashing with `gx-sync/<branch>/<timestamp>` isolation.
  - Multi-state recovery machine (Q1–Q6) protecting in-progress rebases and merges.
  - Interactive continuation (`--continue`), commit skipping (`--skip`), and clean abort (`--abort`).
- **`gx save`**: Guarded staging and committing:
  - Categorized working-tree preview (modified tracked vs. untracked).
  - Built-in hard guard against accidental staging of sensitive files (`.env`, `*.pem`, `*secret*`, `*.key`).
  - Interactive patch mode (`--patch`) and file exclusion filters (`--exclude`).
  - Empty commit prevention and confirmation prompt.
- **`gx ship`**: Safe branch publishing:
  - Divergence and ahead/behind detection against upstream.
  - Protected branch push block (`main`, `master`, `develop`).
  - Safe `--force-with-lease` invocation when `--force` is specified.
  - Automatic Pull Request creation URL generation for GitHub, GitLab, and Bitbucket.
- **`gx status`**: 6-section repository status snapshot (branch state, staged, unstaged/untracked, stash count, divergence, recent commits).
- **`gx clean`**: Safe pruning of branches merged into the base branch with clone-local and remote scopes (`--remote`) and interactive confirmation.
- **`gx config`**: Central configuration registry supporting scalar reads/writes and variadic list appends/overwrites.
- **CLI Options**:
  - Global `--verbose` (`-v`) to display underlying Git commands.
  - Global `--version` reporting CLI build version.
- **Documentation**: Comprehensive user manual, architecture specification, and Git domain references under `docs/`.
