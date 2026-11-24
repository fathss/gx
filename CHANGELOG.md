# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

## 0.4.1 - 2026-11-24

### Fixed

- **Interactive Terminal Prompts & Authentication**:
  - Replaced the asynchronous background spinner loop with clean, non-destructive state indicators, resolving an issue where Git credential prompts (GitHub username/password, SSH passphrases, GPG prompts) were erased by terminal clear codes (`\r\033[K`) and cursor hiding.
  - Streamed captured Git stderr on subprocess failure so underlying authentication and remote errors (e.g., `fatal: Authentication failed for ...`) are immediately visible to the user above actionable hints.
  - Removed specialized subprocess stopping logic across runner execution paths in favor of reliable terminal semantics.
- **`gx clean` Output When No Candidates Exist**:
  - Changed output message from `Pruned 0 branches.` to `No branches to prune.` when there are no candidate branches to delete.

## 0.4.0 - 2026-11-24

### Added

- **Automatic Remote Push in `gx sync`**:
  - Automatically pushes the synchronized feature branch to `<remote>` after Phase 2 (fast-forwards if ahead, safe force-pushes with `--force-with-lease` if diverged after rebase, sets upstream `-u` if untracked).
  - Skips pushing when on a base/protected branch or when up to date with remote.
  - Added `--no-push` flag to `gx sync` (and `--continue` / `--skip`) to bypass pushing to remote and retain the `hint: Run gx ship to push.` prompt.
  - Added mutual exclusion guard blocking `--abort` and `--no-push` simultaneously.
- **Repository Root Config Discovery (`FindConfigFile`)**:
  - Automatically traverses parent directories upwards from the current working directory to locate `.gx/config` or the enclosing Git repository root (`.git`).
  - Enables `gx` commands and `gx config` reads/writes to run seamlessly from child directories while targeting the root configuration.
  - Directs `gx init` executed from child directories to create `.gx/config` at the enclosing Git repository root rather than the subdirectory.

### Changed

- **`gx ship` Output Formatting**:
  - Standardized push success message format to `Pushed <branch> to <remote>` (replacing `Pushed <branch> → <remote>`).
  - Standardized Pull Request creation URL display with indented label `  Pull Request: <url>` (replacing `→ <url>`).
- **`gx sync` Push Rejection Hint**:
  - Improved actionable hint when remote rejects a push during sync (`New commits were pushed to the remote while syncing. Run gx sync again, or inspect git log.`).

## 0.3.0 - 2026-11-10

### Added

- **Two-Phase Feature Branch Sync in `gx sync`**:
  - Automatically reconciles local feature branches with their remote tracking counterpart (`origin/<branch>`) in Phase 1 before integrating with base branch in Phase 2.
  - Detects divergence on feature branches and automatically integrates remote commits using the configured/specified sync strategy (`rebase` or `merge`).
  - Added patch-ID comparison (`git cherry`) via `git.HasUnappliedCommits` to avoid redundant re-rebasing of already applied commits, ensuring consecutive syncs are fully idempotent.
  - Added `git.IsAncestor` check to skip redundant rebase/merge operations when the base branch is already an ancestor of HEAD.
- **Target Branch Sync & Mismatch Guard in `gx sync [<remote> [<target_branch>]]`**:
  - Specifying `<target_branch>` matching current branch syncs exclusively with `<remote>/<target_branch>`, skipping base branch reconciliation.
  - Branch mismatch guard: blocks execution immediately with a checkout hint if `<target_branch>` differs from the currently checked-out branch.
- **Contextual `gx ship` Hint**:
  - `gx sync` (and `--continue`/`--skip`) prompts `hint: Run gx ship to push.` when sync completes on a feature branch with unpushed commits.

### Changed

- **`gx clean` Local & Remote Pruning**:
  - Plain `gx clean` strictly prunes merged local branches and no longer removes remote-tracking refs whose branches still exist on remote.
  - `gx clean --remote` replaces double confirmation prompts with a single unified confirmation prompt listing both local and remote branches on `origin`.
  - Eliminates duplicate counting of remote tracking refs.
- **CLI Help Text (`-h`) Standardization**:
  - Standardized `Examples:` blocks across all commands (`clean`, `sync`, `ship`, `status`, `save`, `stage`, `init`, `config`) without `#` comments.
  - Aligned all descriptions to column 29 (4 spaces after the furthest command).
  - Removed duplicate hardcoded `Flags:` sections inside command descriptions (`save`, `stage`).
  - Added comprehensive `Long:` descriptions and `Examples:` to `gx sync`, `gx ship`, and `gx status`.

## 0.2.1 - 2026-11-09

### Changed

- Change gx version to 0.2.1

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
