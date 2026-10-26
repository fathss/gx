# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-10-26

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
