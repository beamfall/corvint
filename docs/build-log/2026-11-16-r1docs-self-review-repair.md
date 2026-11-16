# 2026-11-16: r1docs self-review repair

## Findings and disposition

P1: the delivered marker in `0c897512c888154fceb1b0680d078c9b525fd36c`
contradicted the qualification record. A subsequent partial trailer corrects the
claim without rewriting history. V1-4365 remains open under LPCV-V0-950 through
LPCV-V0-955, LPCV-V0-963 and GLS-V0-030 through GLS-V0-034. The 467 RSpec baseline
failures, missing full RSpec mutation qualification and 23-test unexecuted-line
control remain blockers. The host must not treat the historical delivered marker
as completion; its trailer check does not implement supersession.

P2: under OSC-V1-008, set Health to RETIRE in lsp-editor-definition-v0,
lsp-stdio-experiment-v0, browser-behavior-falsification-v0 and
work-queue-observation-v0, following their explicit retirement headers. Regenerate
INDEX.json and README from the staged specs. Correct the health evidence counts:
288 added headers comprise 125 RETIRE, 18 MERGE and 145 KEEP. Including the 18
pre-existing KEEP headers, the 306 projected specs comprise 125 RETIRE, 18 MERGE
and 163 KEEP. All 125 projected retired delivery headers now have RETIRE health.
No requirement, delivery header, wire shape, golden or assertion changes.

## Class and limits

These are evidence and inventory corrections, not changes to a program analysis.
The mistaken inventory included bulleted and bold retirement headers. Explicit
retirement is the authority for these four corrections; KEEP cannot override it.
The projection audit checks all retired rows, not just the four reported paths.
No inferred classifier or new syntax allowlist is introduced. Unknown-form
analysis regressions are not applicable: the existing generator projects explicit
health values and its grammar is unchanged. The allowed health vocabulary remains
KEEP, MERGE, RETIRE and OUT-OF-1.0 under OSC-V1-008.

The retired CEM bind/check workflow cannot produce a fresh binding in this tree
(decision 0594, docs/DOGFOOD.md). No replacement proof or seal is claimed.

## Verification

`corvint affected --base df315e35056d15c7933b5ff97c605cedf5ef4919`
selected internal/specindex and tools/openspec-check; its summary remains UNKNOWN
with 267 selected and four retained unknowns (three language frontiers and the
inherited E2E binding). Only focused checks ran.

- `make spec-requirements`: PASS; REQUIREMENTS.tsv unchanged.
- `script/check-line-citations.sh --repin`: PASS, no moved citations.
- `make -s spec-requirements-check line-citations-check`: PASS, including all
  existing internal/specindex tests through heavy.sh (1.046 s, no filter).
  The host-load guard queued for 105 seconds; no lock settings were overridden.
- `script/heavy.sh go test -count=1 -timeout 30m ./tools/openspec-check`: PASS,
  whole existing package, no filter.
- External `check-repair-health.py`: PASS; all four source/INDEX/README values,
  all 125 retired projections, and both added/total counts checked.
- `git diff --check` and staged equivalent: PASS. Go version: go1.27.4.

Raw outputs remain outside the repository under r1docs-evidence; the audit script
is under r1docs.tmp. SHA-256 bindings:

| Artifact | SHA-256 |
|---|---|
| `self-review-affected.json` | `3758a255f256e5f8378a1c52938481663f701a9fdab90dea6dee1ba30d072137` |
| `self-review-health-audit.log` | `4086c8f5510faeeb015b70a8286f8aa153f7b7e45f606060274130c33fe9ef55` |
| `self-review-spec-checks.log` | `bb17e1bddf914a80d91cae01ced7178c82e80e53e6fdfe82f5a0252fae2bd288` |
| `self-review-openspec-package.log` | `ee137ec5c2f7ee929f861b4dd38ecbf8afebe7dbaabe8a27c3382b35a9509cd1` |
| `check-repair-health.py` | `a8d88bd76047233b2039065110b87c49fccf144a023545e3cf86ccb48619ff6d` |
