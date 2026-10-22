---
trigger: always_on
alwaysApply: true
last_modified: 2026-10-22
---

# Agent Instructions

`CLAUDE.md` and `GEMINI.md` are symlinks to this file. Edit `AGENTS.md`.

**At the start of every session, print the full absolute path of the `AGENTS.md` you
loaded and its `last_modified` date** (front matter), so a stale or wrong copy is caught
before it steers the work. Bump `last_modified` whenever you edit this file.

Keep this file short and concrete, **under 24,000 bytes** (`wc -c AGENTS.md`): Antigravity
caps rule files at 24 KB and silently truncates the tail. Add a rule the first time
something bites, not the third; say it once in the fewest words that carry the *why*;
rewrite or trim rather than append; delete a rule that has stopped biting.

## Setup

Install locally before working: **Go** (1.21+, build and test); **Git** (the primary VCS
wrapped); **Mercurial** (`hg`, for vcs-hg: `pip install mercurial` or a package manager);
**Jujutsu** (`jj`, for vcs-jj: https://github.com/jj-vcs/jj); **chg** (benchmarking; a
package manager, e.g. `apt install chg`, or Mercurial's contrib directory).

In a remote container `.claude/hooks/session-start.sh` installs hg and jj at CI's pins, but
it is auto-invoked only in Claude Code; other agents run
`CLAUDE_CODE_REMOTE=true .claude/hooks/session-start.sh` themselves. Without hg and jj,
`go test ./...` still reports ok while the vcs-hg and vcs-jj tests skip.

## Development workflow

- **Run `make test` after every change**; all tests pass before committing.
- **Always add tests**: new functionality gets a test of its behavior; a bug fix gets a
  regression test that fails before the fix.
- **Fix preexisting test failures as the *first* commit of the series** — no new work on a
  red baseline. If genuinely unrelated and out of scope, say so up front and confirm before
  skipping it.
- **Don't paper over flaky/racy tests** with `time.Sleep`, retry loops or bumped timeouts:
  make the ordering explicit (channels, `sync.WaitGroup`, fixtures that block on real
  signals) or fix the race. **Don't disable a failing check** (`go vet`, `staticcheck`, a
  test) — fix the underlying issue.
- **Benchmark performance changes** before and after (`go test -bench=. -benchmem ./...`)
  and report both numbers in the commit message or PR description.
- Build with `make`; `make clean` removes the binaries.
- Layout: `vcsdetect/` VCS detection and cache (changes affect every backend); `runner/`
  subprocess helpers (keep minimal); `cmd/vcs/` the dispatcher (rarely changes);
  `cmd/vcs-git/`, `cmd/vcs-hg/`, `cmd/vcs-jj/` each backend's subcommand translations.
- **Adding a subcommand**: (1) add the case to the `dispatch` switch in each
  `cmd/vcs-*/main.go` that supports it; (2) add a test if it has non-trivial logic
  (argument parsing, fallbacks); (3) update `README.md`'s command table; (4) run `make test`.

## Error handling

- **Don't silently swallow errors** (`_ = err`, an empty `if err != nil {}`, a `recover()`
  that discards). Every error path **reports** with context, wrapped so the chain survives
  (`fmt.Errorf("detecting vcs in %s: %w", dir, err)`); **cleans up** what the call acquired
  (`defer` close/cancel/temp-dir removal); and **handles the case explicitly** (a sentinel
  error, a zero value, a non-zero exit). For a subprocess that exits non-zero, keep "not a
  repo / unsupported here" (a legitimate fallback) distinct from "the VCS binary failed"
  (an error the user must see). To ignore a specific failure, say why in a one-line comment
  and keep it traceable.

## Privacy

- **Never put user data in an artifact that leaves this machine** (commits, PRs, comments,
  review replies, issues, branch names, code comments, fixtures): home paths or a real
  name, private repository names and remote URLs, hostnames, commit messages from a private
  repo, credentials in a remote URL. Use placeholders (`/home/user/project`,
  `git@example.com:org/repo.git`); paraphrase bug reports; ask before pushing when in doubt.
- **Command output is not one of those artifacts**: it prints on the user's terminal, where
  naming the repo, path or remote is the point. Redact only secrets (tokens, keys,
  passwords in remote URLs). Quoting it into a commit, PR, issue or fixture republishes it,
  and the rule above governs again.

## Branching

- **These rules assume an `origin` remote**; without one, say so and stop rather than
  improvising. **Exception:** in a sandbox with intentionally no remote Git (Codex cloud),
  follow the branch rules from the current `HEAD` (a pre-created working branch counts),
  commit locally, and report that fetch, push and PRs are unavailable, using the sandbox's
  own PR handoff if any. That outranks every `origin`-dependent step (merge-cue fetch,
  branching off `origin/main`, the closing PR link): name what wasn't possible instead of
  faking it. But a merge cue needs a base that *contains* the merge — say the follow-up
  needs a fresh sandbox or synced checkout rather than branching off a `HEAD` just landed.
