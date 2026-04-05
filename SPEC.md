# VCS Tool Specification

## Overview

`vcs` is a unified command-line interface that abstracts over multiple version
control systems. It provides a single set of subcommands that translate to
the native commands of whichever VCS is active in the current directory.

## Design Goals

1. **Unified interface**: Same subcommand names across Git, Mercurial, and Jujutsu.
2. **Speed**: Compiled Go binaries with minimal overhead. Uses `chg` over `hg`,
   `syscall.Exec` to avoid extra processes, and `.vcs_cache` to skip repeated detection.
3. **Portability**: Works in any shell (bash, fish, zsh, etc.) since these are
   standalone binaries, not shell functions.
4. **Cache compatibility**: Reads and writes `.vcs_cache` files in the same format
   as the original bash implementation, allowing gradual migration.

## Architecture

```
vcs (dispatcher)
 |
 +-- detects VCS via vcsdetect package
 +-- execs vcs-{git,hg,jj} via syscall.Exec
      |
      +-- translates subcommand to native VCS commands
      +-- runs native VCS via os/exec
```

### Packages

| Package | Purpose |
|---------|---------|
| `vcsdetect` | VCS detection: walks directory tree, reads/writes `.vcs_cache`, detects backend and hosting |
| `runner` | Subprocess execution helpers: `Run`, `Exec`, `ExitCode`, `FindCommand` |
| `cmd/vcs` | Main dispatcher binary |
| `cmd/vcs-git` | Git subcommand translations |
| `cmd/vcs-hg` | Mercurial subcommand translations |
| `cmd/vcs-jj` | Jujutsu subcommand translations |

## VCS Detection

Detection proceeds as follows:

1. Check for `.vcs_cache` in the current directory. If found, read it.
2. Otherwise, walk up the directory tree looking for VCS markers:
   - `.jj` → jj (highest priority)
   - `.hg` → hg
   - `.git` → git
   - `.citc` or `.p4config` → g4
3. Detect backend (e.g. read `.jj/repo/store/type` for jj).
4. Detect hosting by parsing the git remote origin URL from `.git/config`.
5. Write `.vcs_cache` (best-effort, ignores write failures on read-only filesystems).

### Priority

When multiple VCS markers exist in the same directory, jj takes priority over
hg, which takes priority over git. This matches the common case of jj wrapping
a git repo (colocated mode).

### Cache Format

Two lines, no trailing whitespace:

```
<vcs> <backend> <hosting>
<rootdir>
```

- `<vcs>`: `git`, `hg`, `jj`, or `g4`
- `<backend>`: `git`, `piper`, etc. Use `-` if empty.
- `<hosting>`: `github`, `gitlab`, `bitbucket`, `sourcehut`, `gerrit`. Use `-` if empty.
- `<rootdir>`: Absolute path to repository root. May contain spaces.

Example:
```
jj git github
/home/user/my project
```

## Mercurial: chg Support

`vcs-hg` uses `chg` (the Mercurial command server client) when it is found on
`PATH`. This avoids the Python startup overhead on each invocation.

The `--hg-path=PATH` flag overrides auto-detection, allowing callers to cache
the result of the chg lookup:

```sh
HG=$(vcs-hg --help 2>&1 | ...)  # or: which chg || which hg
vcs --hg-path=$HG status
```

## Subcommand Translation

Each `vcs-*` binary implements a `dispatch` function mapping unified subcommand
names to native VCS commands. The mapping is a simple `switch` statement.

### Completeness requirement

**Every VCS backend must handle every command in the canonical list**
(`commands.go`). This is enforced by `TestAllCommandsHandled` in each backend's
test suite. When adding a new command, you must add it to `commands.go` and to
the `dispatch` function of every `vcs-*` binary. If a command doesn't apply to
a given VCS, the handler should either be a no-op (return nil) or return a
clear "not supported" error.

### Listing commands

All binaries (`vcs`, `vcs-git`, `vcs-hg`, `vcs-jj`) support
`--list-commands`, which prints one command per line. This is intended for shell
integration, e.g.:

```sh
for cmd in $(vcs --list-commands); do
    eval "$cmd() { vcs $cmd \"\$@\"; }"
done
```

### Special behaviors

Some subcommands have non-trivial translation logic:

- **`commit`/`amend` (git)**: If no file arguments are given, `--all` is added
  automatically (commit all tracked changes). Flags like `-m`, `-C`, `-c`, `-F`,
  `-t` that take a value are handled correctly.
- **`review`/`upload` (git, jj)**: Pushes, then creates a GitHub PR via `gh` CLI
  if available. Supports `-r`/`--reviewer` flags.
- **`status` (jj)**: Only shows status for undescribed commits (jj's equivalent
  of uncommitted changes).
- **`fastforward`/`pull`/`push` (jj)**: Behavior varies based on backend (git vs piper).

### No-ops

Some commands are intentional no-ops for certain VCS:

- `addremove` in jj (files are auto-tracked)
- `branch` in jj (no concept of current bookmark)
- `evolve` in jj (automatic descendant rebasing)
- `commitforce` in jj (no hooks to bypass)

### Unsupported

Some commands return an error for certain VCS:

- `evolve` in git (no automatic obsolescence)
- `histedit` in jj (use squash/split/edit instead)
- `review` in hg (not implemented)

## Dispatcher (vcs)

The `vcs` binary:

1. Parses `--vcs`, `--hg-path`, and `--list-commands` flags.
2. Handles special subcommands (`detect`, `rootdir`, `backend`, `hosting`,
   `clearcache`, `--list-commands`).
3. For all other subcommands, detects VCS and uses `syscall.Exec` to replace
   itself with the appropriate `vcs-*` binary. This means no extra process
   remains running.

## Hosting Detection

Hosting platform is detected by parsing the git remote `origin` URL from
`.git/config` (or `.jj/repo/store/git/config` for jj). String matching is used:

| URL contains | Hosting |
|-------------|---------|
| `github.com` | `github` |
| `gitlab.com` or `gitlab.` | `gitlab` |
| `bitbucket.org` | `bitbucket` |
| `sr.ht` | `sourcehut` |
| `googlesource.com` | `gerrit` |

This avoids running `git remote get-url origin` as a subprocess.

## Exit Codes

- `0`: Success
- Non-zero: Mirrors the exit code of the underlying VCS command.
- If the VCS command cannot be found or executed, exit code `1`.
