# Sealed continuation accepts the lane closeout's own outcome record

V1-3194. The V1-1894 sealed continuation (`LCP-V0-040`) refused the supported workflow: after an
enrolled strict `finish`, the one-command lane closeout's own `dogfood record` rewrites
`<gitdir>/corvint/local-outcome.json`, so the coordination pin no longer matched and sealed
`finish` returned a bare `sealed-evidence-drift` (retained in the V1-1894 evidence as
`finish-seal-1.stderr`). The only recovery was a hand edit of private state, which the contract
forbids.

## Decision

Sealed `finish` re-pins that single coordination artifact when the replacement is provably the
closeout's record (`LCP-V0-041`). Provenance comes from three independent sources rather than a new
state field: Git reproduces the outcome's base, target, tree, candidate set and path-admission
digests; the evidence target's trace store holds the enrollment's own row with the same admitted
set and a newest, different-task row equal to the outcome's trace; and the coordinator report is
complete for the original base and evidence target and binds the new digest. The OCM status pin
must not change. The re-pin happens only inside `finish`; status and host events still validate
read-only through the pure `validateSealed`. Acceptance forces a fresh continuation report set
whose text names the replaced and replacing digests, so a human review covers the replacement
before completion.

Alternatives rejected: (1) dropping the outcome pin, which would accept any rewrite; (2) a new
`replacedOutcome` state field, which older runtimes could not decode and which adds nothing the
trace store and report do not already prove; (3) re-pinning silently without a new review, which
would let a correct-looking but unreviewed outcome complete.

Every `sealed-evidence-drift` refusal now carries a fixed-label detail
`artifact=<label> reason=<reason>` with pinned/current digests for byte changes (`LCP-V0-042`),
surfaced as `SealedDriftError` and in the CLI envelope `detail`.

## Evidence

`TestLocalCompletionSealedCloseoutRepin` runs the enrolled finish, the real `dogfood finish-lane`,
verify and sealed finish to `satisfied` with no manual edits, then positive-first controls for
tampered, retargeted, unreferenced and unrecorded replacements and a foreign OCM status change,
each refusing with the named artifact and digests and leaving state unchanged. Same-UID tampering
that reproduces every source (Git, trace store and report) is outside the threat model, as in
`LCP-V0-040`.

Rollback: revert the change. Enrollments that already re-pinned keep valid state bytes; others
return to the `LCP-V0-040` refusal with their evidence retained.

## Interrupted final check (V1-3192)

The same lane delivers `LCP-V0-043`: a final strict check ended by the caller's context (the CLI
maps SIGINT/SIGTERM to cancellation) now reports `finish-interrupted` with the step, retained logs
and rerun command, instead of the misleading `final-check-failed` observed in the V1-1894 run.
Only the caller's context counts; the step's own deadline stays a failure. No state is written in
either case, so the rerun reuses bound results. `TestLocalCompletionRealEvidenceWorkflow` covers
the interrupted, genuine-failure and recovery cases.
