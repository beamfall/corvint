<!-- SPDX-License-Identifier: Apache-2.0 -->
# Required archive integrity and bounded flaky rerun exceptions

Owner intent: lane ghci charter, 2026-11-08, V1-5098 and V1-5099. Base:
`9761ea1f459c0d3b3c963953e24a76eaeda16370`. This entry records the delegated decision;
runtime tests, hosted qualification, independent review and CEM binding remain with integration
under the owner's final build-only instruction. No measured or hosted pass is claimed here.

Archive correctness is a premerge guarantee (CIN-V0-8721), enforced by the existing required
go-product aggregate. It now needs artifact-integrity and cadence-plan and checks integrity
before any early return. Failure, cancellation and unknown results block. A skipped job is
accepted only for verified static reuse on a main push, verified PR cadence DEFER, or a batched
PR; the merge queue must succeed, including its existing verified same-tree reuse. This preserves
CIN-V0-4753's fast PR cadence while ensuring the merge queue actually carries the deferred work.
It changes neither the archive wire format nor release/publication authority or the ruleset.

CIN-V0-8720 refines AFP-V0-441: flaky.sh still diagnoses a complete fail-then-pass and retains
the same FLAKY record. A separate authorization reads an exact test/package allowlist from the
immutable event base, never the working-tree file. A failed invocation without a listed test,
an expired/malformed list, or missing/capped/unsupported evidence cannot clear the shard status.
The policy is bounded to 16 KiB/128 entries; stream reads stop at 256 MiB or a 1 MiB line and
refuse rather than authorize a prefix. The FLAKY diagnostic and upload also remain for refused
exceptions. A first-rollout base without the file fails closed.

The initial policy expires on 2026-11-15 at 00:00 UTC and contains only
`TestATRV0059_ClaimCarriesDrainNotice` and
`TestATRV0310_DetachedChildrenAreJoinedBeforeTheLeakCheck` in `internal/tasks/cli`, tied to
V1-4549. The read-only queue snapshot produced 2026-11-04T17:49:52Z reports V1-4549 OPEN;
`2026-11-04-cx-affected-final-review-fixes.md` identifies precisely these unresolved failures.
The earlier report documents repairs to the collateral failures, so they are not seeded.
The local journal and current ticket liveness are NOT_OBSERVED. V1-4678 is absent from the
available snapshot and is not seeded. Policy renewal requires checking the owning open ticket;
the offline CI consumer validates the ticket identity but does not claim a live queue lookup.

Acceptance harnesses (registered in go-static):

- `.github/forward-rows/known-flakes_test.py`: listed/unlisted exact identities, expiry day,
  malformed/duplicate/missing/capped inputs, immutable policy despite working-tree edits,
  and execution of the workflow's actual forgiveness block while preserving the FLAKY record.
- `.github/forward-rows/go-product_test.py`: executes the actual aggregate shell for passing and
  failing integrity, both positive and negative skip authorization, and every early-return class;
  also checks needs and producer bindings. This is an extract test, not a hosted outcome claim.

Run both with `python3 -B` at integration, then the existing whole `tools/ci-forward-row`
package through `script/heavy.sh`, after affected selection. Existing assertions and goldens
are unchanged. The documentation and implementation are separate commits for github-split-check;
this decision entry is its own commit. Rollback is to revert the corresponding workflow/helper
changes; no product state or receipt migration is involved.
