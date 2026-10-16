# Decision 0530 — Citation measure amendments and recorded value re-emission of the frozen packet

Date: 2026-10-15. Status: accepted by the owner for the scope below. Authority: owner approvals in
chat, 2026-10-15: "approve V1-1331, V1-1335 and V1-1332 amendments", and option (a), a recorded
re-emission of the frozen packet, relayed by the coordinator.

## Context

Three CEM-V1-080..082 counting rules disagreed with what they are meant to measure:

- V1-1331: `concentratedHunks` counted every supported hunk when no evidence was shared
  (`maxFanOut` 1), so a map whose hunks all cite distinct evidence looked fully concentrated.
- V1-1335: the annotation stop list was the whole `lexical-relevance-floor-v0` list, which holds
  21 code words (`return`, `value`, `true`, ...), so a short code line such as `return value - 1`
  was read as an annotation.
- V1-1332: `samePathCitations` named the hunk's post-image path, which a whole-file deletion does
  not have, and `CEM-V1-080` still named `cem/0.2` and `cem/0.3` maps after the `cem/1.0` cutover.

The V1-1331 correction changes the `citation` block, and with it `verdictSha256`, of sealed S0E
cases and envelope cells. `CEM-V1-091` let the frozen packet change only by an additive member
regeneration, so no accepted correction of an existing member's values could reach it.

## Decision

1. `CEM-V1-080`, `CEM-V1-081` and `CEM-V1-082` in `docs/specs/cem-stable-v1.md` and
   `protocol/cem-1.0/stable/ALGORITHMS.md` are amended: `concentratedHunks` is 0 when `maxFanOut`
   is below 2; annotation uses 39 stop terms, the floor list without the 21 code words; the hunk
   path is the base path for a whole-file deletion and applies to every rule that names a path;
   retired 0.x maps carry no tier.
2. `CEM-V1-091` and the frozen-packet sentence are amended: an existing member may receive a
   recorded value re-emission, only through `make cem-stable-regen-replace`. The tool takes a
   record of the reason and each predecessor value digest, refuses any changed value the record
   does not name, any recorded value that does not change and any predecessor digest that differs,
   and appends a `CHANGES.md` section. Values are never hand-edited.
3. The sealed values are re-emitted through that mode, and the re-emittable baselines
   (`citation-vectors` `clean`, `tamper-matrix`, `interop/cem01-go/testdata/citation` goldens and
   any drifting `cmd/corvint`, `neutral-criterion` or `receipt-authentication` goldens) are
   regenerated from the native verifier.

## Scope note for the owner

The approved re-emission named S0E cases 14 and 52 and the ordinary-, origin- and linked-sha1
envelope cells. The same V1-1331 correction also changes case 21
(`go1.24.13-nonstructural-positive`) and eight further cells: the relative-sha1 cell, the four
`-sha256` forms, and the comments, shallow-complete and hostile cells. The re-emission recorded
all of them (3 cases and 11 cells, `citation` and `verdictSha256` each), because the stable tests
compare every sealed value with the native result and a partial re-emission fails them. Owner
confirmation of the wider set is requested; until then the PR is not armed for auto-merge.

## Limits

Historical build-log baselines and the 591-map report keep their pre-amendment values and are
not rewritten. The measures stay descriptive: no threshold, cut-off or verdict depends on them.
`make gate` is `NOT_RUN`.

## Rollback

Revert the change that carries this decision. The re-emission's `CHANGES.md` section records
every predecessor digest, so the reverted packet can be checked against it; then regenerate
`docs/specs/REQUIREMENTS.tsv`.
