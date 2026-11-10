# Design: `tasks-integration`

## Decisions

- Tasks is CLI only in 1.0, with nine requirements PROD-TASK-001 to 009; corvint-tasks publishes
  ticket schema v1 and a supported-command list (owner decision R17, 2026-11-09). TIN-V1-002 drops
  MCP parity; TIN-V1-013 adds the command contract.
- Schema names `taskman-ticket/1` and `taskman-receipt-ref/1` are proposed, owned by corvint-tasks,
  and vendored in Corvint pinned by SHA-256 (release-tasks 7/8).
- No Corvint receipt resolver: Tasks stores and audits receipts by content (release-tasks 6;
  TIN-V1-005 retired, TIN-V1-012).
- Trial subject: `beamfall/core` (owner decision R18, 2026-11-09).
- Trial dates (decided 2026-11-09 under R42, C22): PR #4 by 2026-11-13; Tasks RC tag and trial
  start 2026-12-08; trial end 2026-12-29. Publishing the Tasks RC tag is the owner's act. The
  full date list is in TCF-V1-009.
- Trial store (decided 2026-11-09 under R42, C23): the `beamfall/core` store is initialized by
  pull request on 2026-12-08 with the tagged RC binary, so no store is written by a pre-freeze
  format. Seed reviewed by 2026-12-01; dry run on a copy by 2026-12-04. Merging that pull request
  is the owner's act (TIN-V1-014).
- Review evidence is advisory in the gate unless the policy opts in (owner decision R4,
  2026-11-09); the blocking input is `affected` plus the receipt. The review JSON belongs to change
  `review-evidence-check`; the policy keys belong to change `tasks-policy-v15`; the conformance
  record format belongs to change `tasks-conformance`.

## Failure modes

- The conformance record is missing or has a failure: the RC signing check refuses.
- Fixture drift in either repository: that repository's CI fails.
- A stored receipt ID does not resolve on audit (file gone, bytes changed): audit reports a
  failure naming the ticket and the ID.
- A ticket cites a flow or variation ID that does not exist, or both `.corvint/flows.json` and
  `.corvint/e2e-flows.json` exist: the citation check fails closed (`E2E_FLOWS_INVALID` for the
  second case, G6).
- The gate requires Corvint and `corvint` is not installed: the gate fails with a typed reason; it
  never records a pass.
- A store format change during the trial: the 21-day clock restarts.
- The `beamfall/core` store is created on the first trial day with no rehearsal, and a failed
  initialization costs trial days: the dry run on a copy by 2026-12-04 covers it (TIN-V1-014).
- A Corvint-side caller uses a command not on the Tasks supported-command list, or the list is not
  published: the CI check fails or reports NOT_RUN, and the RC is refused (TIN-V1-013).
- The vendored schema copy does not match its SHA-256 pin: Corvint CI fails (TIN-V1-004).
- corvint-docs is absent when the gate records a review report: the docs section is NOT_OBSERVED,
  not a failure (TIN-V1-008).

## Acceptance evidence

- The Tasks conformance record at its 1.0 tag and the RC signing check reading it.
- The shared fixture passing in both CIs at the paired tags.
- One recorded I1 gate run on Corvint's own store, with the receipt ID resolving on audit.
- One resolved and one refused I2 citation.
- A completion gate run recording a review report.
- The 21-day trial record with the ID and hash diff.
- The CI check of TIN-V1-013 failing on an unlisted command.
- The three earlier open questions are settled: the nine Tasks requirements are PROD-TASK-001 to 009
  (owner decision R17), the second trial repository is `beamfall/core` (owner decision R18), and
  review evidence is advisory unless opted in (owner decision R4).

## Open decisions

All are now decided under R42 (panel pick); the rows in `docs/specs/PRODUCT-PLAN-1.0.md`
are settled.

- O21 decided, R42 (panel pick) C9: replayed held-out PRs count toward the 30 soak PRs but are
  reported separately, and at least 20 of the 30 are live Corvint or Beamfall PRs.
- O22 decided, R42 (panel pick) C9: the trial floor is 50 ticket writes and 5 completions per
  trial repository, read from the store event log (TIN-V1-009).
- O56 decided, R42 (panel pick) C10: the impact report matches tickets by commit-message IDs and by
  claim scope, each match labelled, cited tickets first (CRI-V1-005).
- Decided 2026-11-09 under R42, C22: the trial runs 2026-12-08 to 2026-12-29 (TIN-V1-009,
  TCF-V1-009).
- Decided 2026-11-09 under R42, C23: the trial store in `beamfall/core` is initialized by pull
  request; the merge is the owner's act (TIN-V1-014).

## Rollback

Revert the Corvint-side commits; the release check, I2 resolver and fixture check go and the IDs are
retired. The coupling itself is an owner decision; reversing it needs a new owner note.
