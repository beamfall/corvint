# AZU small self-review repair: testless reader callers

Date: 2026-11-05
Starting HEAD: `ada04141cf915858f4af72b520138c49c4ba3e1c`.
Base: `8e2c13aa926fcaf1731fae71b27c41c41c8f09af`.
Scope: V1-4689 / AZU-V0-006. V1-4688 / AZU-V0-005 and V1-4690 / AZU-V0-007
retain their prior implementation and evidence.

The supplied self-review found that an owned, nonhidden Go source can be read by a
testless helper whose caller has tests. The helper has no import edge to that source;
walking only the owner's importers cannot prove absence of observers. The previous
testless-reader traversal admitted unowned and hidden Go paths only, so the plan could
omit the caller test and claim a bounded coverage gap.

Decision: extend that existing traversal to paths whose owner meets the same untested
predicate used before the coverage proof. Preserve tested readers' existing terminal
selection rule. The reader's reverse dependents and their test-only users are selected
with the existing witnesses and frontier bearing rules before coverage classification.
`TestAZUV0006OwnedTestlessReaderCallers` checks both literal and glob readers of an
owned `data/data.go`; `TestAZUV0006OwnedSourceReaderCaller` exercises the Go plugin and
public CLI with a real `os.ReadFile("data/data.go")` helper called from a root test.
Both demand selection of that test, bounded scope, no unknowns and no uncovered units.

The owner's repair instruction explicitly reconciles the two old CLI expectations with
AZU-V0-006. Both existing tests now require the exact isolated-unit uncovered entry,
bounded scope, empty unknowns and empty selections. The isolated-package test additionally
checks an unresolved shell caller's exact NO_SELECTABLE_TEST uncertainty and UNKNOWN
scope; the hostile suite retains all its other explicit uncertainty/refusal assertions.
No golden, wire member, error/exit code, stderr text or frozen core output changes.

Rollback: revert this repair's source/spec/test commit and rebind the CEM. That restores
the reported omission and deterministic expectation failures, not successful qualification.
The spec keeps its experimental status and now states the reader-caller proof obligation.

Self-development routes: reused the existing pre-change capture by owner instruction;
used installed affected selection before tests, and native CEM binding/checking for
structural evidence. Both changed Go packages are selected. Additional capture, broad
mutation/retrieval evaluations and browser execution are outside this repair; no ranking,
learning or provider contract changes. Independent review belongs to the orchestrator.
Native structural checks do not establish semantic adequacy, execution-at-commit or
external interoperability. Cost and token improvements remain NOT_OBSERVED.

Raw evidence stays outside the repository at
`/home/russell/lanes/azu-small-evidence/self-review-current/`.
Verification results, retained limits and evidence digests follow below.


The focused affected checks passed (0.042s), including the literal/glob regression,
the existing isolated/caller/inbound coverage cases and hidden-path reader retention.
The focused CLI checks passed (0.700s), including both reconciled existing tests,
the unresolved-caller assertion and the public owned-source reader regression.
Requirement regeneration passed with no generated metadata changes. Requirement
definitions, traceability and line citations passed (one existing planned traceability
item; 180 existing non-root-basename citations skipped). Repinning found no unchanged
citation content moved, so no second repin was needed.


The entire existing affected package passed in 412.830s. The frozen selection baseline
passed: 33/43 merged cases narrowed (floor 30), zero misses across 26 failing packages
from 15 failed CI runs, and zero test omissions/misses. Its admitted command took 520.3s
on the shared host; this is not a quiet-host performance claim.


The first use-case-receipts check failed because UC-CHANGE-CONSEQUENCE's hostile-tests
receipt pins the modified existing test file. Normal receipt repinning updates only
that subject digest, its repository revision and the ledger digest; no JSON shape or
result changed. The focused hostile suite had already passed. The subsequent receipt
check passed; the whole selected conformance package is also required for these pins.
The failed check remains in receipts-check.log rather than being overwritten.


