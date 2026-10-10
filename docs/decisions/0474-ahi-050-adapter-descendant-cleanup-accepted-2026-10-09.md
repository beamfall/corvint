# Decision 0474 — OpenCode and Gemini adapter descendant cleanup (AHI-050) accepted

Date: 2026-10-09. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-09
("AHI-050 accepted"), then, after two review repairs changed the wording, "AHI-050 confirmed" for
the text at c05bf0a4, covering native ticket V1-0371.

## Context

V1-0371 found that a same-group descendant that ignores SIGTERM and closes its stdio outlived an
OpenCode or Gemini adapter timeout or cancellation, and that the accepted `AHI-009` text required a
group SIGKILL after a normal leader exit, when the leader is already reaped and the group ID may be
reused, and counted `EPERM` after the leader's exit as confirmation. The requirement is `AHI-050` in
`docs/specs/agent-harness-integration-v0.md`. AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `AHI-050` as written at c05bf0a4. The adapters decide every group signal in a
`setImmediate` turn after libuv's reap batch and reread the leader's exit fields there; they signal
the group only while the leader is unreaped and hold the event loop for the termination grace
between SIGTERM and SIGKILL (OpenCode 25 ms, with one shared hold for terminations requested in the
same turn; Gemini 10 ms). After a reap they only probe with signal 0, which confirms cleanup only on
`ESRCH`; a normal exit sends no signal, and a surviving descendant or any other answer, including
Darwin `EPERM`, completes as `corvint-process-cleanup-unconfirmed`. Gemini settles one grace after
its group decision without waiting for close, then closes its pipe ends and unreferences the leader.
A foreign in-process `waitpid(-1)` or off-thread reaping voids both signalling safety and honest
reporting and is an unsupported condition. Acceptance evidence is the eleven `V1-0371` tests in
`integrations/host-adapters.test.mjs`; the shipped adapters are OpenCode 0.7.11 and Gemini 0.2.8.
`AHI-050` supersedes `AHI-009`'s post-exit SIGKILL and `EPERM`-as-confirmed text, which now points
to `AHI-050`.

## Limits

This decision settles intent. The evidence is focused tests on Darwin
(`docs/build-log/2026-10-09-v1-0371-adapter-descendant-cleanup.md`). Linux, `make gate` and the
dogfood CEM bind are `NOT_RUN`; whether Bun, which OpenCode embeds, reaps children off the event-loop
thread or outside the poll phase is `NOT_OBSERVED`. Descendants that left the owned group are not
covered. V1-0371 is not completed by this decision.

## Rollback

Revert this decision and the V1-0371 change: the `requestGroupTermination`, `terminateGroup`,
`signalGroup`, `holdUnreaped` and `cleanupConfirmed` changes in
`integrations/opencode/src/runtime.js` and `integrations/gemini-cli/hooks/corvint-hook.mjs`, their
tests, the package version bumps, the `AHI-050` text and the `AHI-009` pointer, then regenerate
`docs/specs/REQUIREMENTS.tsv`. The adapters then again send a group SIGKILL after the leader is
reaped. No store or wire format changes.
