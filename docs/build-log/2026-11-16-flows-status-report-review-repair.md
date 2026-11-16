# V1-5623 review repair: flow status results, unknown reach and the docs pages refusals

The independent review of the first V1-5623 delivery (lane `claude/r1flows`, reviewed head
`423c4ad20`) returned CHANGES_REQUIRED with ten findings. This entry records the decisions the
repair made and the evidence; the change design's "Decisions made while building" holds the rules.
The intent of `docs/specs/application-flow-status-v1.md` stays `proposed`.

## Findings and decisions

- AFL-V1-006 over the run evidence store. A stored record of an unknown profile
  (`corvint-test-run/9`) or one that does not parse gave exit 0 and `NOT_OBSERVED` everywhere.
  RVD-V1-010 (accepted) skips a corrupt record and marks its tests; AFL-V1-006 refuses a bad
  receipt whole. `flows status` now refuses whole (`run-evidence-corrupt: ...`, exit 2) when a
  record is intact but not a document this build reads, larger than any record a writer keeps, or
  held tests the store cannot name; a record damaged after writing whose tests the log names stays
  the RVD-V1-010 case. `runevidence.Corrupt.Invalid` (not encoded) carries the distinction; the
  other store readers are unchanged. This is a builder reconciliation of two requirements and is
  for the owner to confirm.
- AFL-V1-001. Only `reviewed` test links attach a test; stale and unreviewed links attach none.
  AFU-V1-160 (the first version of the block) is reconciled to that. Fixtures of
  `internal/command/flows` and `cmd/corvint` that relied on an unreviewed link now read a reviewed
  one; `TestAFUV1014StatusSkipsStaticRecord` keeps the STATIC rule covered for status.
- AFL-V1-007, APK-V0-010. A change touching only paths of unknown reach (`src/components/Cart.vue`,
  `db/schema.sql`) gave `flows impact` an empty list. `unresolvedReach` now lists such paths in
  `reach_unknown`, and every flow is named with all variations and `reason: reach-unknown`;
  `status --base` keeps the same flows and variations. Both members are absent otherwise, so
  `impact.golden.json` is unchanged.
- Bounds. The named `--evidence` files are bounded together and apart from the store, so a full
  store no longer refuses a one-record file (`flows gaps` already behaved so).
- Project and slice. A receipt row names no Playwright project; for a variation of several
  projects the result is `NOT_OBSERVED` with reason `project-not-recorded`. A sliced receipt unit
  carries `slice` onto the row.
- `docs pages`. An unknown verb with `--help` is `invalid-arguments` (was `internal-error`); a
  positional after `docs pages flow` is refused (it ran flow-maintain); refusals and hints name the
  new spellings; `corvint-docs flows status|impact|gaps --help` says the verb moved to `corvint`.
- Criterion 3's ingested-report fixture is now the committed Playwright JSON report of the
  AFU-V1-039 acceptance fixture, taken through `flows ingest` to a file and with `--record`, then
  through `flows status` (`TestAFLV1002PlaywrightReportThroughIngestIntoStatus_V15623`).

## Evidence

Each code finding's test was run against the unrepaired code first; the failing output and the
package runs are kept with the lane handback. Focused tests ran through `script/heavy.sh`, one
package per command; `make changed-checks` and `make windows-vet` ran on the repaired tree.
