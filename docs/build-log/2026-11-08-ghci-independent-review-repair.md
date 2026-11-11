<!-- SPDX-License-Identifier: Apache-2.0 -->
# ghci independent review repairs

The owner supplied three independently reviewed findings against the ghci lane:
stale Go workflow pins and upstream fixtures (P1), missing Go traceability (P2),
and an unexplained seed expiry (P3). This repair retains the production integrity
check and every prior negative assertion; no wire, diagnostic, golden or core-freeze
output changes.

V1-5099 / CIN-V0-8721: both dependency pins now include artifact-integrity and
cadence-plan. Each Go package has one goProductUpstream helper that supplies
successful integrity alongside the other prerequisites. The coverage guard tests
therefore reach coverage failures again. TestCINV08721IntegrityBeforeEarlyReturns
pins all four outcome bindings and executes the actual aggregate shell for DOCS,
REUSE and batching across push, pull_request and merge_group events. Failure,
cancellation, absent and unknown future results refuse at the integrity check.
The existing production allowlist is unchanged: success, or skipped with a verified
main-push static REUSE, verified PR cadence DEFER, or batched PR; merge_group needs
success. This addresses the fixture class, not a production analysis gap. The
Python suite retains the complete valid/invalid skip authorization matrix.

V1-5098 / CIN-V0-8720: TestAFPV0441WorkflowBindings additionally pins known-flake
authorization before status=0 without removing an assertion. The traceability
rows now name the Go witnesses for both requirements. The policy row comments and
spec explicitly explain the existing exclusive 2026-11-15 expiry as seven days
from the 2026-11-08 charter date. This is not a renewal or a new claim of live
ticket status: the 2026-11-04 snapshot remains the only observed OPEN status.

Verification used the owner's base 6339f980e13e36c8ff51a593568da3d935ffae77.
corvint affected selected both touched Go packages directly; scope remains UNKNOWN
with 76 LANGUAGE_FRONTIER / NO_SELECTABLE_TEST rows. Both existing whole packages
passed through script/heavy.sh without -run, identity exports or lock overrides.
All nine known-flakes and four go-product Python tests passed. The permitted
spec-requirements-check and line-citations-check passed, including internal/specindex
whole and the target's internal/lrfrepo pins. Repinning found no moved unchanged
citation. The citation checker retains its 180 unsupported-basename skips.

Repository-wide gates, additional affected readers, Darwin qualification,
publication, native Tasks completion and model-backed review are NOT_RUN under
the repair charter; the supplied review is the independent finding source, and
the orchestrator owns follow-up review/host verification. Pre-change capture was
not repeated. CEM binding/checking is the remaining local finish; no seal is allowed.

Raw logs stay in /home/russell/lanes/ghci-evidence. Digests bind these results:

| Evidence | SHA-256 |
|---|---|
| independent-ci-reuse-plan.log | `f2f36380208a0cc95fd370a76198b81525ad3725e56d0db2d83913d50bd23b9a` |
| independent-ci-forward-row.log | `c88817bb44515612180500022f9d11f694decfea621f4dac82fff5b8afe06f8e` |
| independent-known-flakes.log | `daef75f19bd3ff278182ee2aa5617d5f5b5d6f925741bf5b0fec0788e7896375` |
| independent-go-product.log | `f38f0b95219a9c7a7d0eaf22d169436e824969863b8692f5d39700f8f3112738` |
| independent-spec-check.log | `1acb2613d5b6d126a8a1acf819f6e1e28afe15560c58e52dc69a527df849651c` |
| independent-affected.json | `69752df9338307ab82352805f0df680c9ec503a4211129136263df9d642d1cbf` |
