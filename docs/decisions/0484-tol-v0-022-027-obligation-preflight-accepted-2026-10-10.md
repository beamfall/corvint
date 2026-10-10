# Decision 0484 — expected-fail obligations, witness post-check and preflight (TOL-V0-022..027) accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-10
("accept tol").

## Context

GitHub beamfall/corvint#712 and #713 asked that the obligation ledger handle three things: a
Playwright `test.fail` obligation that confirms a known defect, a witness that refuses credit when
a post-check fails, and a read-only check that a ticket's obligations are named in the spec source
before any work starts. The lane proposed `TOL-V0-022..027` in
`docs/specs/corvint-tasks-obligation-ledger-v0.md`:
- `defectConfirmed` and `mixedExpectedFail` reporting (TOL-V0-022, 023).
- The witness post-check refusal (TOL-V0-024).
- The read-only `preflight` verb (TOL-V0-025..027), which covers the source scan, `--plan` and the
  `--deep` gate run in a temporary worktree.

Decision 0456 did not cover these additions. AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `TOL-V0-022..027` as written, after six independent review rounds. The following
record the acceptance:
- the spec's intent line, digest status, requirement statuses and traceability rows;
- the matching entries in `docs/specs/README.md` and `docs/specs/INDEX.json`;
- the lane's build log, `docs/build-log/2026-10-09-gh712-713-obligation-preflight.md`.

## Limits

This decision settles intent only. The delivery status stays experimental. The evidence is focused
tests on synthetic json reports and fixture repositories.

The following are `NOT_RUN`:
- a live Playwright 1.63 `test.fail` fixture;
- preflight against a real Playwright suite.

The accepted TOL-V0-027 limits stand:
- A process that leaves its gate's process group, for example with `setsid`, is not tracked.
- Creating the temporary worktree, including smudge filters, is not bounded.
- `SIGKILL` of preflight can leave the worktree behind; `git worktree prune` repairs it.

Running preflight automatically at pool acquire or claim remains a follow-up. No native ticket is
completed by this decision.

## Rollback

To roll back the decision only, revert it and return the TOL-V0-022..027 status text to proposed.

To withdraw the behavior as well, also revert the #712/#713 change:
- the expected-fail lists and the witness post-check;
- the `preflight` verb and its store helpers;
- the `UNREADABLE_SPEC_PATH` finding.

Then regenerate `docs/specs/REQUIREMENTS.tsv`. Stores are unchanged: the additions write no new
stored state.
