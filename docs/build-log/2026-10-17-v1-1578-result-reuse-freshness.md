# V1-1578: fingerprinted result reuse, redundancy and evidence freshness

GitHub #1021 (fingerprint and result reuse) and #1024 (evidence freshness OLD..NEW). Requirements
PWR-V0-033..040 in `docs/specs/playwright-run-evidence-v0.md` (PWR-V0 max on origin/main was 023;
the range starts at max+10). Agent-drafted and accepted under the owner's standing auto-accept of
lane requirements (2026-10-11).

Decisions:

- Extend `corvint runs` (PWR-V0) instead of a new store or top-level verb: the retained results are
  the Playwright JSON reports the caller names, and the fingerprint reuses the declared
  `corvintRunIdentity` plus a new declared `testRevision`. No root dispatch edit, so no use-case
  receipt repin.
- Fail closed: every row is `REUSABLE` only on a qualified pass at the identical fingerprint
  (`EXACT_PASS`) or, with `--fresh`, at a fingerprint differing only in application/test revision
  where the declared reach is `FRESH` for the test file (`FRESH_PASS`). Missing inputs, no prior,
  non-pass, unqualified priors, any other changed input, `STALE` and unresolved freshness are
  `MUST_RUN` with a reason.
- Shared-code reach is repository-declared in `.corvint/evidence-reach.json`; nothing is
  hard-coded and the declaration itself is always in reach. Freshness is read from the NEW commit's
  tree through bounded pinned Git; every failure is `UNRESOLVED` with a reason, never `FRESH`.
- `testRevision` joins the shard grouping key (PWR-V0-016 amended): shards at different declared
  test revisions are not one run. PWR-V0-015 goldens are unchanged.
- The repository-root lookup for `--fresh` lives in `cmd/corvint` (an admitted unbounded unit), not
  in `internal/evidencereach`, so the new package stays bounded for `unbounded-readers-check`.

Live qualification (this repository, binary built at `38f9525b9`):

- `runs reusable` over the retained `@playwright/test` 1.61.1 reports
  (`internal/playwrightruns/testdata/runs`): 4 selected, 0 reusable, 4 `MUST_RUN`
  `IDENTITY_INPUT_MISSING` naming `testRevision`. The reports predate the declaration, so the read
  fails closed as required.
- `runs redundancy` over 7 reports (5 logical runs): 28 tests, 80 ms, all unfingerprinted.
- `runs fresh --old 55abb815 --new 38f9525b9`: `STALE`, 13 changed paths, 11 deciding (the
  declaration as `reach-config`, Go sources as `shared`). Same commit: `FRESH`.
  `55abb815~1..55abb815`: `UNRESOLVED REACH_UNDECLARED`. `git status` unchanged after the reads.

`NOT_RUN`: a live Playwright run declaring `testRevision` (no runner installed in this lane);
`make gate` and `go test ./...` (scoped lane). Residual: `corvint test` (STR-V0) receipts carry no
per-unit fingerprint, so reuse covers Playwright reports only.

Rollback: revert the change; nothing is persisted.
