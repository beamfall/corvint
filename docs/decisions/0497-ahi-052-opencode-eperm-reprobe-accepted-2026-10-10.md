# Decision 0497 — OpenCode EPERM re-probe (AHI-052) accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-10
("accept AHI-052").

## Context

V1-1116 reported that the OpenCode host adapter completed as `corvint-process-cleanup-unconfirmed`
on hosted macos-15 release-gates runs since V1-0371 narrowed cleanup confirmation to `ESRCH`
(`AHI-050`, decision 0474). The lane proved the cause locally on Darwin: a process group whose
members are all unreaped zombies answers `EPERM` to `kill(-pid, 0)`, and libuv runs due timers
before the poll phase that reaps the leader, so a stalled termination turn probes while the
SIGTERM-killed leader is still a zombie. The lane proposed `AHI-052` in
`docs/specs/agent-harness-integration-v0.md`. AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `AHI-052` as written. It amends `AHI-050` for the OpenCode adapter only:

- When the cleanup probe `kill(-pid, 0)` answers `EPERM`, the adapter repeats that signal-0 probe,
  each repeat a timer at least 5 ms after the last, until it answers anything other than `EPERM`
  or 40 repeats have run. It sends no other signal to the group meanwhile.
- Only `ESRCH` confirms cleanup. `EPERM` on the last repeat, or any other answer, completes as
  `corvint-process-cleanup-unconfirmed`, as in `AHI-050`.
- OpenCode completion moves from 100 ms after the request to 100 ms plus the `EPERM` repeats
  (about 200 ms on an unloaded host; a stalled event loop stretches them).

## Limits

This decision settles intent only. The evidence is the focused host-adapter tests named in the
lane's build log, including a Darwin test that failed 5 of 5 runs on the base. Hosted macos-15
qualification (a release-gates run passing `host-adapter-test` and `TestHostAdapterJavaScriptHosts`)
is still required before V1-1116 completes. Other hosts are unchanged; the Gemini CLI hook's
single probe is tracked separately as V1-1122. The full suite and `make gate` are `NOT_RUN`.

## Rollback

Revert this decision and return the `AHI-052` status text to proposed, pending owner acceptance.
To withdraw the behavior, also revert the V1-1116 change in `integrations/opencode/src/runtime.js`
and the adapter version bump, then regenerate `docs/specs/REQUIREMENTS.tsv`. No stored state
changes.
