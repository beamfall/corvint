# AZU small review fix 2: reader inputs and frontier bearing

Date: 2026-11-05. Starting HEAD: `39fe25f0d624d968522c2f6e46b04eaf1ba6f17d`.
Base: `cdeed7ee6396a8d6bf6225eb5b2b4cb4d601b419`.
Owner scope: V1-4689 / AZU-V0-006, the two findings in review fix round 2.
Other lane tickets retain their earlier disposition.

## Decision and regression evidence

A unit seed and a reached reader each retain one ranked witness. Comparing only
those paths cannot prove reader absence for a multi-file change. Coverage proof
now collects every changed path owned by the unit, independently discovers its
literal, glob, scoped and unbounded readers, and inspects their caller closures.
Unowned declared callers still prevent a coverage-gap proof. No witness ranking,
selection traversal or graph/wire shape changes.

`TestAZUV0006MultipleChangedPathsReaderUncertainty` covers four reader kinds.
Before implementation, literal and glob cases produce BOUNDED, empty unknown and
uncovered go:data; scoped and unbounded controls pass. The public CLI regression
`TestAZUV0006MultipleChangedFilesDeclaredReader` uses real Go/shell plugins:
both data/a.go and data/b.go change, check.sh reads only data/b.go, and a verified
foreign-caller declaration names unowned Makefile. It requires exactly the Go
owner's NO_SELECTABLE_TEST uncertainty, so invalid declaration fallback cannot
hide an incorrect proof.

The earlier proof also inspected raw reader frontiers without TSP-V0-001's
own-files bearing rule. It now uses that existing rule over the complete dirty
set. A reader's build variants remain irrelevant when only another package's
bytes change. The unchanged
`TestShortFragmentsKeepConcatenatedReaders_V1_1818` retains BOUNDED and selected
root_test.go. Conversely, when the reader's own files change in the same batch,
its frontier still bears. `TestAZUV0006ReaderOwnFilesFrontierInBatch` reproduced
an uncovered go:data despite a changed reader's build-constraint frontier; it
requires the changed owner's NO_SELECTABLE_TEST uncertainty.

Membership is cached per path within one plan. With one dirty input every reached
witness already names it, so membership is reused; multiple inputs still discover
readers independently for every owner path. This avoids redundant discovery for
structural changes that seed many owners, without weakening uncertainty checks.

The experimental AZU-V0-006 clause states and traces these constraints. No JSON
members, field names, diagnostics, exit/error codes, committed goldens or
core-freeze outputs change. No existing assertion is weakened. Mandatory repin
moves two citation line numbers in the core compatibility and falsifiable packet
specs; their content hashes and contract text remain unchanged.
Rollback: revert this round's source/spec/test commit and rebind. This restores
the reported risks; no experimental feature is promoted by this repair.

## Verification and retained failures

Native affected selection preceded tests and selected affected, golang and
cmd/corvint. Its receipt retains six UNKNOWN entries: build constraints, nested
modules, binary-exec/read-scope declaration validation, unmodelled shell callers
and an unindexed hidden test. No zero-unknown claim is made. Original pre-change
captures are reused; no new start capture or make dogfood-change runs. Used
self-development routes: affected selection, focused/whole regression checks,
frozen selection replay and native CEM rebind/check.

| Observation | Result |
|---|---|
| New multi-path regression before repair | FAIL, literal/glob cases, 0.019s |
| First whole affected package | FAIL, 197.693s; best budget round p95=186.564399ms, 7.59 probes (bound 5) |
| Cached-reader whole affected retry | FAIL, 258.251s; best calibrated round p95=462.658873ms, 6.48 probes |
| Single-input reuse, focused budget and AZU regressions | PASS, 65.715s |
| Prior whole affected / Go plugin / filtered CLI iteration | PASS, 144.359s / 20.659s / 1.184s; superseded by batch-frontier correction |
| Prior frozen replay on that iteration | PASS, 759.6s; 33/43 merged narrowed, floor 30, zero misses/omissions; superseded source |
| New batch-frontier regression before correction | FAIL, 0.007s; go:data incorrectly uncovered |
| Delivered whole affected package | PASS, 144.221s, no -run filter |
| Delivered whole Go-plugin package, including V1-1818 | PASS, 14.489s, no -run filter |
| Delivered filtered CLI regressions | PASS, 1.093s |
| Delivered spec metadata, definitions, traceability and citations | PASS, 18.6s admitted |
| Final staged repin and citation check | PASS, no remaining unchanged-content move |
| Delivered frozen replay | PASS, 33/43 merged narrowed (floor 30), zero misses over 26 failing packages / 15 failed CI runs, zero test omissions or misses |