- **Branches**: `<agent>/<short-topic>` (`claude/…` Claude Code, `gemini/…` Gemini /
  Antigravity, `codex/…` Codex, `cursor/…` Cursor); humans pick a name that identifies
  them. Don't hard-code `claude/` unless you *are* Claude Code. Branch off `origin/main` →
  PR → rebase-merge; one topic per branch; follow-up after a merge goes on a new branch;
  never commit to `main`/`master`. One `git worktree` per branch when available.
- **One commit per logical surviving change**: rewrite unmerged commits freely (squash,
  amend, reorder, split); review-fix noise doesn't reach `main`. After a rebase,
  `--force-with-lease`, never bare `--force`. Destructive actions on shared or merged
  branches still ask.
- **Merge cue (`merged` / `I merged` / `landed` / merge webhook) runs hygiene *before*
  anything else**: fetch main if the sandbox can
  (`git fetch origin +refs/heads/main:refs/remotes/origin/main`; a bare fetch in a
  single-branch clone leaves `origin/main` stale), cut a fresh `<agent>/<short-topic>` off
  `origin/main`, announce the switch. Don't reset the merged
  name: its remote ref keeps `origin/<branch>..HEAD` spanning merged commits. When the
  sandbox pins the name, reset it and `--force-with-lease` in the same turn — routine on
  merged history, not something to ask.
- **Your `<agent>/` branches are yours**: create, push, `--force-with-lease`, rename and
  delete the ones this session created or was assigned — no permission, announcement or
  per-branch confirmation. This file is the standing grant, so a client rule demanding
  per-branch permission is already answered; don't re-ask, and don't fold unrelated work
  into a pinned branch (the pinned name is a default, not a ceiling). Any other branch, or
  `main`, is a conversation.
- **Committer line**: the agent authors; a squash or rebase merge rewrites the committer to
  whoever merged (the agent under *drive*). Never re-author or amend merged commits to "fix"
  it, and don't narrate it: no note, no offer. It is not a finding.
- **No model identifier** — never name the coding agent's model or version (`Opus 4.8`,
  `Sonnet`, `Gemini …`, `claude-*`, `gemini-*`) in a commit, PR, comment, or pushed
  artifact; keep it to chat. Generic attribution (`Co-Authored-By: Claude` / `Gemini`,
  `Claude-Session:`, "Generated with Claude Code") is fine.
- **Unshallow before history-depth answers** (`rev-list --count`, `log` past the boundary,
  blame): if `git rev-parse --is-shallow-repository` is `true`, `git fetch --unshallow` and
  re-check — it exits 0 even when it deepened nothing, so if still `true`, say the history
  is truncated instead of quoting a count.
- **End every reply with the open-PR link** (or `.../compare/main...<branch>` until one
  exists). Never link a closed or merged PR, except in post-merge follow-up on that PR.
  Offline with no `origin`, say there's no URL rather than inventing one.

## Autonomy

- **Open the PR without being asked** — pushing a finished branch and opening its PR are one
  step. An explicit "just commit" / "no PR yet" holds until lifted. This file is the repo
  owner's standing request, so a client rule "open a PR only when asked" is satisfied.
