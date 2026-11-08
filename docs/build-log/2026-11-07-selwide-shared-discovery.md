# Selwide: one Go discovery path for FULL execution

V1-5018 / AFP-V0-9012. The 2026-11-07 independent review and orchestrator
charter require one implementation for package admission and output exclusions.
This supersedes the copied Python parser decision in
`2026-11-07-selwide-full-universe-repair.md`; mirroring a parser still permits
future disagreement and does not close the class.

Decision: graph construction and execution share `packageSourceFiles`, the
bounded `Source.FilesIncluding` walk, `observeModules` and `groupByDirectory`.
`PackageDirectories` exposes explicit directory arguments; the PR driver uses
Go to validate those arguments and return sorted import paths. The workflow
builds the same tool and invokes `--mode packages --root .`; no Python code
interprets output declarations or maintains another directory skip list.
The new mode is an internal CI helper, documented in help and CAPABILITIES;
no core command, command-tree output or JSON member changes.

The soundness class is excluding tests without shared source authority. The
closed declaration allowlist is exactly `readNonSourcePaths`' existing
`corvint-non-source-paths/0` grammar, reached via `DeclaredNonSourceHolds`:
regular bounded file, known members/profile, canonical protected-path rules,
ordered unique entries and bounded printable reasons. Any invalid or unknown
form grants zero exclusions. There is no list of approved dot-directory
names: every name the graph admits is retained, including unfamiliar names.
VCS metadata and the graph's existing visible skips, testdata and module
ownership rules remain centralized. FULL refuses a truncated walk or an
unproven module frontier; only the known unlisted-nested-module frontier is
allowed because those modules are outside the execution workspace.

Sharded/unsharded FULL and historical FULL observations use explicit package
argv. Historical expected outcomes and rerun universes use the same discovery
function. Shadow validation requires the exact frozen test prefix and complete
canonical expected package list. Container export validation admits the new
explicit package grammar with all named outcomes required, and keeps existing
legacy wildcard receipt validation. The selection's FULL marker, JSON shapes,
existing diagnostics and exit codes remain unchanged. No golden is edited.

Acceptance witnesses: `TestPackageDirectoriesMatchGraph_AFPV09012` compares
disk/immutable execution admission with graph units, including `.vscode`, an
unknown dot name, output declarations and module boundaries;
`TestPackageDirectoriesUnknownWorkspace_AFPV09012` and the existing bounded-walk
test refuse incomplete universes. `TestHiddenSkippedNamesFullExecution_AFPV09012`
executes failing `.vscode/helper` and unknown-dot tests with one and four shards
and forbids wildcard execution argv. `TestHiddenUniverseDeclarations_AFPV09012`
executes the workflow's actual Go command for `.github/` and unknown forms,
retaining strict graph refusal and requiring all hidden runtime packages.
Existing narrowing, disagreement and FULL assertions remain.
`TestHiddenShadowUniverse_AFPV09012` covers historical hidden outcomes;
`TestContainerRunResult` additionally rejects unknown argv and missing outcomes.

Observed failures retained: the first workflow probe's Go stub emitted relative
paths instead of import paths, which the unchanged package grammar refused.
The first whole workflow suite caught an accidental escaped quote in an
unrelated raw shell fixture; that line was restored exactly. The first whole
driver suite exposed four historical-row validation failures after explicit
argv replaced wildcard execution. Its expected-universe discovery and strict
argv validation were updated together; no assertion was weakened to accept the
mismatch. Subsequent focused consumer checks passed.

Self-development routes: pre-change context is reused (recapture forbidden);
`affected` selected all three touched packages but reported UNKNOWN and a stale
installed binary. Runtime tests compile the current source. CEM rebind/report
and dogfood-check are the final local route. Independent review, broader frozen
checks, host integration and native ticket completion remain host-owned.
V1-4941 / AFP-V0-9013 and V1-4710 / AFP-V0-9014 are unchanged this repair.
Rollback: revert this repair and retain the independent findings as unresolved;
do not promote a copied enumerator as equivalent or rewrite frozen outputs.
Raw logs remain under `/home/russell/lanes/selwide-evidence/review-5-*`.

