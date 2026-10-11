# Application navigation ledger — V1-1132 / #774

Date: 2026-10-10
Base: `55aa6bb9f1e9d8db81f8185b5d185fc8c1bbc821`
Intent: proposed, pending owner acceptance. Delivery: experimental.
Owning spec: `docs/specs/application-navigation-ledger-v0.md` (ANL-V0-001..005).

## Bounded slice and decisions

One single-map `flows appmap edges` CLI derives adjacent declared steps, reuses RVN receipt
qualification and endpoint lineage freshness, and checks the flow anchor for selector drift.
Common test keys must bind both endpoints; a JSON proves/assumes sidecar can supply these
declarations but cannot create execution evidence. The result keeps one row per edge, selects
the shortest proven declared flow, lists redundant proven test keys, and fails required or
assumed edges closed before applying output byte caps. Full-ledger digests survive projection.

AMAP actions are free text and its templates are route URL templates. Only exact supported action
strings identify a trigger; other actions remain UNKNOWN. Proof for an UNKNOWN trigger is only
owner-declared adjacency, not proof of a particular interaction or observed step ordering.
Component source templates have no map anchors and are outside this slice. Flow-anchor freshness
is conservative: unrelated changes within the same flow intent can also demote proof.

The implementation uses new ledger and CLI files with small dispatch/help and exit-1 hooks.
It does not change router dialects, the deep-link validator, qualification report, the AMAP spec,
test planning, map encoding or persisted state. Rollback removes these additive files/hooks.
The builder retains its inherited model/effort for the evidence-binding work; an independent
native reviewer uses the inherited configuration. Live model controls and account usage telemetry
were unavailable; cost and comparative savings are NOT_OBSERVED.

## Evidence and review

- Corvint query returned an authoritative AFU-V1 route with omitted results and unparsed sources;
  original AMAP/RVN source and spec evidence supplied the narrower ledger contract.
- `corvint affected --base 55aa6bb9f1e9d8db81f8185b5d185fc8c1bbc821` ran before tests. Its generic
  exhaustive-gate advice is explicitly superseded by this task's focused-only test instruction.
- Pre-edit focused baseline `Test(AMAP|RVN|AMSP|Flows)` passed: appmap 408.371s, CLI 6.221s.
- Initial `TestANLV0` passed for both packages. After the review repair, `TestANLV0` passed again
  (appmap 8.115s, CLI 1.460s). The final focused suite passed (appmap 364.173s, CLI 6.774s):
  `GOTOOLCHAIN=local TMPDIR=/private/tmp/claude-501/gotmp GOMAXPROCS=3 go test -count=1 ./internal/appmap/ ./cmd/corvint/ -run 'Test(ANLV0|AMAP|RVN|AMSP|Flows)'`.
- `go vet ./internal/appmap/ ./cmd/corvint/` passed with local Go 1.27.1 and GOMAXPROCS=3.
- Independent review found a fail-closed aggregation gap: an inconclusive outcome on a second
  key could be ignored beside a pass. The repair blocks every evaluated per-key uncertainty;
  a key with no supplied receipt is explicitly distinguished. Regression cases cover skipped
  outcomes, absent receipts and an unresolved application revision. The independent reviewer
  closed the P1 by inspection with no further findings in the repair.
- Doc-gate first attempts found digest/README claim-format mismatches and a traceability header
  parsed as a test name. Both were repaired. A later gate identified three internal verification
  reasons missing from the owning spec; their meanings are now documented. This was a metadata
  omission, not an additional runtime repair. All requested final doc gates passed through the
  temporary-index setup described below: `spec-requirements-check`,
  `requirement-definitions-check`, `traceability-tests-check`, `decision-numbers-check`,
  `line-citations-check`, `error-code-ownership-check`, `unbounded-readers-check`,
  `use-case-receipts-check` and `use-case-receipts-test` (18 receipts current; five test cases).
  `gofmt` readback and `git diff --check` were clean. Test/gate logs are retained in
  `/private/tmp/anl-focused.log`, `/private/tmp/anl-repair-tests.log`, `/private/tmp/anl-vet.log`
  and `/private/tmp/anl-doc-gates.log`; source hashes in `/private/tmp/anl-verified-source.sha256`.

