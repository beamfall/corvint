# Decision 0391 — one `make gate` run attests a release's acceptance criteria

Date: 2026-09-25. Status: accepted. Authority: repository owner instruction "let one make gate
attest B7" (2026-09-25). Ticket: V1-0235.

The pre-1.0 review (finding B7) reported that decision 0382 item 4 lets one `full-gate` exit
attest every acceptance criterion of a release, including prose criteria that `make gate` does
not test. For v1-0 (`.taskman/releases/v1-0.json`) these include current stable compatibility
and platform evidence, one artifact set bound by every in-scope gate, and the owner's explicit
acceptance of promotion. The review proposed binding each criterion to a named evidence kind (a
test ID, a retained report path and sha256, or an owner receipt) and refusing completion without
it.

## Decision

1. Decision 0382 item 4 stands. One `full-gate` run, whose `make gate` exit is attested to the
   release, attests every acceptance criterion of that release. The store does not require
   per-criterion evidence, and no per-criterion evidence kind is added.
2. A prose criterion that `make gate` does not test is still recorded as evidence in the ticket
   that closes it and in `docs/BUILD-LOG.md`, as today. That record is not a store gate.

## Not decided here

The review's two other B7 claims stay open under V1-0235: MANUAL release-chain tickets may close
with empty obligations (`allowEmptyObligationsKinds` in `.taskman/policy.json`), and the
`companion-release` gate stays required for a Core-only release.

Rollback: supersede this decision and add per-criterion evidence to the release policy in a
later policy version.
