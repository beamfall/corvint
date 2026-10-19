# 2026-10-19 V1-1977: adopter zero-authority report was a ledger measurement defect

Ticket: V1-1977 (P1, no GitHub issue). An external Go adopter with a root `AGENTS.md` and accepted
`docs/adr` decisions showed `zero-authoritative-rate=69/69`, a standing
`frontier-authority-unavailable` DRAFT and p95 1447 ms in `corvint observations`.

## Diagnosis

- Project authority resolved. A read-only `corvint query` in the adopter returned authoritative
  results that cited accepted ADRs, and the synthetic fixture below returns `accepted-decision`
  evidence from a hook event.
- The 69/69 figure was false. User-prompt coverage reaches `observationEvent` after
  `sanitizeHarnessQueryContext` JSON-decodes the packet, so every count is a `float64`. The old
  reader (`integerValue`) accepted only `int`, so it recorded each authoritative count, and the
  omitted count, as zero. The budget-omission rate was under-counted for the same reason.
- `frontier-authority-unavailable` is on every receipt by design (`AHI-054`, decision 0519): no
  accepted closing Frontier authority exists. It says nothing about `AGENTS.md` or ADR authority.
  The digest labelled it `STANDING` with a DRAFT, which read as a project-authority defect.
  Removing it from receipts stays with V1-0869.
- The real setup gap was a missing or stale index snapshot for the committed tree. Codex hooks
  then rejected as `dogfood-event-index-snapshot-stale` (`AHI-031`), and events that did run built
  in memory. That in-memory build is the likely source of the latency. `corvint index --if-stale`
  is the fix.

## Decision

`SOL-V0-011` to `SOL-V0-013` are added to `docs/specs/self-observation-ledger-v0.md`. They are
accepted under the owner's standing auto-accept of lane requirements (2026-10-11).

- `SOL-V0-011`: user-prompt rows record whole-number counts, whether they arrive as `int` or as
  `float64`. Rows with a readable count are marked `authorityCounted`. The zero-authority rate
  counts only marked rows. Unmarked legacy rows print as `AUTHORITY-UNMEASURED` and are never
  reported as zero authority.
- `SOL-V0-012`: `frontier-authority-unavailable` prints as `DEGRADATION-BY-DESIGN` with the
  decision path that fixes it, and is never given a STANDING label or a DRAFT.
- `SOL-V0-013`: retained snapshot-stale adapter rows print one `SETUP` line naming
  `corvint index --if-stale`.

Acceptance criterion 2 named `corvint doctor`. Core has no `doctor` command; only `corvint-tasks`
does. The setup diagnosis therefore lives in `corvint observations`, Core's existing read-only
diagnostic, and adds no new command surface. A Core `doctor` verb is NOT_PRODUCED.

## Evidence

`TestAdopterUserPromptReturnsAndRecordsProjectAuthority` is the external-adopter qualification. It
uses a synthetic Go module with `AGENTS.md` and two accepted `docs/adr/NNNN-*.md` decisions; no
adopter content is copied. The test fails if a user-prompt hook event returns no accepted-decision
authority. It also fails if the ledger row's count differs from the packet's, or if the digest
reports anything but `zero-authoritative-rate=0/1`. Unit coverage is in `internal/observations`:
`TestAuthorityRateCountsOnlyMeasuredUserPromptRows`, `TestFixedFrontierDegradationIsReportedByDesign`
and `TestSetupLineNamesSnapshotRefresh`.

## Limits

- Existing ledgers keep their unmarked rows, which show as `AUTHORITY-UNMEASURED` until they age
  out under the 128 KiB cap. The adopter's own ledger was not rewritten; its repository is
  read-only to this lane.
- Hook latency is not changed. This slice does not measure whether a fresh snapshot brings the
  adopter's p95 under budget.