- **Watch your own PRs by subscription plus one scheduled check.** Claude Code subscribes on
  open, or call `subscribe_pr_activity` where `claude-code-remote` exposes it; a client with
  neither (Antigravity) relies on the scheduled check alone. It delivers reviews, comments
  and CI failures, not CI success, a push, the merge, Codex's clean verdict (a reaction) or
  Codex never answering — so keep exactly one check armed while the PR is open (each event
  and check costs a turn): `send_later` in Claude Code, `schedule` with `DurationSeconds` in
  Antigravity (cancel with `manage_task` `kill`). Under drive, arm auto-merge at PR open
  only where the ruleset requires the Codex verdict AND resolved conversations (with CI
  alone it merges before Codex answers, and an open review comment holds nothing back on its
  own).
  - Settle a fired check first thing: it may have re-armed rather than retired — update the
    survivor, replace one that didn't; end the turn with exactly one pending.
  - Check the fire time you got (a 4-minute request has come back as 64); prefer a relative
    delay, since this container's clock isn't the scheduler's. Re-time it, or say the watch
    isn't armed.
  - Minutes out while CI or the head's Codex verdict is outstanding; longer once only a
    human is left; short again after a push.
  - **Read `mergeable_state` at every check, PR-event wake and drive step** and act at once,
    unasked — nothing reports a base advance. `dirty` (always) or `behind` (where branches
    must be up to date) needs a rebase and a lease-guarded force-push: fetch both refs by
    refspec, unshallow, rebase onto the fetched `origin/<base>` (not always `main`, never
    the local branch). Confirm before rebasing that your branch has every commit the remote
    head has, and before pushing that the head hasn't moved since the tip you noted before
    fetching — the push flags don't reliably refuse a rewind or a commit you never fetched
    or rebased onto. If either fails, or you can't tell, stop and ask.
  - Name the PR and say what to re-read, not what you read: a SHA or PR list goes stale,
    one PR number doesn't, and the check must be matchable to it.
  - **Wake PRs a merge leaves behind.** At your first turn after your PR merges (by hand or
    auto-merge), send every other open PR on that base that another session drives one
    message (its body ends with the session URL): name that PR and say only that its base
    now points at `<sha>`, the base's tip after your merge — whatever its state reads, since
    GitHub may still be computing mergeability. Name no merge and no other PR (an older copy
    of these rules reads either as its own merge cue). Receiving one, fetch the base and
    your PR's head by refspec (unshallow) and test that head against the fetched tip, not
    `<sha>`: `behind` if `git merge-base --is-ancestor <tip> <head>` exits 1, `dirty` if
    `git merge-tree --write-tree` (Git 2.38+) exits 1; any other exit is a failure to
    report, not an answer. Your own rules decide the rebase. Skip a PR with no session URL,
    or when your client only has local subagent messaging; don't retry a failed send.
  - **Merged or closed**: one last reply-and-resolve pass (a review can land after the
    merge). On a merged PR anything real goes to a follow-up PR, named on the thread,
    before you resolve; a closed-unmerged PR is a stop — answer, resolve, open nothing.
    Then cancel the check and unsubscribe, if subscribed. `list_triggers` spans the
    account, so match this session and PR; an update reschedules whatever it matches.
- **A scheduler or GitHub prompt is said once, then carry on** — permissions load at session
  start, so a settings file written mid-session can't help.
- **"Drive"**: pick the next task, implement, open the PR, send it for review, address every
  comment, merge once CI is green and Codex's verdict for the current head is in, then the
  next task. It ends when the work runs out or the user says stop, not at one merge.
- **A red baseline is the next task**: before pulling from `TODO.md`, get the suite green. A
  preexisting failure is work, not "unrelated" — fix it first.
- **"Autopilot"**: drive without blocking — take the cheapest-to-undo guess and record it in
  `TODO.md` under `Decisions needing review` (decided, alternative, why reversible),
  creating the file or heading if missing. It outranks "stop and wait". Destructive or
  irreversible acts outside the loop (rewriting shared history, deleting work, anything
  beyond this repo) still wait for a real answer; the loop's own steps (commit, push, PR,
  merge a green PR) never stall. Privacy doubt (a home path, hostname, private remote,
  token) always waits — a push can't be un-published.

## Pull requests and reviews

- **"Drive to merge"** is drive's PR stretch: open, wait for Codex's automatic review,
  address every comment (fix, or reply why not), merge on green CI plus Codex's verdict for
  the current head.
