<!-- SPDX-License-Identifier: Apache-2.0 -->
# V1-5087: complete runtime widening across owners and planning modes

Owner: Russell Lewis. Date: 2026-11-18. Requirements: CIN-V1-940, CIN-V1-941.

The review found two gaps: runtime widening ran before test-selection declarations resolved
and omitted their owners; snapshot and ignored-file planning bypassed the typed compatibility
retry. An unresolved runtime target could therefore omit an integration owner and its users,
or refuse a valid declaration in those two planning modes.

Resolve declarations before computing widening and apply declared owners during every closure
round. Owners reached from source or test paths feed the next round, including their reverse
dependents and further owners. Snapshot and ignored-file rebuilds now use `affectedBuildGraph`
with their actual root and no cache. The ignored exclusion remains held until retry finishes;
the snapshot retry reads its materialized immutable tree.

The soundness class is omitted declared relations during uncertainty propagation, plus planning
entry points bypassing compatibility admission. Admission stays closed: only the typed unknown
runtime-unit error retries; the complete retry validates the existing runtime-edge profile and
known test holders. The test-selection decoder accepts only its defined profile members and
owner path/test forms. Unknown members and invalid holders refuse. Arbitrary dirty-path forms
cannot certify a smaller closure: they retain the same unbounded runtime witnesses and UNKNOWN
scope. No new parser exception, guessed target path or error-string matching is introduced.

Regression evidence:

- `TestRuntimeEdgeWideningDeclaredOwners_CINV1941` covers the reported owner, its reverse
  dependent, an owner reached in a later round, an unrecognized dirty source form and refusal
  of an unknown owner member. The unrelated unit stays excluded.
- `TestAffectedRuntimeEdgeSnapshot_CINV1940` covers an unresolved target in a snapshot and
  verifies identical planning after the live declaration gains an unknown member. Ordinary
  live planning refuses that member.
- `TestAffectedRuntimeEdgeIgnoredRebuild_CINV1940` covers uncached and cached initial plans
  with an ignored JavaScript report, checks that the report stays excluded on retry, and
  retains the refusal for an ignored Python input outside that exclusion contract.

The full affected package passes (8.293 s), including the existing assertions. The affected
plan selects both changed packages and internal/specindex: 426 selections, 61 unknowns,
scope UNKNOWN. The plan is selection evidence, not whole-repository qualification.
Spec requirements and line citations pass; the invoked internal/specindex package passes
(0.936 s). Citation repinning found no moved citation; 184 existing untracked basenames remain
skipped. CLI whole-package verification remains blocked as recorded below.

The first whole CLI run failed (161.951 s) in seven existing test groups: temporary paths
intended to be outside Git were discovered under `/home/russell/lanes`, changing repository
refusals and preventing profile output. The new runtime regressions reported no failure.
A subsequent sanitized Git probe of the scratch root returned `not a git repository`
(exit 128). The source was unchanged for the retry. The repeat failed in the same seven groups (152.193 s). The cause of enclosing repository
discovery during these tests, unlike the standalone probe, is NOT_OBSERVED. No parent Git
state, golden or assertion was altered. V1-5087 remains PARTIAL for whole CLI qualification;
the host verifier must resolve the scratch environment and rerun this package.

Raw evidence stays under `/home/russell/lanes/batch-m8-evidence/self-review-*`; final command,
citation and intent manifests stay under `/home/russell/lanes/batch-m8.tmp/`. The orchestrator
owns independent review and broader verification. This round changes no wire shape, diagnostic,
exit code, committed golden or existing test assertion. Rollback is to revert this repair;
that restores both reviewed gaps without changing the strict library build contract.

The requested CEM closeout is unavailable: `docs/DOGFOOD.md` records removal of the bind/check/
seal workflow under decision 0594, and this checkout has no corresponding Makefile target.
No replacement capture or seal is produced.

Evidence SHA-256 digests:

- `self-review-test-affected.log`:
  `4c253a3033d00d8dea1cc9589d8a6693b49f36c9e4b9775590a1239f1e00d9b0`
- `self-review-test-cli.log`:
  `849166a47289f676544deb45c4f12e024a277de4e5b4d3cae38d75f59f8801a0`
- `self-review-test-cli-final.log`:
  `0a4d9a1c604aa924d48e614733bdbc2633521749ba6524f02f7e56a456555452`
- `self-review-spec-checks.log`:
  `dce938096530a730075dd9b961dae7e7366bede0d3a81d382c9f0659787c1457`
- `self-review-affected.json`:
  `613a6d4cbd3b6aa1906e9e5eee8261d62682a71ad3ecbe5015af851674a76825`
- `self-review-parent-probe.log`:
  `d2b35cacadc66b7fad8dedd1be5244e17150a6625978c9330f831bd7a9cd6b1e`
