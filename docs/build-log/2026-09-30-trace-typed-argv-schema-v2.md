# Typed argv trace verification — issue 408 / V1-0559

## Accepted intent and boundary

Russell Lewis accepted the schema-v2 proposal on 2026-09-30 (proposal SHA-256
`707d696a76b240423084f10a44ddfae22b2b5da46391eedde5aa306be1ec716f`).
`LTPM-V0-013..014`, `LTA-V0-013`, `LOD-V0-035` and `LAC-V0-038` govern this experimental slice.
The source base is `4663ce7c0f079a111e5e1a6f08de80ae703730e2`.

The recorder adds repeatable `--verify-argv-json`; argv stays literal data. Only invocations
containing typed argv produce schema 2. V1 identity, bytes, command grammar, stored screening and
frozen conformance expectations remain unchanged. Typed entries have a closed union, deterministic
canonical identity, bounds, and shared decoded/structural credential screening. Arbitrary opaque
base64/hex/percent decoding is excluded. The screening claim remains bounded pattern detection.

All trace consumers admit v2 with shared validation. A mixed dashboard store is one source; each
physical revision file is one member/cohort, counted once. Its distinct v2 registry hash is selected
only when an admitted v2 row exists. Console and skill export display labelled canonical JSON;
query/eval scoring uses the existing task/outcome/path projection. Migration and whole-file retention
preserve typed entries. Rollback stops new v2 writes but retains readers for already-stored rows.

## Design review and observed repairs

The independent Gate A review passed after specifying whole-store mixed-version dashboard identity,
value-based stored canonicality, and learned-path qualification. Decision 0088 / `GOC-V0-002`
retire the Python engine; the relevant development gate uses native baseline/learned arms and frozen
independent expectations, never a reconstructed Python oracle.

The mixed snapshot integration test initially failed because compilation still required the frozen
v1-only registry hash. Compilation and verification now select the exact v1 or extended-v2 hash from
the source set. V1-only before/after snapshot bytes are identical. The old schema-error assertion also
exposed the need for the new refusal to name schema 2; this changed only the typed refusal message.

## Verification and retained limits

Focused package tests passed for trace, secret screening, record adapter, dashboard, console,
skill export and eval; migration adapter has no package-local test files and is covered through
trace transformation and CLI migration tests. Relevant CLI record, migration, eval, query, batch,
skill-export and accuracy tests passed. New cases include an independently authored canonical
preimage/trace ID, malformed and escaped values, secret refusal before store mutation, a nonexecution
sentinel, mixed physical dashboard accounting, and typed consumer admission/rendering.

The registered five-repository native development qualification initially reported `NOT_RUN`
because pinned inputs were absent. After all five exact clean checkouts were prepared, the native
baseline (`4663ce7c`) and source candidate (`c040a813`) runs passed with no skipped repositories.
Both produced baseline precision 0.859335 and learned-arm precision 0.849070 (floor 0.80), zero
critical misses, and abstention/epistemic accuracy 1. The learned arm changes two precision cases
(delta -0.010265); the other quality deltas are `not distinguished`, with zero changed cases.
There is no quality delta between the two source revisions. This is development qualification
on the registered frozen fixture, not independent external outcome evidence or release promotion.
The independent source review found two eval-fixture regressions: indented v2 objects must be
validated before JSONL compaction, and nonselected v1 fixtures must retain whole-document structural
validation. Both were repaired with targeted regressions. The aggregate equality check also includes
its v2 selector. The standalone conformance verifier now reads typed rows independently and verifies
actual mixed-store producer output; its ASCII trace encoder is separate from its snapshot encoder.
The final independent source review passed after two bounded repair cycles (reviewed diff SHA-256
`98b8d3cbf7857d29f452fc81777fa5e4e881fa939f75c3b77f9dcc1ce9c24e32`).
The complete standalone conformance suite and changed-package vet passed after the last repair.
Pre-existing v1 standalone conformance/producer authority and outcome-bucket differences are
retained separately as native ticket `V1-0571`; frozen v1 expectations were not rewritten.
Immutable evidence binding remains required before publication.
No repository-wide `make gate` was requested; scoped focused validation is not equivalent to it.

Corvint prechange query/impact and affected-plan evidence were retained. The required initial
`dogfood-change` ran before editing at an unchanged base and returned `NOT_PRODUCED` for a CEM
(no diff), OCM, and local outcome (no source paths); those initial failures are retained rather than
reported as passed. Final CEM/OCM/outcome binding is post-commit under `docs/DOGFOOD.md`.