The first two frozen attempts were interrupted (exit 130) when their source
changed; neither qualifies any delivered source. All failures and superseded
observations remain outside Git. Timing assertions, load thresholds, semaphore
settings and fixture behavior are unchanged. Shared-host wall time is not a
quiet-host performance claim. The delivered Go diff against starting HEAD is
`69a76e66d25691c4c79752ca93148fb937bcedd328623b5597610ba18c1ef073`
(SHA-256, tested-go-delivered.patch); no Go source changed after delivered tests.

All Go checks use script/heavy.sh, one package per command, Go 1.27.3 and
TMPDIR=/home/russell/lanes/azu-small.tmp (700). The configured shared lock
directory works; no lock/load/slot/parallelism override is used. Tests and gates
inherit no Git author/committer identity exports; those occur only in bind and
commit shells. The metadata gate's separate specindex/lrfrepo checks were also
selected by affected. The final native CEM report and check are recorded in the
external handback after the committed source and reviewed citation plan.

Independent review belongs to the orchestrator; no model-backed reviewer runs.
Whole CLI execution is excluded by this round's explicit filtered-regression
charter; prior whole-CLI host failures remain unqualified. Exhaustive repository
gates, Windows before-push vet (no push), Tasks writes, publication and sealing
are excluded. Darwin execution, external interoperability, new native guarding,
coverage/discrimination campaigns, live browser execution, retrieval/learning
and cost/token savings remain NOT_RUN or NOT_OBSERVED as applicable. Existing
lane qualification gaps remain. Git commits succeed despite a best-effort lock
attempt on read-only /home/russell/projects/corvint/.git/packed-refs.lock; that
path and its permissions are left alone.

Raw evidence stays outside Git at
`/home/russell/lanes/azu-small-evidence/review-fix-2/`.
SHA-256 digests of the decisive observations:

- `affected.json` (144684 bytes): `41e1cde8957ab546040ca29e8d6c972954aa2e838e32a07db00a8ebc0962f464`.
- `test-before.log` (1264 bytes): `3dc4875f88574347ad3223d52e34479d7ad0ed3f09b4e543833d8c46449b7859`.
- `test-affected-whole.log` (1736 bytes): `55ebfed21c8db7b7d115c74574178edd74ec8258533830130fdbe357de2d4002`.
- `test-affected-whole-retry.log` (1737 bytes): `c416437ab3cb6b117feeecd8a6e52d062eda00f00cbe97003b48f2ced55f51b6`.
- `test-affected-budget-final.log` (796 bytes): `1140a49d73c83705dd38249220dc437b339280a2bb042d79f20dd382c384f6c2`.
- `selection-baseline-terminal.log` (10438 bytes): `9d2ad273f1709ca25c0a5088c0d80f0c600b609ec6b531c2783e5dd7ace0aa20`.
- `selection-baseline-terminal.json` (927773 bytes): `22054aa84c0743a7820a0bf1269550c41435bc29819f3069f9986a2f86bdefc9`.
- `test-frontier-batch-before.log` (1121 bytes): `9a758cf43e991ced0f60b606d6edb92bc54c8fba34a2aee7a4e9d2b3a0f2ca67`.
- `test-affected-whole-delivered.log` (798 bytes): `7467fee2a277fed572f845bdc4459d22c2a100d1bd1c8d85b7e3b912f77a4114`.
- `test-golang-whole-delivered.log` (610 bytes): `400adb9a8283e180f35508b24443aa9629489cd90e3282c9041e9164fd4c512d`.
- `test-cli-regressions-delivered.log` (764 bytes): `290a8f8ca40fab3080858210876ff212c157c33d6b94f98c3533ba8df8bcf284`.
- `scoped-gates-delivered.log` (3933 bytes): `00b868c210bc461ac9804f33df0b8e0ace4ffd477766e51e6ab8b7ec4921125a`.
- `repin-delivered.log` (190 bytes): `85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133`.
- `line-citations-check-delivered.log` (118 bytes): `855a4551b199f407e4e7cec705511746022a39aa0f70383fc13aac9acc748bba`.
- `tested-go-delivered.patch` (9601 bytes): `69a76e66d25691c4c79752ca93148fb937bcedd328623b5597610ba18c1ef073`.
- `selection-baseline-delivered.log` (10438 bytes): `ec1d0c31a347262def51da1f99883646a6a72de6461413fae241cf8c2970c41b`.
- `selection-baseline-delivered.json` (927773 bytes): `501c0a261f1f6433a381392961db465e4c79857423731a4db0d0eea39692836c`.
