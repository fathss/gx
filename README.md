# gx

[![CI](https://github.com/fathss/gx/actions/workflows/ci.yml/badge.svg)](https://github.com/fathss/gx/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/fathss/gx)](https://github.com/fathss/gx/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/fathss/gx)](https://goreportcard.com/report/github.com/fathss/gx)

**gx** is an opinionated Git CLI that wraps multi-step git workflows into safe, single commands — sync, save, and ship without manually juggling `stash`, `fetch`, `rebase`, and `push` every time.

gx is not a replacement for `git`. It automates routine developer rituals with safety invariants: zero commit loss, sensitive file detection, protected branch guards, and automated conflict-pause recovery.

---

## Table of Contents

- [Installation](#installation)
  - [Download Pre-built Binary](#option-1-download-pre-built-binary-recommended)
  - [Build from Source](#option-2-build-from-source-go-121)
  - [Updating gx](#updating-gx-to-the-latest-version)
  - [Shell Auto-Completion](#shell-auto-completion-optional)
- [Quick Start](#quick-start)
- [Core Commands](#core-commands)
- [Global Flags](#global-flags)
- [Configuration](#configuration)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [License](#license)

---

## Installation

You can install `gx` either by downloading a pre-compiled binary (fastest, no Go required) or by building it from source.

### Option 1: Download Pre-built Binary (Recommended)

No Go compiler required. Binaries are available for Linux, macOS, and Windows.

#### Step 1: Download the binary for your system

Go to the [GitHub Releases](https://github.com/fathss/gx/releases) page and download the archive matching your operating system and processor:

| Operating System | Processor Architecture                 | Archive File to Download           |
| ---------------- | -------------------------------------- | ---------------------------------- |
| **Linux**        | 64-bit (Intel / AMD)                   | `gx_<version>_linux_amd64.tar.gz`  |
| **Linux**        | ARM 64-bit (Raspberry Pi, ARM servers) | `gx_<version>_linux_arm64.tar.gz`  |
| **macOS**        | Apple Silicon (M1 / M2 / M3 / M4)      | `gx_<version>_darwin_arm64.tar.gz` |
| **macOS**        | Intel processor                        | `gx_<version>_darwin_amd64.tar.gz` |
| **Windows**      | 64-bit                                 | `gx_<version>_windows_amd64.zip`   |

#### Step 2: Extract and install

**For Linux and macOS:**

Open your terminal in the folder where you downloaded the file:

```bash
# 1. Extract the downloaded archive (replace with your file name)
tar -xzf gx_0.1.0_linux_amd64.tar.gz

# 2. Move the executable to a global directory in your PATH
sudo mv gx /usr/local/bin/

# 3. Verify that gx is installed and ready
gx --version
```

**For Windows:**

1. Extract the downloaded `gx_<version>_windows_amd64.zip` file.
2. Inside you will find `gx.exe`.
3. Move `gx.exe` to a folder of your choice (for example: `C:\Program Files\gx\`).
4. Add that folder to your System `PATH` environment variable so you can run `gx` from Command Prompt or PowerShell.
5. Verify in PowerShell or CMD:
   ```powershell
   gx --version
   ```

---

### Option 2: Build from Source (Go 1.21+)

If you already have Go installed on your machine, you can build `gx` directly from source:

#### Step 1: Clone the repository

```bash
git clone https://github.com/fathss/gx.git
cd gx
```

#### Step 2: Compile and install

Run `make install`:

```bash
make install
```

_What this does:_ Compiles the code and places the binary inside your Go bin directory (`$HOME/go/bin`).

#### Step 3: Ensure your Go bin directory is in your PATH

If you have never configured your Go path before, add this line to your shell configuration file (`~/.bashrc` or `~/.zshrc`):

```bash
export PATH="$HOME/go/bin:$PATH"
```

Reload your shell configuration:

```bash
source ~/.bashrc   # or: source ~/.zshrc
```

Verify the installation:

```bash
gx --version
```

_(Alternatively, if you prefer not to use `~/go/bin`, you can run `make build` and manually move `./gx` to `/usr/local/bin/gx` with `sudo mv gx /usr/local/bin/`)_.

---

### Updating gx to the Latest Version

When a new version is released:

- **If you installed from source:**
  ```bash
  cd /path/to/gx
  git pull origin develop
  make install
  gx --version
  ```
- **If you downloaded a binary:**
  Download the latest archive from [GitHub Releases](https://github.com/fathss/gx/releases), extract it, and replace the old `gx` file in `/usr/local/bin/` (or your Windows directory).

---

### Shell Auto-Completion (Optional)

Enable Tab completion for commands and flags in your shell:

- **Bash (Linux):**
  ```bash
  gx completion bash | sudo tee /etc/bash_completion.d/gx > /dev/null
  ```
- **Zsh:**
  ```bash
  gx completion zsh > "${fpath[1]}/_gx"
  ```
- **Fish:**
  ```bash
  gx completion fish > ~/.config/fish/completions/gx.fish
  ```

Restart your terminal or reload your shell profile to apply completion.

---

## Quick Start

```bash
cd your-repo

# 1. Initialize configuration (auto-detects remote and default branch)
gx init

# 2. Check current repository snapshot
gx status

# 3. Pull latest base branch updates into your feature branch
gx sync

# 4. Review, stage, and commit changes with safety guards
gx save -m "feat: implement user authentication"

# 5. Push feature branch and generate PR creation URL
gx ship

# 6. Prune local and remote branches already merged into base
gx clean
```

---

## Core Commands

| Command         | Description                                                          | Documentation                                      |
| --------------- | -------------------------------------------------------------------- | -------------------------------------------------- |
| **`gx init`**   | Auto-detect remote/default branch and seed `.gx/config`              | [docs/commands/init.md](docs/commands/init.md)     |
| **`gx sync`**   | Stash, fetch, rebase/merge base branch, and restore stash cleanly    | [docs/commands/sync.md](docs/commands/sync.md)     |
| **`gx save`**   | Preview changes, block sensitive files, and commit                   | [docs/commands/save.md](docs/commands/save.md)     |
| **`gx ship`**   | Push branch with lease, enforce branch protection, and print PR link | [docs/commands/ship.md](docs/commands/ship.md)     |
| **`gx status`** | Synthesized 6-section human-readable repository snapshot             | [docs/commands/status.md](docs/commands/status.md) |
| **`gx clean`**  | Prune merged local and remote branches safely                        | [docs/commands/clean.md](docs/commands/clean.md)   |
| **`gx config`** | Inspect and update configuration settings                            | [docs/commands/config.md](docs/commands/config.md) |

---

## Global Flags

- **`-v, --verbose`**: Print every underlying git command executed under the hood.
- **`--version`**: Display the installed gx version.
- **`-h, --help`**: Help for any command.

---

## Configuration

Persistent repository settings are stored in `.gx/config`:

```json
{
  "remote": "origin",
  "defaultBranch": "develop",
  "syncStrategy": "rebase",
  "sensitivePatterns": [".env", "*.pem", "*secret*", "*.key"],
  "protectedBranches": ["main", "master", "develop"]
}
```

View and modify settings using `gx config`:

```bash
gx config                          # View all settings
gx config syncStrategy merge       # Switch to merge strategy
gx config sensitivePatterns *.key  # Append sensitive pattern
```

Read the full specification in [Configuration Concept](docs/concepts/configuration.md).

---

## Documentation

Full documentation is available in the [`docs/`](docs/) directory:

- **[User Guide](docs/README.md#user-guide)**: Command references, flags, and usage examples.
- **[Concepts & State Machines](docs/concepts/sync-machine.md)**: Sync state machine (Q1–Q6) and pause recovery architecture.
- **[Contributor Guide](docs/internal/architecture.md)**: Strict architectural layering and Git domain abstractions.

---

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on code style, architectural boundaries, and running the test suite (`make test`).

---

## License

This project is licensed under the MIT License — see the [LICENSE](LICENSE) file for details.
