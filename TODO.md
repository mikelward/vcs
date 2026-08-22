# TODO

## Decisions needing review

Guesses made under autopilot, recorded so nothing decided without the
repository owner silently becomes permanent. Each says what was decided, what
the alternative was, and why it is reversible.

### The fork gap is documented upstream, not fixed here

**Decided:** take the shared codex-review setup as-is, with the fork-pull-request
limitation recorded in `mikelward/codex-review`'s `docs/CONSUMER.md` rather
than fixed.

**Alternative:** hold this conversion until the shared action publishes its
check result against `pull_request.head.sha`, so a fork pull request can
satisfy a required `codex-review-check`.

**Why this way:** the owner confirmed external fork pull requests are not a
case these repositories take today, and the premise is unconfirmed — the
head-associated check here comes from the `push` trigger, which same-repo
pull requests always get. Waiting would strand the conversion behind an
unproven gap, and the three files here are byte-identical template copies, so
the fix belongs upstream once rather than as a local edit that would fail the
pin.

**Reversible:** entirely. When the remedy lands upstream, this repository
re-copies `templates/` and gets it for free. The remedy is written out in full
there — the scope to use, the trap to avoid — so implementing it later is a
fresh pull request, not a rediscovery.

### This pull request merges without a `codex` verdict, because it cannot have one

**Decided:** merge the conversion without a clean `codex` verdict on its head.
The status on the previous head read "Codex left findings on this head" — that
finding being the fork gap above, answered, resolved and documented — and the
current head has no status at all.

**Alternative:** keep waiting for a clean verdict.

**Why this way, and the reason is structural rather than a judgment about
Codex.** This pull request is what puts `codex-review.yml` on the default
branch. Until it merges the sweep is not there to be run: `pull_request_target`
and `schedule` both take their definition from the default branch, so neither
fires for this pull request. Confirmed by looking rather than assumed — every
sweep run in this repository so far was triggered by
`pull_request_review_comment`, which resolves against the merge ref and is the
only route that works before the merge. So no sweep observes this head, no
`codex` status is written for it, and waiting for one is waiting for something
that cannot arrive. Nothing is bypassed that was protecting anything: this
repository's ruleset does not require `codex` yet, and requiring it is
deliberately a follow-up.

Every remaining consumer conversion has the same property — the first
installation of a gate cannot be gated by itself.

**Reversible:** the merge is revertable, and the gate it is ahead of is not
yet in force. What is *not* reversible is the precedent, so it is written down
here rather than left as a habit: merging past a `codex` gate is for a finding
already answered and recorded, never for one nobody has read.

## Later

- Add the three ruleset settings this setup expects: require `codex` (not
  `sweep`), require `codex-review-check / codex-review-check`, and require
  branches to be up to date before merging. All three are explained in the
  shared `docs/CONSUMER.md`.

## Review and merge gates

- [ ] **Add `zizmor` to the ruleset's required set** once it has reported
      on a pull request: the new zizmor workflow runs unfiltered on every
      PR precisely so it can be required (a paths-filtered workflow
      creates no check run at all on a non-matching PR, which a ruleset
      waits on forever) — the posture piloted in mikelward/lanes and
      mikelward/ci-commit-artifact. `repo-rules mikelward/vcs` with no
      arguments applies the standard `lanes codex zizmor` set.

- [ ] Verify the settings half of the fleet's bar — every repository works
      the same: comprehensive automated review, required merge gates, and
      auto-merge. The workflow files (CI and the codex-review set) are all
      present here; what git cannot show, and the 2026-08-18 audit could
      not verify, is the settings half: a ruleset on the default branch
      requiring the CI gate, the `codex` status, conversation resolution
      and up-to-date branches, and the auto-merge setting enabled.
