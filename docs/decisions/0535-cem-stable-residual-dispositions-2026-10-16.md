# Decision 0535 — Dispositions of the decision 0532 open items: pointers kept, missing objects named, refused base reads not digested

Date: 2026-10-16. Status: accepted. Authority: owner delegated to the orchestrator (2026-10-16);
decided by the orchestrator. Tickets: V1-1296, V1-1297, V1-1299, V1-1307.

## Context

Decision 0532 left three choices to the owner and recorded a fourth limit:

- V1-1296: remove the three dangling digest-pinned pointers that `CEM-V1-261` records
  (`parentPlan` in `stable/MIGRATION.json` and in its inherited S0E copy, and
  `repair6Routing.sourceReview.path` in `repository-envelope-packet/INPUTS.json`).
- V1-1297: give a missing pinned Git object its own repository code on both consumers.
- V1-1307: amend `DCW-V0-047` so that a refused base `cem status` read records `NOT_PRODUCED` and
  its refusal code instead of a base digest. Decision 0532 recorded this as `DCW-V0-072`.
- V1-1299: a cem/0.1 source's out-of-band patch digest. `CEM-V1-104` holds this as a recorded
  limit.

## Decision

1. V1-1296: declined. Sealed evidence stays immutable. The three pointers sit in sealed bytes that
   are pinned by digest and already marked informative. Removing them would rewrite and re-pin the
   stable manifest, the packet manifest and `INPUTS.json`, and the evidence would change only to
   tidy history. `tools/cem-stable-regen --check` already reports each pointer as a named, recorded
   exception (`CEM-V1-261`), and that record fails if a pointer ever resolves or changes. No packet
   byte changes.
2. V1-1297: accepted. When Git cannot resolve a pinned full object ID, both consumers probe
   `git cat-file -e`. An absent object (probe exit status exactly 1) is `UNSUPPORTED`
   `repository` `repository-object-unavailable`. A present object that is unreadable or not a
   commit stays `git-read-failed`, and so does a probe that exits otherwise (for example 128). Labelling an absent object `git-read-failed` claims a read failure that never
   happened, which is invented certainty about the cause (product invariant 2).
   - `CEM-V1-103` is amended and supersedes the decision 0532 hold. The portable consumer charges
     the probe as an `object-existence-probe` ledger operation.
   - The sealed S0E cells `missing-base`, `shallow-missing` and `missing-promisor` re-emit `code` and
     `verdictSha256` as a new sealed version, through `make cem-stable-regen-replace`. The old bytes
     stay in Git history.
   - `missing-promisor` is included because its pinned object is also absent and both consumers
     now report the new code for it. `corrupt-base` keeps `git-read-failed`.
   - The re-emission needed two `CEM-V1-091` extensions: a cell-only run that leaves
     `FULL-RESULT-CASES.json` byte-identical, and an explicit `--cell-values` entry for a cell whose
     matching cases disagree on the re-emitted member. A cell with no entry is still refused.
3. V1-1307: accepted. `DCW-V0-047` is amended: a base verifier that refuses the read (an exit other
   than 0 or 1) records `NOT_PRODUCED` and its refusal code in place of a base digest. This applies
   to the SUMMARY `base=` field and to the report's `baseIndependence` member. This supersedes
   `DCW-V0-072`. The digest of empty output looks like an observed read that never happened, and
   that is invented certainty (product invariant 2). A rejecting base (exit 1) still records its
   stdout digest.
4. V1-1299: closed as obsolete; no code change. The cem/0.x to cem/1.0 converter was removed by
   `CEM-V1-204`. On origin/main no `internal/cem/convert` package or `cem convert` route exists, so
   no producer can reach a cem/0.1 out-of-band patch digest. What remains are the retired-spec
   refusal (`wire.RetiredSpec`) and the provenance `sourceSpec` enumeration, which records
   historical conversions. Neither is dead code. The `CEM-V1-104` limit stays as historical text.

## Limits

`make gate` is `NOT_RUN`. The native stable verifier's result for the re-emitted cells is not run
as a cell: no native test executes envelope cells. Two pieces of evidence cover it instead:
- `TestResolveMissingPinnedObjectUnavailable` (`internal/cem/gitauth`)
- the cell verdict digest `a9ae2c5eee6d2ab67c75a2c746fdd855f444f13c93b252e7ef1640c70eca3f26`, which
  equals the value 11 sealed native full-result cases already carry with the same keys

The portable consumer runs every cell.

## Rollback

Revert the change that carries this decision, then regenerate `docs/specs/REQUIREMENTS.tsv`. The
re-emitted cells, the stable manifest and the tuple bindings return to their 0532 bytes. Both
consumers report `git-read-failed` again, and `DCW-V0-072`'s empty-output digest returns.
