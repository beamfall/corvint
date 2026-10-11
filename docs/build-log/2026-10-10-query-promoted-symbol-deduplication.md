# V1-0266 review round 1: deduplicate promoted symbol identities

Base: `0791c3d0d8efc2f08b7b24743e8088e27b575ce8`.
Intent: `GPK-V0-083` is **accepted, decision 0516** (V1-0266).
No new requirement ID, decision, owner acceptance, promotion or ticket completion is produced.

## Finding and minimal repair

The round-1 P2 finding showed that same-named Go methods on different receivers in one
file can have distinct line-based candidate IDs but render the same `path:name` result ID.
Placement appended both, consuming limit/budget slots and displacing other evidence.
Check rendered `kind:id` against the existing placed set before appending each wider
candidate. Keep the first candidate in confident-selection order and the original order
of remaining evidence. The proposed clause now explicitly covers collisions among wider
candidates as well as symbols already carried in the trailing packet.

`TestEvalLeadWithSymbolsDeduplicatesRenderedIdentity` uses two distinct line-based
candidates with one rendered symbol ID and distinct line evidence. It requires the first
symbol, then a feature with the same ID but a different kind, then a document, each once;
the original packet stays unchanged. Without the fix it fails because both symbols lead
(`review1-red.log`). The relevant pre-change tests passed (`review1-baseline.log`, 2.438s).
The analyzer source-audit convention conservatively invalidates consumer-only changes:
schema `corvint-analyzer/114` becomes `/115` and the audit digest is repinned. No packet
field, fixture, frozen golden, receipt or task/trace store migration is introduced.
Rollback restores the placement loop and matching schema/audit pins together.

## Verification and limits

Evidence files are under `/private/tmp/claude-501/gotmp/q/`, with toolchain `go1.27.1`,
`GOTOOLCHAIN=local`, `TMPDIR=/private/tmp/claude-501/gotmp` and `GOMAXPROCS=3`.

- The new regression failed without the fix (`review1-red.log`) and the focused placement,
  precedence, floor, substitution, negative-claim and analyzer audit tests passed
  (`review1-focused.log`, 14.127s).
- The complete `internal/contextindex` package test passed (`review1-package.log`, 621.084s).
- All nine requested doc targets passed (`review1-doc-gates-evidence.log`):
  `spec-requirements-check`, `requirement-definitions-check`, `traceability-tests-check`,
  `decision-numbers-check`, `line-citations-check`, `error-code-ownership-check`,
  `unbounded-readers-check`, `use-case-receipts-check` and `use-case-receipts-test`.
  Regenerated `REQUIREMENTS.tsv` is unchanged. No existing line citation needed repinning.
- Standalone `sh script/repin-use-case-receipts.sh --check` passed all 18 receipts
  (`review1-receipts.log`); `cmd/corvint/main.go` and receipt pins are unchanged.
- The new binary `/private/tmp/claude-501/gotmp/q/corvint-after2` was built from this
  candidate, then run from `/private/tmp/claude-501/q-bf` against this worktree's unchanged
  `testing/context-retrieval-goldens.json`, with stdin closed. `eval-after2.json` matches
  the supplied `eval-before.json` byte-for-byte after normalization removing only
  `evaluation.metrics.latency_ms` and `evaluation.revision`. All seven cases, selector
  order, states, missing lists and remaining metrics match: critical misses `0`, recall
  `1`, must-read hits `11`, packet bytes `47690`. The matching normalized SHA256 is
  `b1a01e5dce1ffe5b2a2ee1db2e536b40e44ea07eb0b33a76e89c7f6410aa9807`.
  Candidate binary SHA256 is
  `d6439b469b33cab929d8a3d8d604cafeae24be48c80dd03c608c2aaf4eefc341`;
  unchanged goldens SHA256 is
  `d4ae96df2a3bf01d05a44a848894390c915b8ea34b93cabba9ef2a31a4ad9392`.
  The single latency observation `12490.727 -> 14510.424` ms is not a performance claim.
- Independent native read-only review found no blocking issue. It inspected the frozen
  source diff, helper/test assertions, red and passing focused evidence and independently
  verified seven-case parity (`review1-independent-review.txt`). Source-diff SHA256:
  `4bcdb584a62e5dc6e3d204df45fe1f8367aad0b62ac91f0c688c0944b4b057c1`.
  The final prose evidence update is outside that source review. Doc checks passed
  after the evidence update; the later package-result sentence changes no gate input.
  `git diff --check` passed.

Corvint context and affected-plan routes were used at this root/base; original receipts
are `review1-context.json`, `review1-affected-before.json` and `review1-affected.json`.
Context retained slot omissions and the affected plan retained language-frontier unknowns;
static full-gate advice does not override the owner's explicit scoped checks. Snapshot
refresh was unavailable: its derived-state write under the main repository's Git directory
was denied by the sandbox. Post-commit CEM/OCM binding and dogfood completion are deferred
because commits are explicitly prohibited. No full gate or repository-wide tests are run.
Documentation gates use an isolated index under the evidence directory because the real
worktree index is outside the writable sandbox; this checks the staged candidate bytes
without mutating the shared index. The isolated object directory holds candidate blobs;
the first staging attempt exposed that redirecting the index alone cannot redirect blob
writes. Bare macOS `mktemp` ignored `TMPDIR` and tried its denied system default; the
requested Make targets pass using installed GNU `gmktemp` exposed as `mktemp` only in a
private evidence-directory PATH. Repository scripts are unchanged. The receipt fixture test inherited and replaced the
isolated index on the first pass; a repeat immediately exposed that fixture-only index as
stale. The final pass reconstructs and stages the candidate index, runs its eight read
gates, then clears the isolated Git environment for `use-case-receipts-test` so its fixture
owns its own index. Both parts passed; the actual repository index is unchanged. Account usage telemetry is NOT_OBSERVED (no available
account-usage tool); no spend or performance claim is made.

Installed Core is `1.0.0-rc.3` build 407; Tasks is
`0.0.0-tcp01-unverified+build.407`. Official GitHub release evidence reports Core
`v1.0.0-rc.2` (2026-10-06) and standalone Tasks `tasks-dev-20260929.2` (2026-09-29).
The optional updater's network checks fail their version smoke with
`process-after-start-failed: descendant snapshot unavailable`; qualification/freshness
remains UNKNOWN and no downgrade or update is attempted. Tooling blocker filing is
outside the authorized filesystem and the owner explicitly prohibits `.taskman` writes.
No commit, push or ticket-store write is performed.
