# Contributing to gx

Thank you for your interest in contributing to `gx`!

gx is designed with a strict layered architecture and high safety guarantees. Please follow these guidelines when contributing.

---

## Getting Started

### Prerequisites

- **Go**: 1.21 or later
- **Git**: 2.26 or later
- **Bash**: Required for running the integration test suites

### Development Setup

Clone the repository and build the binary:

```bash
git clone https://github.com/fathss/gx.git
cd gx

# Build binary to root directory
make build

# Run tests
make test
```

---

## Architectural Invariants

Every contribution must follow the layering rules described in [Architecture Documentation](docs/internal/architecture.md):

```
cmd  →  internal/workflow  →  internal/git  →  internal/runner
```

1. **`cmd/`**: Entrypoints only. Parses flags and arguments, loads configuration, and delegates to a single workflow function. Never execute git commands or business logic directly here.
2. **`internal/workflow/`**: Business logic orchestration. Converts any external failures into `cli.Error{Message, Hint}`.
3. **`internal/git/`**: One function per Git operation. Takes a `runner.Executor` and returns raw errors. Never prints to stdout/stderr and never constructs `cli.Error`.
4. **`internal/runner/`**: Subprocess execution. Wraps process failures in `CommandError` preserving stderr and exit codes.
5. **No data loss invariant**: No command may leave the repository in an unrecoverable or silently corrupted state.

---

## Testing

gx relies on integration test suites written in Bash simulating realistic Git repositories under `/tmp`:

```bash
# Run all suites (parallel across CPU cores by default)
make test
# or
bash tests/run-all.sh

# Run with custom concurrency or serial mode
bash tests/run-all.sh -j 2           # constrained concurrency (2 workers)
bash tests/run-all.sh --serial       # sequential mode with live streaming output

# Run a single suite
bash tests/gx-sync-test.sh

# Run a single targeted test within a suite (instant filter)
bash tests/gx-sync-test.sh 05        # matches test function or label containing "05"
TEST_FILTER=abort bash tests/gx-sync-test.sh
```

When adding new flags or workflows, ensure:

- Existing tests pass without regressions (`make test`).
- New behaviors include tests covering success cases, edge cases, and safety invariants.
- **Do NOT create Go test files (`*_test.go`)**: All testing in gx is strictly bash-based under `tests/`.
- For new suites requiring Git repositories, use `copy_template "<name>" <setup_func>` from `tests/lib.sh` to benefit from in-memory golden template caching (`cp -a`).

---

## Documentation

If your changes modify command behavior, arguments, flags, or configuration keys:

- Update the relevant command guide in `docs/commands/<cmd>.md`.
- Update architectural or domain references in `docs/internal/` or `docs/concepts/`.

---

## Pull Request Process

1. Fork the repository and create your branch from `develop`.
2. Ensure `make test` and `go vet ./...` pass cleanly.
3. Keep commits atomic and descriptive following Conventional Commits (e.g. `feat: ...`, `fix: ...`, `docs: ...`).
4. Submit a Pull Request targeting the `develop` branch.
