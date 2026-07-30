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

## Talking to the user

- **One question at a time.** Never stack multiple questions in a single turn —
  ask the most important one, wait for the answer, then ask the next if you
  still need it. A wall of bundled questions is harder to answer than a short
  back-and-forth.
- **Don't interrupt.** Never fire off a question while the user is still
  typing. Let them finish; a half-typed message isn't an invitation to jump in.
- **Keep replies short — don't dump a full page.** Lead with the single most
  important point and stop. If there's more, say the first point and ask
  whether they're ready for the next one rather than emptying everything at
  once.

## Asking questions

- **Ask in chat, never with `AskUserQuestion`.** That's Claude Code's
  multiple-choice question prompt, and it's broken in the Claude mobile app —
  a question asked through it may be unanswerable. Plain chat also keeps the
  question, its context, and the answer in one readable thread.
- **After asking, stop and wait for the answer.** Don't proceed on an assumed
  answer, pick a "recommended" option yourself, or keep working on the part
  the question affects.

## Error handling

- **Don't silently swallow errors.** `_ = err`, a bare `if err != nil {}`, or
  a `recover()` that discards what it caught hides real failures and burns
  hours when something eventually breaks. Every error path needs to do three
  things: **report** the error with enough context to identify the failed call
  (wrap it — `fmt.Errorf("detecting vcs in %s: %w", dir, err)` — so the chain
  survives); **clean up** what the call acquired (`defer` the
  close / cancel / temp-dir removal); and **handle the case explicitly** —
  pick what the caller sees (a sentinel error, a zero value, a non-zero exit)
  rather than letting control fall through. This tool shells out constantly,
  so the common case is a subprocess that exits non-zero: distinguish "not a
  repo / subcommand unsupported here" (a legitimate fallback) from "the VCS
  binary failed" (an error the user must see) instead of collapsing both into
  a silent fallback. If you genuinely want to ignore a specific failure, name
  the reason in a one-line comment and keep it traceable.

## Privacy

- **Never put user data in any artifact that leaves this machine.** That
  includes commit subjects and bodies, PR titles / descriptions / comments,
  review replies, issue text, branch names, code comments, test fixtures, and
  anything else that ends up on GitHub. For a VCS wrapper that means: absolute
  paths containing the user's home directory or real name, private repository
  names and remote URLs, hostnames, commit messages copied out of a private
  repo, and any credential or token that shows up in a remote URL. Use generic
  placeholders (`/home/user/project`, `git@example.com:org/repo.git`) in
  examples, fixtures, and reproductions. If a user-supplied bug report contains
  any of it, paraphrase in the commit / PR — don't quote verbatim. When in
  doubt, ask before pushing.
- **Command output is not one of those artifacts.** Diagnostics, error text and
  progress lines print on the user's own terminal, and naming the repository,
  path or remote is usually the point of the message. Redact only secrets:
  tokens, keys, and passwords embedded in remote URLs. Quoting that output into
  a commit, PR, issue, or fixture republishes it, and the bullet above governs
  again — paraphrase or use a placeholder there.

## Branching

- **These rules assume an `origin` remote.** Without one you can't fetch,
  branch from `origin/main`, push, or open a PR — say so and stop rather than
  improvising a local substitute. **Exception:** in a sandbox that
  intentionally provides no remote Git support (Codex cloud, say), follow the
  normal branch rules from the current `HEAD` — a pre-created working branch
  counts — commit locally, and report that fetch, push, and pull requests are
  unavailable, using the sandbox's own PR handoff if it has one. That exception
  outranks every `origin`-dependent step below it — the merge-cue fetch, cutting
  a branch off `origin/main`, the closing PR link — so work from the current
  `HEAD` and name what wasn't possible instead of faking it. One limit: a merge
  cue needs a base that *contains* the merge, and an offline sandbox can't fetch
  one. Say the follow-up needs a fresh sandbox or a synced checkout rather than
  branching off a `HEAD` whose commits just landed upstream.
- **Branch naming.** Feature branches are prefixed with the agent's own
  short name: `<agent>/<short-topic>` (e.g. `claude/...` for Claude Code,
  `codex/...` for Codex, `cursor/...` for Cursor, etc.). Human contributors
  pick a name that identifies them. The placeholder `<agent>` below stands
  in for whichever prefix you use — don't hard-code `claude/` unless you
  *are* Claude Code.
- **Workflow.** `<agent>/<short-topic>` branch off `origin/main` → PR → merge
  via rebase or squash. One topic per branch. Follow-up work after a merge
  goes on a new branch. Never commit to `main` / `master`.
- **Use `git worktree` when it's available.** Give each branch its own
  worktree instead of switching branches in place, so work in progress on one
  branch isn't disturbed by work on another.
- **One commit per logical surviving change.** Rewrite unmerged commits
  freely (squash, amend, reorder, split). Review-fix noise shouldn't survive
  into `main`.
- `git push --force-with-lease` to your own live feature branch after a
  rebase is routine — don't ask. Confirm before destructive actions on
  shared/merged branches.