## Verification evidence

| External artifact | SHA-256 | Result |
|---|---|---|
| `review-5-affected.json` | `02cbd83a49b2c56b88d430c073e992b5ba155eef40435f2536814430afcc1f29` | UNKNOWN: 426 selected, 71 unknown; three touched packages selected |
| `review-5-focused-workflow.log` | `ae88699aa5dd062da54bbefc4173c40f6f80431dfd3bd61549770418c050220b` | FAIL: stub emitted relative package paths |
| `review-5-whole-workflow.log` | `75d5a2f6067498fa3db9f1ea16afea2e5fc7fb59ff7c51272b583bb5c550dd36` | FAIL: accidental fixture quote; restored exactly |
| `review-5-whole-driver.log` | `5e2c6de2bd93978ccf594dce15423d9fc76a599b3149778972c94a5448a14a5a` | FAIL: four historical validators expected wildcard argv |
| `review-5-focused-consumers.log` | `7ea26ee35a103007a3a9919c20d30cc6c6434fd439c347ecd8812166740dd3a7` | PASS: focused shadow/export consumers, 46.475 s |
| `review-5-whole-graph.log` | `f48e9086ae4dae1580f29f7e7f9e6b35da07acff1f27968d67971691c6a17d32` | PASS: whole Go adapter, 9.429 s |
| `review-5-whole-workflow-bound.log` | `1f34e196de0d9be4c49a2b58dc856b2b1f2fbdb0b7ec479ed47126d95dce7aa4` | PASS: whole workflow package, 67.067 s |
| `review-5-whole-driver-final.log` | `bed76b052ed1572f6b83e86d61e370ad111f34985b7a8e50fe15a0f9d3d4ed9a` | PASS: whole PR driver, 178.610 s |
| `review-5-spec-check-final.log` | `8c7c73354beb66cc5090b5383fa310218b7f09e22195e1079d2b9e97392471cb` | PASS: spec requirements and staged line citations |
| `review-5-repin-final.log` | `85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133` | PASS: no moved citations; 180 skipped basename citations |

All three touched packages passed as whole packages through `script/heavy.sh`,
without `-run` filters or exported Git identity. The existing narrowing tests
and frozen-argv assertions passed. Go toolchain: go1.27.3. The final VERIFY file
records these three commands and the spec/citation check. No broader gate,
Windows/Darwin execution, hosted qualification or independent-review verdict
is claimed. The shared semaphore queued the spec check for 13 seconds and
warned about low `/tmp` space; no configuration or lock override was used.

The final input precheck refused `test-read-scopes-invalid`: the workflow test's
new real helper build needs its transitive source closure in the existing
read-scope declaration (AFP-V0-023). Its wrapper reports only the first five
findings. A scoped `go list -deps` of the helper identified the complete closure;
the entry now names `go.mod`, the exact dependency package directories and the
helper directory (existing `.github/` covers both hidden dependencies).
The scope check remains strict; no declaration or assertion is removed. This
refusal happened before any prepare or bind. A confined whole-package rerun
checks the actual compiler reads as well as the unconfined package result.

Integration evidence (same external artifact directory):

| Artifact | SHA-256 | Result |
|---|---|---|
| `review-5-inputs-check.log` | `a307bd7bebc42af9cb7a19b1f89facf13fd67f6499ba127f418934f15f6279c7` | REFUSED before bind: undeclared build closure |
| `review-5-whole-workflow-scopes.log` | `3cc7de863a08f93a2f14127eadd1eb2473c56299fad234eeb6b8cb7f7100c41b` | PASS unconfined whole package, 49.013 s |
| `review-5-whole-workflow-confined.log` | `e91385c6c7032cb08e2d23f4a9eb74996ec049cc212add4cf17beeb979e26924` | PASS whole package under test-confine, 52.603 s |
| `review-5-citations-scopes.log` | `855a4551b199f407e4e7cec705511746022a39aa0f70383fc13aac9acc748bba` | PASS staged line-citation check |
