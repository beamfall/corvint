# Decision 0532 — cem/1.0 stable consumer residuals: drift charge, protocol pointers, missing objects, cem/0.1 patch digests and the refused base status digest

Date: 2026-10-15. Status: accepted under the owner's standing auto-accept (2026-10-11) for the new
requirements `CEM-V1-260`, `CEM-V1-261` and `DCW-V0-072`; the in-task choices for V1-1297, V1-1299
and V1-1307 were delegated to the lane by the orchestrator (owner delegation of 2026-10-05). No
accepted requirement's behaviour changes. Three possible amendments are left to the owner below.

## Context

Five residuals from the cem/1.0 cutover (V1-1275..V1-1278) remained open:

- V1-1295: on the f0fda542 map's `UNSUPPORTED` `unsupported-resource-limit` result the native
  verifier wrote 68 drift rows and the portable consumer 102, so `verdictSha256` differed.
- V1-1296: `protocol/cem-1.0/stable/MIGRATION.json` names a missing `../migration-plan.json`.
- V1-1297: a missing pinned Git object reports `git-read-failed`, not a distinct code.
- V1-1299: a cem/0.1 source's out-of-band `patchSha256` gave a converted map `REJECT`
  `patch-digest-mismatch`; refuse it with a new `CEM-V1-101` code or keep the `CEM-V1-104` limit.
- V1-1307: under an admitted `DCW-V0-047` declaration a base verifier that refuses the `cem status`
  read is recorded with the digest of empty output.

## Decision

1. V1-1295: the difference was the native drift stage reading each changed record's base blob a
   second time. `CEM-V1-260` fixes the drift charge for both consumers (one target path lookup, and
   one target blob read only for a changed regular-file entry, searching the span bytes the evidence
   stage authenticated) and adds the shared case `protocol/cem-1.0/stable/drift-resource-limit/`,
   run at a seam-lowered limit of 113 operations.
   The partial drift list stays in an `UNSUPPORTED` result, as V1-1295 decided on 2026-10-14.
2. V1-1296: `CEM-V1-261` makes `tools/cem-stable-regen --check` refuse a dangling digest-pinned
   pointer under `protocol/cem-1.0`. The three dangling pointers it finds (`parentPlan` in
   `stable/MIGRATION.json` and in the packet's inherited S0E copy, and
   `repair6Routing.sourceReview.path` in `repository-envelope-packet/INPUTS.json`) sit in sealed,
   digest-pinned packet bytes that already mark them informative, so they are recorded as named
   exceptions, not removed.
3. V1-1297: keep `git-read-failed`. The sealed S0E cells `missing-base` and `shallow-missing` pin it
   on both consumers, and a distinct code needs an object-existence probe that changes the sealed
   operation ledgers. Since `CEM-V1-204` removed the converter, the `CEM-V1-103` path no longer
   arises from Corvint's own producer. `CEM-V1-103` cites this decision.
4. V1-1299: keep the `CEM-V1-104` recorded limit and add no `CEM-V1-101` code. The converter no
   longer ships (`CEM-V1-204`), so a new refusal code would be unreachable. `CEM-V1-104` cites this
   decision.
5. V1-1307: keep the empty-output digest and document it. `DCW-V0-072` states that the base digest
   is the digest of the base verifier's stdout whatever its exit status, so a refused read records
   `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`, and that a successful
   `cem status` always writes its canonical line, so that digest never means a successful empty read.
   Recording `NOT_PRODUCED` and the refusal code instead would change the `DCW-V0-047` field set,
   an accepted requirement, so it is not done here.

## Owner decisions left open

- Remove the three recorded dangling pointers: rewrite `stable/MIGRATION.json`, its inherited copy
  and `INPUTS.json`, and re-pin the stable manifest, the packet manifest and `INPUTS.json`
  `originalS0EPublicFiles` (a sealed-byte change).
- Give a missing pinned object its own repository code: both consumers probe object existence, and
  the sealed S0E ledgers and cells `missing-base` and `shallow-missing` are regenerated.
- Amend `DCW-V0-047` so a refused base read records `NOT_PRODUCED` with its refusal code in place of
  the base digest.

## Limits

`make gate` is `NOT_RUN`. The shared drift case lowers the operation limit to 113 through each
consumer's test seam, because spending the full 1,024 operations took 14 minutes natively and passed
the 30-minute verification deadline on the portable consumer on a loaded host; a charge difference
that shows only past operation 113 is caught by re-running a full-limit map, not by the case. It is
skipped where stable process containment is not qualified.

## Rollback

Revert the change that carries this decision, then regenerate `docs/specs/REQUIREMENTS.tsv`. The
native drift re-read returns, V1-1295 reopens as a recorded difference, `--check` stops scanning
protocol pointers and `DCW-V0-072` goes.
