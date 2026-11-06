# V1-4729: sharded ratchet explanation repair

Date: 2026-11-06
Base: `9db7a94169821d9b28701eaa88866aee2fec1083`.
Repair input: P2 self-review finding at `d1c0f986f74f55c1f5952bf01fed9668fcb48f74`:
FULL retains the ownership obligation in GoSlices but both partitioned-unit
constructors discard its explanation.

## Decision and requirement coverage

AZU-V0-9900 requires the named ratchet and triggering path in dry-run reasons
even during FULL fallback. Copy the package's retained Reason and its existing
ReasonCode into every CI slice in placeGo and placeMeasured. Partition flags,
patterns, placement, catch-all execution and FULL widening stay as specified.
No new fields, codes, stderr text, goldens or core-freeze outputs are introduced.
Rollback removes those four assignments and restores the missing explanation;
there is no format migration or new capability.

TestOwnershipPartitionedReasons_AZU_V0_9900 uses the committed affected-package
partition across 16 shards. Twelve cases cover universe, balanced and measured
placement, narrowed and FULL plans, and gate-only and independently reached
whole-package obligations. Each partitioned unit must carry the exact reason
and existing code, siblings occupy different shards, the catch-all is present,
unrelated units acquire no reason, and the caller's slice remains unchanged.
The existing whole-package reason regression is retained unchanged. The owning
spec's acceptance evidence names the new witness; no requirement is amended.

## Verification and retained limits

Raw evidence: `/home/russell/lanes/ratchetsel-evidence/sharded-repair-*`.
Before the implementation fix, the new regression fails all 12 cases because
partitioned units lack the reason and code (0.238 seconds). After the fix,
`script/heavy.sh go test -count=1 -timeout 30m ./internal/sessiontest` passes
the entire touched package, including every existing test (3.485 seconds).
The local toolchain check reports the required go1.27.3. Identity variables
are absent during tests; the prescribed private TMPDIR and configured heavy
semaphore are used without overrides.

Self-development routes: affected selection is used against the charter base;
the installed binary's stale notice is retained, and a tree binary built with
heavy.sh repeats selection without that notice. Both select sessiontest and
retain five unknowns (three language frontiers and two binary consumers).
Existing pre-change context receipts are reused as instructed. Final CEM
binding/checking reuses that capture; no fresh dogfood-change capture is run.
Retrieval/learning evaluations and optional integrations are not applicable to
this execution-unit explanation repair. The orchestrator owns independent
review and host-wide qualification.

This finding is fixed; V1-4729 retains its prior PARTIAL disposition for
outstanding host qualification and prohibited native completion writes.
Earlier CLI qualification failures are not requalified or represented as
passing. Whole-repository selection, tests/vet, doc-gates, Windows vet,
selection-baseline-check, gate-affected, direct corvint test, model-backed
review, task writes, publication and seal are NOT_RUN under the repair scope.
The aggregate local outcome remains failed for the retained CLI qualification.

The staged citation repin finds no moved citations.
`make -s spec-requirements-check line-citations-check` passes, including its
heavy-wrapped specindex/lrfrepo checks; configured host-load waits are retained
in the log. Gofmt and the staged whitespace check pass.

| Evidence | SHA-256 |
| --- | --- |
| `sharded-repair-before.log` | `ccf3b048994c10fa1d824071c3cb7c56f3a90f713a98a64d4e8163380cf52935` |
| `sharded-repair-sessiontest-whole.log` | `f56e56b40c230f8a70d86774baf339f03f7a88025620725dcf1131eb3e1058eb` |
| `sharded-repair-spec-citations.log` | `5398952bf04e7a056479082edc776bea03bfaf4ed6cd1c57d054984454281a45` |