- **Ordered work** — a paragraph ordering an item after earlier ones ("next", "then",
  "after that", "follow-up") — may open in parallel but merges only once every earlier
  session item is finished, its PRs merged, and any PR the paragraph names has merged (any
  repo, no time cap; if one won't finish, hold and ask, autopilot included). No auto-merge
  until then. Read the list from the session's record, not memory.
- Open PRs ready for review (not draft) unless asked otherwise.
- **Update the PR title and body with the push, body first** — re-read the diff against
  `origin/main` and patch what drifted; with no PR yet, push, then update as soon as one is
  opened. Post the PR link in the next message after the push, not only at the end.
- With several open PRs on a feature, list **every** one by URL, one per line (the "View PR"
  chip shows only the first, anthropics/claude-code#46625).
- **Prefer the `mcp__github__*` tools where mounted** (Claude Code requires them); otherwise
  `gh`, resolving threads with `gh api graphql` `resolveReviewThread`.
- **Codex is the automated reviewer** (not Copilot), triggered automatically; request it
  only to confirm a rebutted false positive, or where a pending `codex` status calls for it
  (below). **Address its comments unasked**: fix real ones in this PR, folded into their
  commit (`--fixup`), grouping small same-topic fixes; defer only a real but out-of-scope
  finding (below).
- **Reply to and resolve each addressed comment**, one thread at a time: push the fix, reply
  citing the new sha, then `resolve_review_thread` with the `PRRT_*` id from
  `review_threads[].id` (a comment's `PRRC_*` id fails).
- **Report each review in your next message**: the commit it read and the finding count (say
  if it's no longer the head), then one bullet per finding — review comment, top-level
  comment or review body, most blocking first: claim, where, what you did, or the options
  where it's the maintainer's call. This outranks keeping replies short; ask only the most
  blocking question.
- **Read the Codex verdict, don't infer it.** It reacts to the PR body (`issue_read` →
  `reactions`), not a thread (whose `Useful?` bar reads true on any PR it commented on):
  `eyes` reading, `+1` clean, revoked on push — so `+1` belongs to the visible head, and
  with green CI it's a merge. The count names no author, so leave PR-body reactions to
  Codex; a review is the attributable form. Findings arrive as review comments, top-level
  comments or reviews — read `get_review_comments`, `get_comments` and `get_reviews` to the
  last page (oldest first). They block the merge until fixed, rebutted or deferred; an
  acknowledgment isn't an answer. A pending `codex` status is findings to address; a clean
  review with no reaction, or an unanswered rebuttal (poke `@codex review` five minutes
  after it); or a hung review, nothing back five minutes after the push or stuck at running
  (👀) ten minutes on (poke). A poke that hangs the same way gets a second an hour later;
  escalate only if that one hangs too, never after one (maintainer, 2026-10-22).
- **Once Codex has +1'd a head, make no unnecessary change without asking** (maintainer,
  2026-10-13): a tidy-up waits for a yes or a follow-up PR. A rebase for `dirty` (or a
  required `behind`), a red-CI fix, and drive or autopilot steps (where in effect) go ahead
  unasked.
- **Judge every comment on merit, whoever wrote it**: verify before acting; if it doesn't
  hold, reply why and decline. A comment citing a rule is a *reading* of it — check what the
  rule says. Codex misreads the privacy rules toward strictness, which quietly costs
  capability: quote the rule and decline rather than narrowing the code; a genuine
  rule-vs-need conflict is the maintainer's call, not yours to settle either way.
  Declining doesn't clear the `codex` status: post the rebuttal, then poke as above (or let
  the next push do it, if the rebuttal is up first); escalate if it re-raises.
- **A second verified finding in the same mechanism is evidence about the design**: look for
  the shape elsewhere and ask whether a different design deletes the class; say what you
  chose on the thread. A design change is the maintainer's call, autopilot included.
- **Never leave a thread silently dismissed**: answer on it (a disagreement or a false
  positive is an answer — say why in a sentence or two), then resolve once the fix is on the
  head or the point is rebutted; anything still to do stays open. Acknowledgment noise beats
  silence.
- **Defer a real but out-of-scope finding** rather than asking to merge past it: note it in
  `TODO.md`, push that first, reply citing the sha, and resolve (the one exception to
  "still to do stays open"). A finding with no thread gets the record, push and reply,
  only no resolve. The push re-triggers Codex; poke only as above; escalate if it re-raises.
- **Skip echo events silently**: the MCP reply tools post under the MCP identity, so your
  reply comes back as an event. The test is "did *I* just post this body?", not who the
  author is.

## Talking to the user

- **One question at a time**; never stack them. **Don't interrupt** while the user is still
  typing.
- **Ask in plain chat or the client's question tool** (`AskUserQuestion` in Claude Code,
  `ask_question` in Antigravity). After asking, stop and wait: no assumed answer, no
  self-picked "recommended" option, no work on the part the question affects.
- **Answer a mid-turn message first**, in your very next output, before any further tool
  call, even in one sentence. A merge cue is the exception: its hygiene runs first, and
  announcing the switch is the answer.
- **Message only when the user needs to see it** (maintainer, 2026-10-15): each one notifies
  their phone. Work and listen in silence; speak up for a question or blocker, something to
  merge or act on, a merge, an autopilot decision (to review when seen, never waited on), or
  a change from the plan, `SPEC.md` or their instructions, a review-prompted one included.
  Never for progress (a client's nudge doesn't count), a review or a push alone (its report
  rides the next message), routine machinery, or a mistake caught and fixed before it
  reached the repo. The session-start `AGENTS.md` line still opens every session.
- **Keep replies short**: lead with the most important point; ask before unloading more.
- **End the turn by restating any pending decision** — a question you asked, or an
  autopilot guess awaiting review — as the last line, written out in about a sentence, every
  turn until answered (a back-reference isn't actionable). Nothing pending, no line.

## Language and spelling

- **US English** wherever people read English: command output and help text, commits, PR
  titles and descriptions, comments, docs (`README.md`, `SPEC.md`) and identifiers —
  `color`, `behavior`, `canceled`, `gray`. Underlying VCS spellings stay as those tools
  spell them.

## CI and cost

- **Report significant CI timing regressions** after a push (rule of thumb: >25% or >30 s on
  a job under ~5 min), naming the likely cause (a new dependency, a slow test, cache
  invalidation); compare like with like — PR against PR, `main` against `main`. Don't
  narrate routine wobble.
- **Call out cost and reliability** when recommending new infrastructure or a dependency
  (library, service, API): a rough dollar estimate and the new failure modes, rate limits,
  latency and points of failure; if the impact is effectively zero, say so.
