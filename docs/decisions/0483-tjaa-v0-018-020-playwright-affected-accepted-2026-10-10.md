# Decision 0483 — Playwright computed `use` values, discovery producer and malformed-receipt reasons (TJAA-V0-018..020) accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-10
("accept TJAA"), relayed by the coordinating session.

## Context

GitHub #709 asked the opt-in `playwright-affected/0` profile to stop leaving a project's browser
identity unresolved over a computed non-identity `use` option, to produce the caller-owned
discovery receipt from a caller-run `playwright test --list --reporter=json` listing, and to name
why a malformed receipt was refused. The lane proposed `TJAA-V0-018`, `TJAA-V0-019` and
`TJAA-V0-020` in `docs/specs/typescript-javascript-affected-adapter-v0.md` and went through twelve
review rounds. AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `TJAA-V0-018..020` as written. Each requirement's status, the two cross-references
to them in the spec body, the traceability rows and the lane's build log
(`docs/build-log/2026-10-09-gh709-playwright-affected.md`) record the acceptance. The spec-level
intent stays proposed and the delivery status stays experimental, because the general adapter
remains proposed/experimental.

## Limits

This decision settles intent only. The evidence is focused tests. The accepted behaviour stays
conservative:

- a `process.env` read widens the browser identity;
- a `delete`, `++` or `--` token, an escape, or an ambiguous slash anywhere in the config admits
  neither member-read root;
- the residual gaps are a bare identifier reading a global accessor, mutation from another module,
  a computed alias of `devices`, and Go `(?i)` Kelvin and long-s case folding;
- only one real listing shape is covered.

Live qualification and `make gate` are `NOT_RUN`. Native tickets V1-1065, V1-1066 and V1-1067 are
not completed by this decision.

## Rollback

Revert this decision and return the `TJAA-V0-018..020` status text to proposed. To withdraw the
behaviour, also revert the GitHub #709 extension as the spec's rollback section describes: restore
the static-literal check for every `use` value, remove `affected discovery` and its producer, and
drop the MALFORMED `reason` and `detail` members, then regenerate
`docs/specs/REQUIREMENTS.tsv`. No persisted format or store is written.
