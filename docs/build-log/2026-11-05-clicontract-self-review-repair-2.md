# clicontract self-review repair

Owner intent: repair the five P2 findings against incoming head
`af359c18345a1b13137961d405d071fb828ea23b`, retaining base
`d436dff76de1b8e75c427069ae7f80003ae99bc2`. Preserve native contracts,
goldens, existing assertions and the pre-change capture. The owner reserves
independent review and integration; finish at checked binding without seal,
publication or Tasks writes.

## Decisions and requirement mapping

- V1-2685 / CAP-V0-9300: render an explicit catalog root before generic root
  help. The process conformance test now checks each root's actual usage,
  flags, required inputs and example, in addition to its existing assertions.
- V1-3736 / ACX-V0-9301: a valid existing path in a catalog-declared positional
  root slot reaches the native command even when it resembles a verb. Bare
  unknown words retain the existing refusal; explicit `./` paths already reach
  the native parser. This resolves the ambiguity without changing the frozen
  all-binary unknown-verb assertion. The process regression compares native and
  adapter success bytes for real `go` and `./go` directories.
- V1-3736 / ACX-V0-9301: classify discovery-only and native CLI execution modes
  explicitly. Gateway `serve` refuses with the existing protocol-required
  envelope before process launch; gateway short operations remain eligible for
  native execution. The process test reserves the requested address without a
  background server and requires the offline refusal, no native failure details,
  empty stdout and an entirely unchanged empty working directory.
- V1-2685 / CAP-V0-047 and CAP-V0-9300: carry guaranteed argument equalities
  through conjunctions in route discovery, including parenthesized and reversed
  equalities. Keep the else branch unconstrained and do not infer a prefix from
  disjunction. Negative controls use already-cataloged `alpha` and `help` words
  but require the missing `alpha help` route to be reported.
- V1-2685 / CAP-V0-9300: generate canonical folded help before shim trees and
  documentation. The existing scratch Makefile test additionally starts with a
  stale folded tree and changed help source, requires all four refreshed shims
  and current documentation after one run, and compares the entire fixture to
  a second run. The same test against the incoming Makefile fails with
  `corvint-tests-accept consumed stale folded help` (exit 1).

No new wire field, refusal code, exit status, stderr text, golden or core-freeze
output is introduced. Rollback reverts this scoped repair; no state migration.

## Verification evidence

Raw evidence lives outside the repository under
`/home/russell/lanes/clicontract-evidence/self-review-repair-2/`.
All Go tests use `script/heavy.sh`, one package per command, with private
`TMPDIR=/home/russell/lanes/clicontract.tmp`. The configured lock directory is
unchanged. Whole touched packages run without a filter. The original
pre-change capture is reused; `make dogfood-change` is not rerun.

Self-development routes used: affected advice, command help for the existing
citation workflow, and final CEM binding/checking. Retrieval/learning changes,
external providers and live interoperability are not applicable. Independent
review is orchestrator-owned, NOT_RUN here. Prior host qualification failures
remain retained in the lane handback; this scoped repair does not erase them.
Missing telemetry and semantic acceptance remain NOT_OBSERVED.

- Affected selection before tests: UNKNOWN, 232 selected and 92 unknown
  (80 LANGUAGE_FRONTIER, five NO_SELECTABLE_TEST, five UNOWNED_DIRTY_PATH,
  two UNSELECTED_BINARY_CONSUMER); every executed Go package is selected.
- Whole packages PASS: `internal/cliprofile` 0.019s, `internal/clitree` 0.009s,
  `tools/capabilities-check` 11.139s, `tools/cli-discovery-gen` 5.226s, and
  `conformance/cli-errors-v0` 166.869s. No existing assertion was weakened.
- `script/capability-catalog_test.sh`: PASS; incoming-Makefile negative control
  fails as expected. Real `make capability-catalog`: PASS, 10.9s execution;
  snapshots, generated documentation and core-freeze outputs remain unchanged.
- `make spec-requirements`: PASS, no generated changes. Initial unstaged-spec
  refusal resolved by staging the specs. Citation repin reports no moved anchors.
- An initial clitree compile failed on a misplaced brace in the edit; the corrected
  source passes the whole package and the independent drift-gate package. Losing
  logs remain retained. Heavy commands initially queued behind eight occupied
  slots were interrupted (130) and retried in the runner's bounded light class;
  no lock directory override or resource limit relaxation was used.

| Artifact | SHA-256 |
|---|---|
| cliprofile.log | 1230df5a95bbcd8a78e30f75da5d29c14c0f7b9ce6d5c600158043f482cd7c6d |
| clitree-retry.log | 9f8b782ec3d9cd34253f633ba010276befc38c772c3c088ba650499628047b33 |
| capabilities-check.log | 4239d632bc58466d9ecb3df77469cecdac121bedcf811ae3b1cf16d937c9c576 |
| discovery-gen.log | 5c741c5f5bbbb1c42eac2efd7ee2eff4a391e953865c3e657680501252dbd644 |
| conformance.log | bd950d3e88ff509340a25ae326c3b2ceff3fefd6242ac4d375bcd655d3b450d7 |
| catalog-regression.log | e6b6cb6f12681c4c5575f8b3735099ba1cce6e86f2fbbd9043b96b7e4fbba8c3 |
| catalog-negative.log | d77dd158f3950fa0a4a6c69c7b62283eca24877bb99fd667fb944ee3e2399624 |
| catalog-retry.log | ff54e213f8313633e7d2df65e695d76a347ef402b432adbbf7937c556f8ad265 |

The local aggregate outcome remains **failed** because earlier host qualification
failures are unresolved; all five requested repair checks pass. Exhaustive gate,
Windows/Darwin qualification, independent review and native queue completion are
NOT_RUN for this scoped round. The checked CEM is structural evidence only;
per-hunk adequacy, guarding-test execution and semantic acceptance are not claimed.

Final focused documentation, line-citation, ownership, requirement-definition,
traceability, README, capacity, test-read-scope and declaration-compatibility
checks PASS (47.5s execution). `docs-final.log` SHA-256:
`9c59ec2dcea1f0c2132d7de72dcb1a2b8311aca38eb7e4a71dbb71265c252e2d`.