The entire CLI package ran (594.506s) and failed with exactly 46 top-level test names.
All 46 match the original BASE failures retained in wave1105b/failure-attribution.json;
none is a new failure name, and neither reconciled test nor the new regression failed.
Diagnostics retain the unsafe /home/russell/lanes ancestry (mode 775), outside-repository
root detection, source qualification, migration-ratchet digest and adapter-host failures;
matching names do not by themselves prove identical causes. No base run was repeated.
The whole selected use-case conformance package passed (0.251s) after receipt repinning.
That pin update happened while the CLI run was in flight; Go source did not change.
Conformance is qualified against the final pins, with no execution-at-commit claim.
The wider CLI qualification remains PARTIAL pending host rerun; both requested repairs
are complete. The local dogfood outcome remains failed.

All Go tests ran through script/heavy.sh, one package per command, using Go 1.27.3,
TMPDIR=/home/russell/lanes/azu-small.tmp (700), and the configured shared semaphore.
The two Go source packages and conformance package ran their existing tests whole.
Heavy admission waited up to 120s for host load; the CLI additionally queued 5s for a
slot. No lock-directory or admission override was used. Non-Linux execution, full
make gate, the full AZU-V0-009 mutation/full-suite safety campaign and independent
review remain NOT_RUN in this repair. Independent review belongs to the orchestrator.
Publication, native Tasks writes, model-backed review and sealing are NOT_RUN by
charter. Original pre-change evidence is reused; no new capture or start-of-change
make dogfood-change ran. Guarding-test discovery remains NOT_PRODUCED because its
native runner would bypass heavy.sh. Existing native OCM linkage, prechange chronology,
execution and external qualification limits remain; structural CEM PASS cannot replace them.

Evidence digests (SHA-256; all raw files outside the repository):

| File | Bytes | SHA-256 |
|---|---:|---|
| `affected-installed.json` | 143574 | `303eff70a15e8fcf0202d2c344f2a38d0fb7db3c6e69293b20148baee9b378ef` |
| `test-affected-focused.log` | 928 | `de25baf2c1109a785231a73dfc69b8958341ff637445d71def76a731916478c3` |
| `test-cli-focused.log` | 897 | `0803e4bcd54df765901023b7e63f32964533f361687553ff02ae88ffe2968217` |
| `test-affected-whole.log` | 927 | `e2d7faf1e8cd455786f3cda9c918ee2de51a4fd167209acfbabb6d36abed7594` |
| `test-cli-whole.log` | 66576 | `c1ab669c842b243a26b5524a7419e8b72fe0d4059275c4ecb24bd8ff289e27e1` |
| `test-conformance-whole.log` | 597 | `4e7343831ea757d3345c0aa872b983edc32bb15fa169dce9c38933c2125c3156` |
| `selection-baseline.log` | 10438 | `39cea50187298c84d482432930b8f541642fc2873f5f4a829de46b389b4b929c` |
| `selection-baseline.json` | 927770 | `481fe7636038f0aa56b582cc523130866ef3b78a952f3b5dac612cf2e33e0142` |
| `docs-scoped.log` | 148 | `7d6bb5e3b394557bd55cfa8f2082e943eec85216abc2522e6fb937d490273049` |
| `spec-requirements.log` | 2183 | `5140bfe753d512dd4e84ac3a91ed60a749c56711daac0e0512e4cc2b537c52d4` |
| `receipts-check.log` | 755 | `7279969ede0a42b63c171d429d9e6084c601d73158d12e74e84e26d345027bb7` |
| `receipts-check-final.log` | 480 | `2fc9cc07c07dcf9b3c2eadaea76334c2ecd7a1a0376962f1de349d3dfcf629b2` |
| `cli-failure-comparison.json` | 4759 | `57009c91150455c9e1bf1064c2f374389866fa65d810fca5677665b6bed73bb2` |
| `citations-repin.log` | 190 | `85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133` |
