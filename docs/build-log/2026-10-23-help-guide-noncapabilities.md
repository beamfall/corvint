# Help guide Non-capabilities kept in step with delivery (GitHub #1626)

Date: 2026-10-23. Lane: opencode/help-guide-gh1626 (base 7b3a8e55f). Requirements `CAP-V0-024..029`
in `docs/specs/capability-catalog-v0.md` are accepted under the owner's standing auto-accept
(2026-10-11). `CAP-V0-024` amends the `CAP-V0-018` non-capability rule.

## Verification of the seven listed items (binary built from 7b3a8e55f)

Each item was checked by running the binary in this worktree (a scratch corpus of
`internal/capcatalog` at `7b3a8e55f`) or, where a CLI run needs a full producer setup, by the
owning focused tests.

| Guide item | Issue | Evidence | Verdict |
|---|---|---|---|
| per-test runtime coverage not ingested | #1587 | `flows ingest --coverage-artifact lcov=t.lcov` then `flows ingest-coverage --format lcov` printed `corvint-coverage-ingest/1` with one `tested_by` relation from `source:internal/capcatalog/guide.go` | delivered |
| affected --path: tsconfig paths, non-test module selected | #1578 | synthetic repo: `affected --path lib/helpers/x.ts` selected `tests/a.spec.ts` through a `paths` alias; `--path tests/util.ts` selected nothing | delivered for these shapes; the reporter's layout still selects 0 (open #1604, re-test on main-7b3a8e5) |
| supplied test_run has no association axis | #1571 | `TestDCPV1323AssociationAxisWithholdsWitness` passed | delivered |
| no delta worklist across revisions | #1585 | `docs corpus delta --previous c.json --artifact c.json --format jsonl` streamed a header and entries | delivered |
| no mechanical claim-mention check | #1584 | `docs corpus build --max-unresolved-mention-rate` in help; `TestDCPV134*` passed | delivered |
| docs_search returns no claim hits | #1573 | `docs corpus search --include claims` returned `result_kinds`; `TestDCPV1328*` passed | delivered |
| docs_search has no field weighting or kind filter | #1579 | `docs corpus search --kind symbol` kept only symbols, `--kind package` returned `empty`; `TestDCPV1331*` passed | delivered |

## Decisions

- The seven items left the Non-capabilities section. Goals 8 to 11 now show runnable examples for
  test discovery, coverage ingest, search `--kind`/`--include claims`, `claims --query` and the
  delta `jsonl` stream. The CAP-V0-018 check keeps each one in step with the live catalog. The delta
  stream's schema is not named, because no help topic names it and the guide may name only
  catalogued schemas.
- The one undelivered residual names open #1604 and the `typescript:path-alias-unresolved` reason.
- Guard: `NONCAPABILITY-STATUS.tsv` (id, state, evidence) is embedded in `internal/capcatalog` and
  read offline. `NonCapabilityProblems` fails when an item names an unrecorded ID, a closed issue or a
  delivered requirement. A "By design" item must cite a requirement recorded `excludes`. Any other
  item must cite an open issue or an undelivered requirement. The product-boundary items cite
  `DCW-V0-001` and `WQO-V0-033`, the catalog item cites `CAP-V0-016`, and the archive item cites the
  new `CAP-V0-029`. That requirement records the 2026-10-23 owner decision as a numbered exclusion.
- The guard is a separate function, so `GuideProblems` keeps its signature for `cmd/corvint` and the
  MCP help tests.

## Limits

- The table is caller-maintained. A closing PR that does not update its row is not caught. No
  network check exists by design.
- Observed outside this slice: `corvint docs corpus discover-tests --help` describes `--runner` as
  `jasmine|mocha` (the legacy runner text), but the operation accepts `playwright` and `rspec`. That
  help text lives in `cmd/corvint`. It is reported in the PR, not changed here.

## Review repair (PR #1627)

- The Non-capabilities parse now fails closed (`CAP-V0-026`). It took only unindented `- `
  bullets, so a `* ` item, or an indented or numbered one, skipped the closed-issue,
  delivered-requirement and missing-row checks. Now every non-blank line after the one intro
  paragraph must be a `- ` item or a two-space continuation directly under one. Any other line is
  reported by `NonCapabilityProblems` and `GuideProblems`
  (`TestCAPV0026UnrecognisedNonCapabilityLinesFailClosed`).
- The coverage-ingest example now passes `--test-key` and says the LCOV file holds only that test's
  coverage (`CAP-V0-027`). Without the key, a run with two records is refused. Live run with a
  binary from this branch: a two-test `go-test-json` run plus `ingest-coverage` without `--test-key`
  printed `coverage-refused`, "--test-key is required when the run evidence holds 2 records". With
  `--test-key "example.com/upload > TestRetry"`, the run was `joined` with one `tested_by` relation
  and no unknowns. The guide check used to cut a line at the ` > ` inside that quoted key. It now
  cuts only at a redirection outside quotes, so the whole example is checked
  (`TestCAPV0027GuideTeachesDeliveredCapabilities`).