- **Merge cue (`merged` / `I merged` / `landed` / merge webhook) runs hygiene
  *before* engaging with the rest of the message:** `git fetch origin`, cut
  a fresh `<agent>/<short-topic>` branch off `origin/main`, announce the switch.
- **Unshallow before answering anything that depends on git history depth.**
  The sandbox clones shallow, so `git rev-list --count`, `git log` past the
  shallow boundary, and blame return wrong answers without warning. If
  `git rev-parse --is-shallow-repository` says `true`, run
  `git fetch --unshallow` first, then re-check — it exits 0 even when
  it deepened nothing, so if `--is-shallow-repository` is still `true`, say the
  history is truncated instead of quoting a count.
- End every reply with the open-PR link (or `.../compare/main...<branch>`
  until a PR exists). Never link to a closed or merged PR — except when the
  reply *is* post-merge follow-up on that PR, where linking it is correct. In an
  offline sandbox with no `origin` there's no URL to end with — say that, rather
  than inventing a link that resolves to nothing.

## Pull requests and reviews

- **"Drive to merge"** is shorthand for the whole loop: open the PR, wait for
  the automatic Codex review, address every review comment — fix it if you
  agree, reply on the thread saying why if you don't — and merge once CI is
  green and Codex has left its thumbs up.
- Open PRs ready for review (not draft) unless asked otherwise.
- **On every push, update the PR title and body** so they describe the full,
  latest state of the branch — not the scope it had when it was opened.
  Re-read the diff against `origin/main` and patch whatever drifted, then
  post the PR link in the chat reply for that push, not only at the end of
  the conversation.
- When a feature has multiple open PRs, list **every** open PR by URL,
  one per line — the "View PR" chip sticks to the first link and hides
  the rest (anthropics/claude-code#46625).
- **Codex is the automated reviewer on this repo** — not Copilot. Its reviews
  are triggered automatically; you don't request them.
- **Address Codex comments automatically — don't wait to be asked.** Read each
  one, decide whether it's a real issue or a false positive, and if it's real,
  fix it in the same PR. Fold the fix into the commit it belongs to (rebase /
  `--fixup`) rather than tacking on an "address review" commit, per the
  one-commit-per-logical-change rule. Group several small fixes into one
  commit when they share a topic.
- **Reply to (and resolve) every addressed comment**, one thread at a time,
  not in bulk. `resolve_review_thread` works — pass the `PRRT_*` thread node
  ID from `pull_request_read` / `get_review_comments` (`review_threads[].id`)
  as `threadId`. A comment's `PRRC_*` node ID fails; they're different
  objects. Order of operations: push the fix commit first, then reply citing
  the new sha, then resolve.
- **Report when Codex finishes reviewing a fresh push** — a one-liner naming
  the SHA and comment count, e.g. `Codex reviewed 87d9f02 — 0 comments`. Tie
  it to the *latest* pushed SHA so a stale review of a superseded commit isn't
  conflated with the current state.
- **Judge every review comment on merit, whoever wrote it.** Verify the claim
  before acting; if it doesn't hold up, reply saying why and decline.
- Never leave a review comment thread silently dismissed. Either reply on
  the thread *or* resolve it. When you think a comment is a false positive,
  say *why* on the thread (one or two sentences). Acknowledgement noise
  is fine and preferred over silence.
- **Skip echo events silently.** `mcp__github__add_reply_to_pull_request_comment`
  / `add_issue_comment` post under whichever GitHub identity backs the MCP
  auth, so a moment after you post a reply the same body comes back as a
  webhook event authored by that identity. That's your own echo, not user
  feedback — continue without a chat-side acknowledgement. The test is "did
  *I* just post this body?", not "who is the author?".
- **Keep watching merged PRs for late review comments.** Reviewers and bots
  routinely comment *after* merge. Stay subscribed and handle each new comment
  per the reply-or-resolve rule. Stop once every comment posted on or after the
  merge commit has been answered, or after ~24h of silence.

## Language and spelling

- Use **US English** everywhere people read English: command output and help
  text, commit subjects and bodies, PR titles and descriptions, comments,
  docs (`README.md`, `SPEC.md`), and identifiers — `color` not `colour`,
  `behavior` not `behaviour`, `canceled` not `cancelled`, `gray` not `grey`.
  Underlying VCS spellings stay as those tools spell them.

## CI

- **Report significant CI timing regressions.** After CI finishes on a push,
  compare against recent runs of the same job on the same kind of ref. Only
  call out significant slowdowns (rule of thumb: >25% or >30s on a job under
  ~5min) — don't narrate routine wobble. Name the likely cause: a new
  dependency, a slow new test, cache invalidation. Compare like with like —
  PR against PR, `main` against `main`.

## Cost and reliability

- When recommending new infrastructure or a new external dependency
  (libraries, services, APIs), include a brief dollar-cost estimate and
  note reliability implications: new failure modes, rate limits, added
  latency, extra points of failure. If the impact is effectively zero,
  say so explicitly rather than omitting the note.
