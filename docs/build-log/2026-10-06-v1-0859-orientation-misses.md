# 2026-10-06: orientation misses in the subjectless context packet (V1-0859, V1-0431)

Human-owned intent: tickets V1-0859 (rc.2 orientation retrieval misses) and V1-0431 (first
criterion: lexical rows ordered by match strength, not a flat score). The 1.0.0-rc.2 untouched-
repository run (V1-0019) found the subjectless `corvint context --task` packet missing files the
baseline carried on fifteen public cases and on a private third-party repository. This entry is
the diagnosis of every public miss, the rule chosen, and the evidence that binds it. The private
leg (Beamfall) is not diagnosed here: its paths and content stay out of this repository, and no
digest of it was available to this change, so that leg is `NOT_RUN`.

Spec: `docs/specs/task-context-packet-v0.md`, TCP-V0-013 (amended) and new TCP-V0-059, TCP-V0-060,
TCP-V0-061; all experimental.

## Diagnosis

Reproduction: the origin/main build (`1.0.0-rc.2 (build 0)`) run at limit 20 against the parent
of each case commit, in a scratch clone of urfave/cli (public) and of this repository, from the
case data of the rc.2 run. The packets reproduced the rc.2 run sha-for-sha. For each missed file
the table names the rc.2 rule that dropped it; BM25 is the TCP-V0-014 score of the file for the
task, `rank` its position among all lexical hits of its class.

| case | missed file | class rank, BM25 | rc.2 rule that dropped it |
| --- | --- | --- | --- |
| urfave/cli 277cbea3 | `docs/v3/examples/flags/advanced.md` | doc 4/56, 10.85 | documentation quota: outscored every admitted code row (weakest 2.32); the quota admitted two documentation rows |
| urfave/cli 277cbea3 | `godoc-current.txt` | doc 8/56, 6.48 | documentation quota (same) |
| urfave/cli 277cbea3 | `testdata/godoc-v3.x.txt` | doc 9/56, 6.48 | documentation quota (same) |
| urfave/cli 277cbea3 | `flag_impl.go` | code 37/48, 1.20 | result limit: fifteen code positions, 36 code rows stronger |
| urfave/cli 277cbea3 | `flag_bool_with_inverse.go` | code 38/48, 1.19 | result limit (same) |
| urfave/cli 4c05a5cb | `docs/v3/examples/flags/advanced.md` | doc 3/46, 17.03 | documentation quota: outscored by two code rows only (weakest admitted code 5.60) |
| corvint fc594fa6 | `docs/DOGFOOD.md` | doc 7/460, 18.13 | documentation quota: above the weakest admitted code row (17.13) |
| corvint f451a34f | `docs/specs/agent-harness-integration-v0.md` | doc 5/602, 15.92 | documentation quota: above the weakest admitted code row (13.74) |
| corvint f451a34f | `README.md` | doc 60/602, 10.71 | result limit; four `pair` rows of one test file also held positions 13 to 16 |
| corvint f7f586bd | `tools/retrieval-bench/main_test.go` | code 22/3022, 18.20 | result limit: fifteen code positions (weakest admitted 18.70) |
| corvint f7f586bd | `tools/retrieval-bench/main.go` | code 28/3022, 17.66 | result limit (same) |
| corvint f7f586bd | `tools/retrieval-bench/README.md` | doc 25/659, 16.79 | result limit: below the weakest admitted code row |
| corvint d867347d | `docs/specs/proof-carrying-context-optimization-v0.md` | doc 10/473, 18.68 | result limit: below the weakest admitted code row (23.57) |
| corvint 4c607266 | `docs/specs/daily-change-evidence-workflow-v0.md` | doc 3/584, 27.43 | documentation quota: the strongest lexical hit of the task after two documentation rows; stronger than every code row |
| corvint 57f06a6a | `ROADMAP.md` | doc 23/614, 16.59 | documentation quota: above the weakest admitted code row (14.72), 22 documentation rows stronger |

