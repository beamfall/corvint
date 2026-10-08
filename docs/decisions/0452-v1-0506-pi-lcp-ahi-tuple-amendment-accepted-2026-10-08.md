# Decision 0452 — Pi LCP/AHI tuple amendment accepted

Date: 2026-10-08. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-08
("approve all, and you can self approve these tickets"), covering native ticket V1-0506.

## Context

V1-0506 requires an owner-accepted LCP/AHI amendment that explicitly admits the exact Pi tuple
before any continuation. The amendment landed through PR #384 and is recorded in
`docs/build-log/2026-10-01-pi-capability-contract.md`: `LCP-V0-008` admits the ordinary Pi
`agent_before_settle` boundary for host `pi` 0.99.1, surface `extension`, adapter 0.3.2, with one
bounded remediation continuation, `stopHookActive` loop release, session-namespaced identity and
refusal on errors, aborts, queued work and stale generations; `LCP-V0-009` admits that tuple for
the four dogfood events under the shared `corvint-dogfood-event/0` envelope, deadline and refusal
codes; `AHI-024` binds the bridge to that exact tuple. `PWV-V0-008` in
`docs/specs/pi-workflow-v0.md` keeps continuation unavailable without such an amendment.

## Decision

The owner accepts the Pi tuple amendment in `LCP-V0-008`, `LCP-V0-009` and `AHI-024` as written.
The intent strings of `local-completion-policy-v0.md` and `agent-harness-integration-v0.md` record
it (decision 0452; V1-0506) in the header, digest, `docs/specs/README.md` and `INDEX.json`.

## Limits

This decision settles intent only. Delivery statuses are unchanged (`implemented` for LCP,
`experimental` for AHI); the Pi adapter remains FALLBACK. The spec and ticket keep continuation
conditional on exact-host qualification, and the protected Pi profile, supervisor participation
and comparative qualification stay unadmitted. No other host tuple, version or event is admitted.
V1-0506 stays open pending its native closeout.

## Rollback

Revert this decision and restore the previous intent strings in both specs, the README rows and
INDEX entries, then regenerate `docs/specs/REQUIREMENTS.tsv`.