## Environment limits and handoff

Installed Core: `1.0.0-rc.3 (build 407)`; Tasks: `0.0.0-tcp01-unverified+build.407`.
Available official versions remain UNKNOWN: `corvint-update check --allow-network --component
core|tasks` failed its version smoke with `descendant snapshot unavailable`. No verification was
bypassed and no binary was replaced.

Index refresh and `make dogfood-change BASE=...` failed because the linked worktree's Git metadata
is outside sandbox write roots. The normal `git add` also failed to create its index lock.
Requirements generation and doc checks therefore use a temporary copied index and object store
under `/private/tmp/anl-git-*`; a task-local Git wrapper confines this alternative to the current
worktree, preserving fixture repositories' own Git state. A scratch `mktemp` wrapper supplies an
explicit writable template for macOS's otherwise unwritable no-template default. These wrappers
are outside tracked source. The real worktree index is NOT_STAGED and needs coordinator staging.

No commits, pushes or `.taskman` writes were made. CEM binding, OCM, post-commit dogfood check/seal,
native ticket completion, live Playwright and adopter qualification are NOT_PRODUCED/NOT_RUN.
No full gate or `./...` test was run. No main.go edit or use-case receipt repin was needed.
Tooling-friction ticket filing is deferred because this lane explicitly forbids store writes.

## Proposed follow-ups

Typed interaction declarations and spec annotation parsing; component/template and page-object
anchor coverage; multi-map per-app aggregation; test costs beyond declared flow step counts;
test-plan integration; live receipt qualification on the adopter's suite. These remain proposals,
not completed acceptance claims. The ticket stays open for owner acceptance and the remaining
acceptance scope; this builder supplies only the requested smallest useful end-to-end slice.

## V1-1133 / #775: navigation observation (ANL-V0-006..008)

Slice: reporter JSON only (`playwright-navigation-events/0`), resolved with the existing
`screenIndex.urlMatches` (hash prefix aware). Output is a separate learned document, so static nodes
cannot change. Redaction is name-and-shape based (secret-shaped names, 24+ character opaque values,
userinfo, action input values). Follow-ups: read Playwright trace zips directly (action metadata and
locators), feed observed edges into the edge ledger as PROVEN evidence, and verify `--receipt`
bindings against observed crossings. Builder rule change (owner, 2026-10-11): this and later tickets
were hand-built; Codex ran only as reviewer.

## V1-1134 / #776: prefix lint (ANL-V0-009..011)

Slice: static `test-sequence` chains plus the edge ledger. The map records neither per-edge
assertions nor write effects, so those come from an explicit `test-prefix-facts/0` sidecar and an
undeclared effect abstains (fail closed); without facts the lint flags nothing. Ledger rows match
test-sequence edges on from/to only because sequence edges carry no trigger. A proving test key may
be the flagged spec itself (receipt keys are titles, spec paths are files); not excluded here.
Follow-ups: derive read-only/mutating classes from page-object method effects, use observed traces
(ANL-V0-006) as sequence input, and exclude self-proofs once receipts map to files.

## V1-1135 / #777: page-state assertion grouping (ANL-V0-012..014)

Slice: a read-only grouping view over the existing `test-consolidation-input/0`, as
`test-plan page-state`; `consolidate` and `check` are untouched. Only non-destructive,
state-preserving variations with all anchors group; changes are blocked rather than keyed to a
post-change state. Follow-ups: `test-plan extract` from Playwright specs (needs page-object
resolution), `check` drift detection against a pasted grouping, ordering inside a shared visit.

## V1-1137 / #779: change impact on screens and edges (ANL-V0-015..016)

Slice: `flows appmap impact` over the paths in `git diff --name-only BASE HEAD`. The map stores
templates as URL templates, not files, and has no component binding, so the ticket's
template/component hop and the contextindex import graph are not delivered: such paths read
unknown and force `full-suite-required`. Descendants use the existing `parent` lineage. Follow-ups:
declared component/template bindings with the import graph, per-app verdicts for multi-map input,
and routing the verdict into `corvint affected`.
