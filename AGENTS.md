# Agent Instructions

Keep this file short and concrete — add a new rule the first time something
bites, not the third.

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

- **Always add tests.** New functionality gets a test that exercises its
  behavior; bug fixes get a regression test that fails before the fix.
- **Fix any preexisting test failures as the *first* commit of the series.**
  Don't stack new work on a red baseline. If the failure is genuinely
  unrelated and out of scope, say so up front and confirm before skipping it.
- **Don't paper over flaky/racy tests** with `time.Sleep`, retry loops, or
  bumped timeouts. Make the ordering explicit (channels, `sync.WaitGroup`,
  test fixtures that block on real signals) or fix the underlying race.
- **Don't disable a failing check** (`go vet`, `staticcheck`, a test) to make
  it pass — fix the underlying issue.

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

## Branching

- **Workflow.** `claude/<short-topic>` branch off `origin/main` → PR → merge
  via rebase or squash. One topic per branch. Follow-up work after a merge
  goes on a new branch. Never commit to `main` / `master`.
- **One commit per logical surviving change.** Rewrite unmerged commits
  freely (squash, amend, reorder, split). Review-fix noise shouldn't survive
  into `main`.
- `git push --force-with-lease` to your own live feature branch after a
  rebase is routine — don't ask. Confirm before destructive actions on
  shared/merged branches.
- **Merge cue (`merged` / `I merged` / `landed` / merge webhook) runs hygiene
  *before* engaging with the rest of the message:** `git fetch origin`, cut
  a fresh `claude/<short-topic>` branch off `origin/main`, announce the switch.
- End every reply with the open-PR link (or `.../compare/main...<branch>`
  until a PR exists). Never link to a closed or merged PR.

## Pull requests and reviews

- Open PRs ready for review (not draft) unless asked otherwise.
- When a feature has multiple open PRs, list **every** open PR by URL,
  one per line — the "View PR" chip sticks to the first link and hides
  the rest (anthropics/claude-code#46625).
- Watch the review for automated findings and any comments, and proactively
  address them.
- Never leave a review comment thread silently dismissed. Either reply on
  the thread *or* resolve it. When you think a comment is a false positive,
  say *why* on the thread (one or two sentences). Acknowledgement noise
  is fine and preferred over silence.

## Cost and reliability

- When recommending new infrastructure or a new external dependency
  (libraries, services, APIs), include a brief dollar-cost estimate and
  note reliability implications: new failure modes, rate limits, added
  latency, extra points of failure. If the impact is effectively zero,
  say so explicitly rather than omitting the note.
