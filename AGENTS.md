# Agent Instructions

## Setup

Install the following tools locally before working on this project:

- **Go** (1.21+): Required to build and test.
- **Git**: Required. The primary VCS this tool wraps.
- **Mercurial (hg)**: Required for testing vcs-hg. Install via `pip install mercurial` or your package manager.
- **Jujutsu (jj)**: Required for testing vcs-jj. Install from https://github.com/jj-vcs/jj.
- **chg**: Required for benchmarking. Mercurial's command server client for faster hg operations. Install via your package manager (e.g. `apt install chg`, or build from the Mercurial contrib directory).

## Development workflow

### Always run tests

Run tests after every change:

```
make test
```

This runs `go test ./...` across all packages. All tests must pass before
committing.

### Benchmarking

When making performance-related changes, benchmark before and after:

```
go test -bench=. -benchmem ./...
```

Report the benchmark results in your commit message or PR description,
including both the before and after numbers.

### Building

```
make        # build all binaries
make clean  # remove built binaries
```

### Code organization

- `vcsdetect/` - VCS detection and cache. Changes here affect all VCS backends.
- `runner/` - Subprocess execution helpers. Keep this minimal.
- `cmd/vcs/` - Main dispatcher. Rarely needs changes.
- `cmd/vcs-git/` - Git subcommand translations.
- `cmd/vcs-hg/` - Mercurial subcommand translations.
- `cmd/vcs-jj/` - Jujutsu subcommand translations.

### Adding a new subcommand

1. Add the case to the `dispatch` switch in each `cmd/vcs-*/main.go` that supports it.
2. Add a test if the subcommand has non-trivial logic (argument parsing, fallbacks, etc.).
3. Update the command table in `README.md`.
4. Run `make test` to verify.
