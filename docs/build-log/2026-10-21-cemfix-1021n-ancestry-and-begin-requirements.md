# cemfix-1021n: postmerge ancestry guards and begin-time requirements validation

Lane `cemfix-1021n` (base `58d4ed2d3`, one `origin/main` merge before dogfood). Requirements are
accepted under the owner's standing auto-accept of lane requirements (2026-10-11).

## V1-2259: postmerge readers refuse rewritten ancestry

**Problem.** `postmergeconnector` (binding verification) and `postmergemetrics` (`Measure`) answer
`merge-base --is-ancestor` in a checkout that may carry `info/grafts` or be shallow, so a rewritten
history could be reported as a verified pair.

**Decision.** Reuse `gitancestry.Rewritten` (no dependency on PR #1466's `RewrittenAt`).

- The connector checks it first in `verifyBinding` and refuses `ancestry-rewritten` (`PMC-V0-010`).
  The check comes first because a grafts deprecation warning otherwise breaks the earlier
  `rev-parse` read with an unnamed `commit-unavailable`.
- Metrics returns `ErrGit` (unavailable evidence, `PMM-V0-008`).

`internal/witness` `resolveBaseTree` stays unguarded: the `corvint-witness-v0` graft contract
already covers it.

**Residual unguarded readers, out of this lane's scope.**

- Outside the excluded packages: appflows, `cmd/corvint/runs_baseline.go`, breakagemap, dashboard
  trace, sharedpath, upgradechanges, tracerecordrepo, gitnotes, genesis, and root-commit reads.
- `dogfoodflow` and `contextindex` are excluded from this lane.

**Evidence.**

- `TestChangedFilesRefusesRewrittenAncestry_PMCV0010` and
  `TestMeasureRefusesRewrittenAncestry_PMMV0008` cover grafts and shallow checkouts, each followed
  by a clean read.
- Both fail without the fix: the connector test gets `commit-unavailable` or `<nil>`, and the
  metrics test gets `<nil>`.

## V1-0680: `dogfood begin` validates numbered requirements

**Problem.** `dogfood begin` enrolled any intent path that existed at HEAD. Finish then refused the
same intent with `invalid-requirements-section` after the selected checks had already run.

**Decision (`LCP-V0-037`).** `Begin` reads each intent blob (4 MiB bound) before `plan.json` creates
a generation. It refuses with the code that finish's default-form `ocm prepare` would report,
through the new `lrfrepo.IntentRequirementsRefusal`, which reuses `requirementsFromBlob`. Begin and
finish therefore cannot disagree on the same bytes. A tree or an oversized blob refuses with
`repository-object-unavailable`.

Fixtures that enrolled an instructions file or a section-less document now enroll a spec intent:

- `cmd/corvint` claude, Pi, event, handoff and verify tests
- the opencode inspector qualification fixture
- the host-lifecycle conformance runner, which now uses `docs/intent.md` in place of `add.go`

**Evidence.**

- `TestEnrollmentRefusesIntentWithoutNumberedRequirements_LCPV0037` covers three refusal codes,
  each repeated 16 times with no state written, after which a valid plan still enrolls. It fails
  without the check.
- The `localcompletion`, `lrfrepo`, `opencodequalification` and conformance packages pass, as do
  the focused `cmd/corvint` dogfood and local-completion tests.

## Skipped tickets

- **V1-1339 and V1-2180:** already delivered, by f807a6706 (#1440) and fda3374ee (#462).
- **V1-1567:** needs the SQLite sqllogictest runner, a network fetch and build.
- **V1-2030:** needs an owner decision between an independent identity probe and hook-reported
  incompleteness.
- **V1-0585:** not started (budget). Its `DOGFOOD_OCM_LINKS` side lives in excluded `dogfoodflow`.

## NOT_RUN

- Tier-2 affected packages.
- Live opencode and Claude host-lifecycle qualification.
- `make gate`.

## Rollback

Revert the lane commits. Each requirement names its own rollback.
