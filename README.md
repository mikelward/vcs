# vcs

A unified command-line interface for multiple version control systems.
Compiled Go binaries that provide a consistent set of subcommands across
Git, Mercurial, and Jujutsu.

Originally a set of bash shell functions; reimplemented in Go for speed
and portability (works in bash, fish, zsh, or any shell).

## Supported VCS

| VCS | Binary | Notes |
|-----|--------|-------|
| [Git](https://git-scm.com/) | `vcs-git` | |
| [Mercurial](https://www.mercurial-scm.org/) | `vcs-hg` | Uses `chg` when available for faster startup |
| [Jujutsu](https://github.com/jj-vcs/jj) | `vcs-jj` | Supports both git and piper backends |

## Usage

```
vcs <subcommand> [args...]
```

The `vcs` command auto-detects the VCS for the current directory (caching
the result in `.vcs_cache`) and dispatches to the appropriate `vcs-*` binary.

```
vcs status
vcs commit -m "fix bug"
vcs graph
vcs diff foo.go
```

You can also call the per-VCS binaries directly:

```
vcs-git status
vcs-hg --hg-path=/usr/bin/chg log
```

### Flags

| Flag | Description |
|------|-------------|
| `--vcs=NAME` | Skip auto-detection; use the given VCS (`git`, `hg`, `jj`). |
| `--hg-path=PATH` | Path to `hg` or `chg` binary (passed through to `vcs-hg`). Useful for callers that cache the lookup. |
| `-n`, `--dry-run`, `--simulate` | Print the underlying VCS command to stderr instead of running it. Must appear before the subcommand. Can also be toggled via the `VCS_DRY_RUN` environment variable. |
| `--list-commands` | Print all supported subcommand names, one per line. Useful for shell integration (see below). |

Example:

```
$ vcs -n commit -m "fix bug"
+ git commit -m 'fix bug' --all
```

### Special subcommands

| Subcommand | Description |
|------------|-------------|
| `detect` | Print the detected VCS name. |
| `rootdir` | Print the repository root directory. |
| `backend` | Print the VCS backend (e.g. `git` for jj-on-git). |
| `hosting` | Print the hosting platform (e.g. `github`). |
| `prompt-info` | Print all prompt info (project, subdir, branch, status, fetch_stale) in one invocation. See `--format`, `--color`. |
| `prompt-line` | Print the full preprompt first line (hostname, shpool tag, directory/VCS info, auth warning) in one invocation. See `--hostname`, `--production`, `--shpool`, `--color`, `--no-ssh`, `--auth-cache`, `--auth-cache-ttl`. |
| `clearcache` | Remove `.vcs_cache` files under the current directory. |

## Commands

All subcommands work across all supported VCS, translating to the native
equivalent. Some commands are no-ops where the concept doesn't apply
(e.g. `addremove` in jj, which auto-tracks files).

### Everyday commands

| Command | Description | Git | Hg | Jj |
|---------|-------------|-----|----|----|
| `status` | Show working copy status | `git status --short` | `hg status` | `jj diff --summary` (if undescribed) |
| `commit` | Record changes | `git commit` | `hg commit` | `jj commit` |
| `amend` | Amend the current commit | `git commit --amend` | `hg amend` | `jj squash` |
| `diff` / `diffs` | Show changes | `git diff` | `hg diff` | `jj diff` |
| `diffstat` | Show change statistics | `git diff --stat` | `hg diff --stat` | `jj diff --stat` |
| `add` | Track files | `git add --intent-to-add` | `hg add` | `jj file track` |
| `addremove` | Track new, remove missing | `git add --all` | `hg addremove` | (no-op, auto-tracked) |
| `graph` | Show commit graph | `git log --graph` | `hg log --graph` | `jj log` |
| `changelog` | One-line log | `git log --oneline` | `hg log --template` | `jj log --template` |
| `show` | Show a commit | `git show` | `hg export` | `jj show` |

### Navigation

| Command | Description | Git | Hg | Jj |
|---------|-------------|-----|----|----|
| `checkout` / `goto` | Switch to revision | `git checkout` | `hg checkout` / `hg update` | `jj new` |
| `next` | Move to child commit | children search | `hg update -r min(children(.))` | `jj next` |
| `prev` | Move to parent commit | `git checkout HEAD~` | `hg update -r .^` | `jj prev` |
| `branch` | Print current branch | `git rev-parse --abbrev-ref` | `hg branch` | (no-op) |
| `branches` | List branches | `git branch` | `hg branches` | `jj bookmark list` |
| `base` | Show current commit summary | `git log -1 --oneline` | `hg log -r .` | `jj log -r @\|@-` |
| `map` | Show base or graph | base if at tip, else graph | same | same |

### History editing

| Command | Description | Git | Hg | Jj |
|---------|-------------|-----|----|----|
| `reword` | Edit commit message only | `git commit --amend --allow-empty` | `hg commit --amend -e` | `jj describe` |
| `describe` | Edit commit message | `git commit --amend --only` | `hg commit --amend` | `jj describe` |
| `squash` | Squash commits | `git merge --squash` | `hg fold` | `jj squash` |
| `split` | Split a commit | `git rebase -i` | `hg split` | `jj split` |
| `drop` | Remove a commit | `git rebase --onto` | `hg prune` | `jj abandon` |
| `graft` / `pick` | Copy a commit | `git cherry-pick` | `hg graft` | `jj duplicate` |
| `rebase` | Rebase commits | `git rebase` | `hg rebase` | `jj rebase` |
| `histedit` / `diffedit` | Interactive history edit | `git rebase -i` | `hg histedit` | `jj diffedit` |
| `undo` | Undo last operation | `git reset --mixed HEAD~` | `hg undo` | `jj undo` |
| `unamend` | Undo last amend | `git reset --mixed HEAD@{1}` | `hg unamend` | `jj undo` |
| `uncommit` | Undo commit, keep changes | `git reset --soft HEAD~` | `hg uncommit` | `jj squash --from @-` |

### File operations

| Command | Description | Git | Hg | Jj |
|---------|-------------|-----|----|----|
| `copy` | Copy file (VCS-aware) | `cp` + `git add` | `hg copy` | `cp` |
| `move` / `rename` | Rename file | `git mv` | `hg rename` | `mv` |
| `remove` / `rm` | Remove file | `git rm` | `hg remove` | `rm` + `jj file untrack` |
| `restore` | Restore file to committed state | `git checkout --` | `hg revert` | `jj restore` |
| `revert` | Revert all changes | `git reset --hard` | `hg revert` | `jj revert` |
| `ignore` | Add to ignore file | append to `.gitignore` | append to `.hgignore` | append to `.gitignore` |
| `track` | Track file | `git add --intent-to-add` | `hg add` | `jj file track` |
| `untrack` | Stop tracking file | `git rm --cached` | `hg forget` | `jj untrack` |

### Remote operations

| Command | Description | Git | Hg | Jj |
|---------|-------------|-----|----|----|
| `pull` | Fetch and update | `git pull --rebase` | `hg pull --update --rebase` | `jj git fetch` / `jj sync` |
| `push` | Push changes | `git push` | `hg push` | `jj git push` / `jj upload` |
| `fastforward` | Fast-forward only | `git pull --ff-only` | `hg sync --tool=internal:fail` | `jj git fetch` |
| `incoming` | Show what would be pulled | `git log HEAD..@{upstream}` | `hg incoming` | `jj op log` |
| `outgoing` | Show what would be pushed | `git log @{upstream}..HEAD` | `hg log -r draft()` | `jj log -r mutable()` |
| `pending` | Show uncommitted/unpushed | outgoing or status | `hg status` | `jj log -r mutable()` |
| `review` / `upload` | Push and create PR | `git push` + `gh pr create` | (not supported) | `jj git push` + `gh pr create` |
| `submit` | Push to remote | `git push` | `hg submit` | `jj git push` / `jj submit` |

### Inspection

| Command | Description |
|---------|-------------|
| `annotate` / `blame` | Show line-by-line authorship |
| `changed` | List changed files |
| `at_tip` | Check if working copy is at branch tip |
| `fetchtime` | Unix timestamp of last fetch |
| `unknown` | List untracked files |

### Other

| Command | Description |
|---------|-------------|
| `absorb` | Absorb changes into relevant commits |
| `commitforce` | Commit bypassing hooks |
| `evolve` | Evolve obsolete commits (hg only) |
| `fix` / `lint` | Run code formatters/linters |
| `precommit` | Run pre-commit hook |
| `presubmit` | Run pre-push hook |
| `recommit` | Amend commit |
| `submitforce` | Push bypassing hooks |
| `uploadchain` | Upload chain of commits |

## Cache

VCS detection results are cached in `.vcs_cache` in the current directory.
Format (2 lines):

```
<vcs> <backend> <hosting>
<rootdir>
```

Fields use `-` as a sentinel for empty values. Example:

```
git git github
/home/user/myrepo
```

Use `vcs clearcache` to remove cache files.

## Building

Requires Go 1.21 or later.

```
make          # build all binaries
make test     # run tests
make install  # install to PREFIX/bin
make clean    # remove built binaries
```

`make` uses real file targets, so re-running it is free when sources haven't
changed. This makes it safe to run from a login script.

## Installing

For non-root users, `make install` defaults to `~/.local/bin`:

```
make install
```

For root, it defaults to `/usr/local/bin`. Override with:

```
make install PREFIX=/opt/vcs
```

Ensure `PREFIX/bin` is on your `PATH` (most distros include `~/.local/bin`
by default).

## Shell integration

Since these are standalone binaries, they work in any shell. Use
`--list-commands` to generate wrappers for all commands automatically:

**bash/zsh:**
```sh
for cmd in $(vcs --list-commands); do
    eval "$cmd() { vcs $cmd \"\$@\"; }"
done
```

Or pick specific short aliases:
```sh
alias st='vcs status'
alias ci='vcs commit'
alias di='vcs diffs'
alias gr='vcs graph'
```

**fish:**
```fish
for cmd in (vcs --list-commands)
    alias $cmd "vcs $cmd"
end
```
