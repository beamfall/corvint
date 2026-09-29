# CAL-V0-018 incremental import

The accepted S6 contract requires one full audit followed by work proportional to the records
written under that same lock/session. IMPORT_APPLY now owns a process-local sequence of freshly
decoded canonical records, ID/case-fold indexes, receipt identity and physical metadata. Public
Model remains the untrusted-input oracle; whole-export importer graph/reference validation,
request identity, canonical wire state, physical CAS and pending redo retain their existing paths.

Capacity reuses the archive encoder's per-entry arithmetic and checked totals at every existing
publication prefix, including abort/fresh UNPAUSE reserve, request-shard creation, evidence dedup,
growing-before-shrinking posts and head digit changes. A size-only digest-width placeholder never
becomes content evidence. Capacity forks share only privately owned metadata with bounded undo;
returned batch bytes are detached and expose neither a generic Plan nor mutable final inventory.
Only the exact pending batch advances once after successful publication. Refusal, cancellation,
failed apply or session exit abandons the cursor; restart performs redo and fresh audit.

## Early complete-command proof

The recovered historical export is pinned now at SHA-256
`be368b0524acacf375dcd28bf104646b691f9a88382f892ac58c6556c07c3d01`: 14,102,044 bytes,
2,894 records and 1,450 dependency edges. Adjacent original logs identify the earlier loaded/profiled
run; this is not original historical cryptographic attestation. The 3,094-record current roadmap
and separately named synthetic input are not substituted for this corpus.

An independently reviewed runner imported those unchanged bytes into fresh, disposable ROADMAP
fixtures. Baseline `2e281305c49badc68d1a46035ca4ff966cbaff7c` took 177.97 s wall / 158.44 s
user / 28.49 s system, RSS 115,195,904 bytes. Prototype
`441a3a8433b6db9ab639cf169c4a255770b736f8` took 60.60 s wall / 22.85 s user / 23.55 s
system, RSS 114,884,608 bytes: 65.95% less complete-command wall time, above the predeclared20%
threshold. Both commands exited0 with every one-second load sample below12 CPUs; maximum sampled
loads were10.90/11.02/10.48 and9.46/10.10/10.18. Input, runner, initial queue/policy, Go settings
and CPU identities matched. Source and binaries stayed frozen throughout both arms.

Both produced2,894 matching tickets/1,450 edges/414 receipts/head415; receipt audit was
CONSISTENT/AGREES and immediate reimport left all intent/state bytes unchanged. Initialization,
build and reimport are outside the timed denominator. Fresh destination does not mean flushed OS
caches; fixture gate definitions were not executed. No production import/cutover is claimed.
The earlier271s loaded/profiled result and the sandbox-rejected synthetic timing remain unqualified.

## Bounded invariants and discovered failure

Small new/revision parity compares receipt, descriptor, head, artifacts, every capacity prefix and
final metadata to ordinary Model. Further tests cover seed/accessor ownership, retained batches,
wrong/duplicate tokens, changed queue/policy/head/receipt/barrier/reservations, closed/zero cursors,
branch/replay/clock refusals, exact-limit/minus-one cost parity, overflow, encoding/PAX/padding,
reserve gaps, head digit growth and1/16/64 growing batches. Existing whole-export missing-reference,
cycle, gate and forward-reference checks remain before publication. Full selected package checks
and an independent review are required on the final source, not inferred from the early timings.

A deletion-between-batches regression initially exposed that generic settled publication permits
absence for pending redo. The approved repair is import-only: immediately before a *fresh* receipt
link, every posted ticket must have a pre-entry and its physical digest must equal that preimage,
including exact absence. Missing, replaced, already-post and wrong-type projections refuse; no
fresh receipt commits. Prior batches or prepared evidence may remain. Pending committed receipts
still recover through unchanged redo *before* the new audit and sequence. Receipt/post/head fault
restart witnesses cover that distinction. Cooperative locking/CAS is not hostile-editor atomicity.

Static proportional-work inspection: one seed clone/full validation/receipt-chain pass; per-batch
Digest, ticket decode, freeze and undo iterate only the closed batch/artifact limits. Import never
rebuilds the whole ticket graph per batch. Cost measurement encodes only bounded manifest metadata;
entry contributions update only touched files. Reserve-path hints are seeded once over occupied
names and advanced by actual insertions, not synthetic reserve simulations. Capacity forks copy
fixed-size totals/hints plus empty bounded undo maps, never the full retained inventory. The batch
growth witness preserves one owned inventory and independently revalidates its final public view.
This inspection and exact-prefix tests support proportional work; timing alone does not prove it.

## Evidence and closeout boundary

Scratch evidence: `/tmp/corvint-import-scaling-20260928`; recovered input, provenance, paired
summaries and runner outputs: `/tmp/corvint-1-0-orchestrator-20260928/import-beamfall`.
The final source adds cancellation and the fresh-preimage guard after the early pair; it requires
its own frozen focused gate and fresh candidate-only real-input qualification. Early matched-pair
identity is retained separately. Final local acceptance/integration/native completion is pending
at this source freeze; the coordinator owns native task writes. No full repository gate is claimed.

Original Corvint query, missing-receipt/prechange NOT_PRODUCED reasons and affected advice are
retained. Query/context, affected and enrolled CEM/OCM apply; retrieval-learning evaluations and
provider/UI/runtime routes do not. Workflow-only trace-fixed Corvint is separately qualified and
is not the candidate executable. One wrapper invocation's setpgid warning and failed test iterations
remain visible; a separate TERM regression proved all owned descendants gone before measurements.
Rollback restores per-batch public Model without migrating any durable state.
