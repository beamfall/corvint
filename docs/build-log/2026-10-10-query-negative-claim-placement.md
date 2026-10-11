# V1-0266 follow-up: preserve the nearest-negative-claim packet

Base: `5453a900378d350679fea03c7fad292e5e8087cc` (the initial port in this worktree).
Intent: `GPK-V0-083` remains **proposed, pending owner acceptance (V1-0266)**.
No new requirement ID, decision, owner acceptance, promotion or ticket completion is produced.
This entry supersedes the initial port's unrun adopter-evaluation claim with measured evidence.

## Failure and narrow amendment

The initial port computed wider confident symbols before the negative-claim scan and placed
them even when the original packet required `nearest-negative-claim` abstention. Retaining
the state alone did not retain its evidence: at limit 5 three placed symbols evicted the
critical audited-deletion scenario and an adjacent store-invariant scenario. Wider lexical
support cannot answer a capability override contradicted by project claims.

The proposed amendment blocks placement only when the existing nearest-negative-claim
abstention is active. Evaluate that predicate after relevance-floor withdrawal/substitution
and before placement. Preserve the original support index, result universe/order and its
limit/budget accounting. Suppress record ties only after actual placement. An inactive
nearest claim still permits placement; there is no numeric limit threshold, score tuning,
confidence relaxation or change to the existing override/negative-claim predicates.
The independent omitted-competing-record abstention remains operative.

Rollback removes this guard together with the proposed placement, restoring the pre-port
ranking and its matching analyzer audit. The schema stays `corvint-analyzer/114`; only the
production-source audit digest is repinned. No wire, task/trace store or fixture migration.

## Observed evaluation

Read-only corpus: `/private/tmp/claude-501/q-bf`, clean at
`6e82abd857c77c0ca250b6c4a8f5e1c86684a535`. Inputs are the unchanged
`testing/context-retrieval-goldens.json` in this worktree, with stdin closed.
Base binary: `/private/tmp/claude-501/gotmp/q/corvint-base`.
Fresh candidate: `/private/tmp/claude-501/gotmp/q/corvint-after`, built with
`GOTOOLCHAIN=local TMPDIR=/private/tmp/claude-501/gotmp GOMAXPROCS=3 go build ./cmd/corvint`
and an explicit `-o` to that path.

- Fresh base and initial-port runs reproduced the reported regression in case 5 only:
  critical evidence misses `0 -> 1`, must-read hits `11 -> 10`, recall `1 -> 0.909091`.
- Final base/candidate evaluations match byte-for-byte after JSON normalization removing
  only `evaluation.metrics.latency_ms` and `evaluation.revision`. All seven cases retain
  the same selectors/order, states, packet bytes, `missing: []`, and remaining metrics.
  The actual raw delta is latency alone: `9833.622 -> 8089.378` ms, a single observation,
  not a performance claim. Critical misses are `0`, recall is `1`, packet bytes are `47690`.
- Raw outputs remain under `/private/tmp/claude-501/gotmp/q/` as
  `eval-before-followup.json`, `eval-regressed.json`, and `eval-after.json`.
  The equal normalized output SHA256 is
  `b1a01e5dce1ffe5b2a2ee1db2e536b40e44ea07eb0b33a76e89c7f6410aa9807`.
  Base binary SHA256: `c7f146361d13e88f1cf5e24390aff3d5b4bd2878ba279db3103a1fbf2be2385d`;
  candidate SHA256: `921f420fb711b6c48ff431bb87766b00e66e8a3c46a796148c08388a47d25089`.

## Verification and limits

`TestEvalQueryWiderSymbolsPreserveNegativeClaimAbstention` reproduces the limit-5
two-scenario eviction and also checks limit 10. Disabling the guard makes the finalized
test fail with the three symbols leading (`negative-final-red.log`). Its inactive-claim
control permits placement at both limits; at limit 5 the unchanged omitted-reading rule
still asks for widening, while limit 10 is READY. A temporary pre-port oracle comparison
also proved the complete active-abstention packets unchanged at limits 5 and 10, and nine
no-wider packets unchanged at limits 1, 3 and 10 (`packet-compat-followup.log`). The oracle
tests are archived outside the repository, not shipped as duplicated production code.

Focused floor, substitution, omission, precedence, adopter-shaped limit-10, negative-claim,
deduplication and analyzer tests passed (`focused-followup.log`, 3.211s). Independent
read-only review found no blocking issue; it separately inspected the red test output and
the three eval outputs, confirming the guard, original-packet preservation and final parity.
All requested doc gates passed: `spec-requirements-check`, `requirement-definitions-check`,
`traceability-tests-check`, `decision-numbers-check`, `line-citations-check`,
`error-code-ownership-check`, `unbounded-readers-check`, `use-case-receipts-check`, and
`use-case-receipts-test` (`doc-gates-followup-final.log`). The first doc run failed only on
three shifted source citations. After reading their original/current source, only line
numbers were repinned in `core-compatibility-freeze-v1.md` and `snapshot-batch-v0.md`;
their content hashes and accepted intent are unchanged. Regenerated `REQUIREMENTS.tsv`
is unchanged. Standalone `sh script/repin-use-case-receipts.sh --check` passed all 18
receipts (`repin-check-followup.log`); `cmd/corvint/main.go` and receipt pins are unchanged.
The toolchain is `go1.27.1`; `git diff --check` passed. No full gate or `./...` run is
authorized. The optional package-wide rerun is NOT_RUN for this bounded follow-up;
the previous 1262.594s package run is historical evidence, not a test of this repair.

Corvint context, query and affected routes were used; original outputs are retained in the
same scratch directory. Advice retains its unknowns and does not replace requested checks.
Snapshot refresh and `make dogfood-change` refuse writes to shared Git metadata outside
the sandbox. Immutable post-commit CEM/OCM/seal is `NOT_PRODUCED` under the explicit
no-commit scope. Documentation staging/generation uses a copied temporary Git index and
object directory; it cannot claim real-index staging. Account usage telemetry and live
model/effort controls are unavailable (`NOT_OBSERVED`); inherited reasoning settings are
used for the bounded repair and independent evidence review.

Final readback found that `script/repin-use-case-receipts_test.sh` inherits
`GIT_INDEX_FILE`/object overrides into its fixture's `git add -A`/commits. This replaces
the caller's scratch index with the UC-X fixture (confirmed by its indexed contract and
unrelated staged deletions after the five cases pass). The real index remains untouched.
Restore the copied index, restage only task-owned files, and isolate this fixture test
with the three Git overrides unset. Retain this confirmed harness defect for orchestrator
deduplication/filing; `.taskman` writes are explicitly excluded here.

Installed Core is `1.0.0-rc.3` build 407; Tasks is
`0.0.0-tcp01-unverified+build.407`. The official release listing currently shows Core
`v1.0.0-rc.2` and Tasks `tasks-dev-20260929.2`. Both updater checks fail version smoke with
`process-after-start-failed: descendant snapshot unavailable`; freshness remains UNKNOWN.
No verification bypass, update or downgrade is attempted. Tooling blocker filing is retained
for the orchestrator because this task explicitly forbids `.taskman` writes.