Eight of the fifteen misses are documentation that outscored an admitted code row and was cut by
the two-row documentation quota of the old TCP-V0-013 (head of five code rows, two documentation
rows, remaining code, remaining documentation). Seven are plain result-limit drops: the file is
weaker than every row carried. No miss is a non-Go code file cut by its class: non-Go code already
competes inside the code class under TCP-V0-014, and the two non-`.go` misses carry a documentation
suffix (`.txt`, `.md`). Every lexical row scored a flat 300 (V1-0431's first criterion), so a
reader could not tell the strongest documentation hit of a task from the weakest admitted code row.

## Rule chosen

TCP-V0-013 as amended: the five-code head stays, then one merged TCP-V0-014 order over the
remaining code and documentation hits, documentation capped at half of the lexical fill
(`ceil(fill / 2)`, TCP-V0-059); the head gives way to that share when the fill is short.
Documentation beyond the share is appended after all code. A lexical or documentation row scores
`300 + round(299 x bm25 / strongest)` (TCP-V0-060), so order and score agree and no lexical row
reaches a relation tier. When the limit or the share omits a lexical hit, `coverage.uncertainty`
states the omitted class, the count, the share, and the strongest documentation row the share
omitted (TCP-V0-061); the member is absent when nothing was omitted.

Alternatives, simulated over the rc.2 hit dumps of the eight public cases (critical files carried
/ treatment-only misses, limit 20, before the implementation): rc.2 rule 23/16; merged order after
the head with no share 26/13; four-row quota 25/14; per-class maximum normalisation 27/12
(outlier-sensitive: one strong row rescales its whole class); round robin 26/13; merged with a
one-third share 26/13; merged with a half share 28/11 (chosen). The simulation is development
evidence over the same cases the rule is tuned on.

## Before and after

Same binaries, same clones, same case commits, limit 20; `before` is origin/main, `after` this
change. Critical files are the rc.2 run's `critical` list; a treatment-only miss is a critical
file the baseline packet carried and the treatment did not.

| case | critical carried before -> after | treatment-only misses before -> after | gained | lost |
| --- | --- | --- | --- | --- |
| urfave/cli 277cbea3 | 4 -> 6 | 5 -> 2 | `advanced.md`, `godoc-current.txt`, `godoc-v3.x.txt` | `flag.go` (not in the baseline packet) |
| urfave/cli 4c05a5cb | 9 -> 9 | 1 -> 0 | `advanced.md` | `help_test.go` (not in the baseline packet) |
| corvint fc594fa6 | 6 -> 6 | 1 -> 1 | none | none |
| corvint f451a34f | 4 -> 3 | 2 -> 3 | `agent-harness-integration-v0.md` | `cmd/corvint/host_adapter.go`, `cmd/corvint/host_adapter_test.go` |
| corvint f7f586bd | 0 -> 0 | 3 -> 3 | none | none |
| corvint d867347d | 0 -> 0 | 1 -> 1 | none | none |
| corvint 4c607266 | 1 -> 2 | 1 -> 0 | `daily-change-evidence-workflow-v0.md` | none |
| corvint 57f06a6a | 1 -> 1 | 1 -> 1 | none | none |
| total | 25 -> 27 | 15 -> 11 | 6 | 4 |

Regressions, reported not hidden: `corvint f451a34f` carries one fewer critical file. Its two lost
rows are code hits weaker than nine documentation rows the merged order now carries; the rc.2
packet reached them only because four `pair` rows of one test file and the quota kept
documentation out. `flag.go` and `help_test.go` were critical rows the rc.2 packet carried and the
baseline did not; the merged order carries stronger documentation in their place. The eleven residual treatment-only misses are seven result-limit drops
unchanged by any ordering rule (`flag_impl.go`, `flag_bool_with_inverse.go`, `README.md` of
f451a34f, the three retrieval-bench files, `proof-carrying-context-optimization-v0.md`),
`docs/DOGFOOD.md` (the 22nd row of the same packet at limit 24: two admitted `pair` rows and the limit keep it out at 20), `ROADMAP.md` (22 stronger documentation rows, share of 9),
and the two f451a34f rows above. Each after-packet states its omission in `coverage.uncertainty`;
for 57f06a6a the second line names the 22 documentation rows above a code row the fill carried
that the share omitted.

Packets in the small-fixture goldens: `internal/contextindex/testdata/context-recipe-default-golden.json`
keeps its twelve rows (`docs/guide.md` moves from row 6 to row 10 by strength and `docs/needle.md`
from row 6 to row 7, because the head of two code rows no longer counts the `mentioned`
`pkg/parser/parser.go` toward itself; scores 313 to 599);
`cmd/corvint/testdata/core-freeze/context-reserved-rows.json` gains the optional
`coverage.uncertainty` member (one line, limit 1), an additive member under CCF-V1-006 regenerated
in this change; `context-task.json` is unchanged (nothing omitted).

## Evaluation integrity

After this tuning the rc.2 cases (urfave/cli, this repository, and the private repository) are
development evidence: the rule was chosen on them and the numbers above cannot stand as held-out
validation. Held-out validation on a newly frozen repository (V1-0859 criterion 3, V1-0019) is
`NOT_RUN` here and belongs to the owner's untouched-repository run of rc.3.

Frozen evaluations: `corvint eval` goldens target another repository's frozen index and do not
exercise the subjectless packet of this tree (not applicable); the retrieval-bench samples are
not present locally and the 444 MB download needs an explicit approval (`NOT_RUN`); the blind
v2/v3 trials are query-based and do not read the lexical fill (not applicable). The private
third-party leg is `NOT_RUN`.

## Checks

- `TestTaskContextDocumentationCompetesByStrength`, `TestTaskContextDocumentationShareStatesTheOmittedClass`,
  `TestTaskContextLexicalScoreCarriesStrength`, `TestTaskContextLexicalFillCountsOnlyOpenPositions`
  (new), `TestTaskContextPlacesDocumentationAfterFiveCodeRows` (rewritten for the merged order),
  `TestTaskContextKeepsRoutedRowsWhenResultsAreWithheld` (withheld-line assertion added), the
  recipe and core-freeze goldens, and the context tests of `cmd/corvint` pass; `go vet` on
  `internal/contextindex` and `cmd/corvint` is clean.
- `TestAnalyzerSchemaInputs` (IDX-SNAP-V0-017) fails in this change: `taskcontext.go` is one of
  the pinned extraction inputs, and the audit over-invalidates consumer-only changes by design.
  The change does not alter extraction or pack encoding. Bumping `analyzerSchemaID` to
  `corvint-analyzer/104` and repinning the audit digest is left to the owner; until then the
  `internal/contextindex` package test is red on that audit alone.

## Review

Independent review (Codex, read-only, on the first commit of this change) returned six findings:

1. P1, `TestAnalyzerSchemaInputs` red: not fixable in this change; the schema bump to
   `corvint-analyzer/104` is the owner's step (Checks above).
2. P1, frozen before/after retrieval evidence: the retrieval-bench samples are not local and the
   download needs an approval this change does not hold; `NOT_RUN` (Evaluation integrity above).
   The eight-case before/after table is development evidence, not the frozen evaluation.
3. P2, a hit an earlier slot admitted (or the subject) counted toward the head and the share
   although `take` drops it as a duplicate: fixed; such a hit keeps its strength position and
   takes no head, share or fill position (TCP-V0-059 amended). The recipe golden moves
   `docs/needle.md` from row 6 to row 7 for this reason. Pinned by
   `TestTaskContextLexicalFillCountsOnlyOpenPositions`.
4. P2, the fill was computed before `reserve` prepended the governing, spec-mentioned and
   instruction-routed rows, so at a small limit the head spent a position the reservation then
   displaced: fixed; `compile` reserves before the lexical fill and the fill subtracts the
   reservations no earlier slot admitted. Pinned by the same test (limit 2, AGENTS.md, one code
   and one stronger documentation hit).
5. P2, under a TCP-V0-016 withholding `coverage.uncertainty` called the withdrawn hits "omitted
   by the result limit": fixed; the line names the `unsupported-conjunction` verdict and the
   share line is absent (TCP-V0-061 amended). Pinned by the same test and by
   `TestTaskContextKeepsRoutedRowsWhenResultsAreWithheld`.
6. P3, the residual-miss accounting named six misses while listing more: corrected to the eleven
   treatment-only misses the table carries.

The eight public cases were rerun with the fixed binary: the per-case critical counts, gains and
losses are unchanged (27 carried, 11 treatment-only misses); the share lines changed where a
reservation now counts (`9 of 18` instead of `10 of 19` positions on the Corvint cases) and
where the comparison row is now the weakest code row the fill carried (57f06a6a: 22, not 133).

The second review (Codex, read-only, on the fixed commit) returned three findings:

7. P2, under a TCP-V0-016 withholding a reservation the result limit cut was counted as
   withheld, although the verdict withdraws no reservation: fixed; `lexicalCoverage` counts a
   reserved hit the packet does not carry as omitted by the limit and the ordinary hits as
   withheld, on two lines (TCP-V0-061 amended). Pinned by
   `TestTaskContextStatesAReservationTheLimitCutAsOmitted` (routed fixture, limit 1).
8. P2, the share line counted every deferred documentation hit that outscored a carried code
   row, including hits the five-row head displaced, which the same packet carries without the
   share: fixed; the comparison row is a code row the fill carried past the head
   (`lexicalHead`), so a hit the head or the limit displaced is counted in the first line only
   (TCP-V0-061 amended). Pinned by `TestTaskContextDocumentationShareStatesTheOmittedClass`
   (six code and eight documentation hits at limit 12 keep the share line; three and six at
   limit 6, and one and six at limit 4, lose it) and
   `TestTaskContextLexicalFillCountsOnlyOpenPositions` (the mentioned-row case loses it).
9. P3, TCP-V0-059 promised at most `ceil(fill / 2)` documentation positions and TCP-V0-013 that
   no documentation hit is cut while a weaker code hit is carried, neither of which the deferred
   tail and the share hold: the spec now states that the deferred tail fills the positions no
   code hit takes and qualifies the strength-order guarantee to the share (TCP-V0-013's head
   sentence: past the share the head and the share together may cut a stronger documentation
   hit while weaker code hits are carried, the three-code six-documentation limit-6 case).

Rerun on the eight public cases after these fixes: every packet and every uncertainty line is
identical to the previous rerun (27 carried, 11 treatment-only misses); the four share lines
each name a code row the fill carried past the head, and the four cases without one had no
share line before.

The third review (Codex, read-only, on the second fixed commit) returned three findings:

10. P1, `TestAnalyzerSchemaInputs` red: unchanged, the owner step of item 1; the digest at the
    final commit is in the handoff.
11. P2, `lexicalHead` named the head's members by path, which TCP-V0-035's opt-in reorder
    invalidates: a recent code hit promoted into the head left the member it displaced counted
    as a code row carried past the head, so the share line claimed a deferred documentation hit
    lost its position to the share. Fixed; the head is a count, and a code row is carried past
    the head when the fill's code rows outnumber the head positions (TCP-V0-061 amended).
    Pinned by `TestTaskContextShareLineIsCountedThroughTheRecencyReorder` (three old code
    hits, six stronger documentation hits and one recent code hit at limit 6 under
    `CORVINT_CONTEXT_RECENCY=on`: the recent row holds a head position and the packet states
    the limit line alone).
12. P3, TCP-V0-013 said a stronger documentation hit is never cut by the head alone, which the
    three-code six-documentation limit-6 case contradicts: qualified to the share, with the
    `lexicalRows` comment and item 9 aligned.

The fourth review (Codex, read-only, on the third fixed commit) returned two findings:

13. P2, a held code row's lexical copy, promoted into the head by TCP-V0-035's reorder, was
    said to spend a head position so that the counted head suppressed a due share line: not
    reproduced; `take` drops a chosen copy without counting it, so the packet and both lines
    equal the reorder-free ones. Pinned by the held-copy subtest of
    `TestTaskContextShareLineIsCountedThroughTheRecencyReorder` (a mentioned, recent
    `src/seed.py`, five code and six documentation hits at limit 10).
14. P2, the share line read the documentation the share kept out from the fill's deferred
    tail, which TCP-V0-035's reorder of the documentation rows invalidates: a recent deferred
    hit promoted into the share left the hit it displaced uncounted and named the wrong
    strongest path. Fixed; `lexicalDocumentation` keeps every documentation hit that competed
    for a fill position and the line counts those the packet does not carry that outscore a
    carried code row (TCP-V0-061 amended). Pinned by the promoted-documentation subtest (six
    code and eight documentation hits at limit 12, a recent `docs/h.md`: two omitted by the
    share, the strongest `docs/f.md`).

The eight public cases rerun on the fourth fixed commit carry the same 27 hits and miss the
same 11, every packet byte-identical to the third rerun. The fifth review (Codex, read-only, on
the fourth fixed commit; its sandbox could not run the Go tests, and it modelled the reorder-free
accounting against the earlier rule over 30,000 cases with held paths, reservations and
truncation, finding no difference) returned one finding:

15. P2, the share line's comparison row was the weakest carried code row of any position, so a
    weak recent code hit that TCP-V0-035's reorder moved into the head stayed the comparison
    row and the line claimed share omissions the packet, whose code row past the head outscored
    every documentation hit, did not make. Fixed; the comparison row is the weakest lexical
    code row after the first `lexicalHead` of them in packet order (TCP-V0-061 amended).
    Pinned by the weak-recent-hit subtest of
    `TestTaskContextShareLineIsCountedThroughTheRecencyReorder` (five old code hits above
    eight documentation hits above one recent code hit at limit 12: the reorder-free packet
    states the share line naming `docs/g.md`, the reordered packet the limit line alone).

The sixth review (Codex, read-only, on the fifth fixed commit) approved with no findings: the
carried code rows stay a descending-bm25 subsequence with the reorder off, so the comparison row
past the head equals the earlier minimum whenever one exists (a 50,000-case accounting model
found no difference across held paths, reservations, truncation, withholding and promotion), and
the eight public cases rerun on that commit are byte-identical to the fourth rerun.

## Rollback

Revert this change: the old TCP-V0-013 order (five code, two documentation, remaining code,
remaining documentation), the flat 300 score, no `coverage.uncertainty`, and the two goldens.
