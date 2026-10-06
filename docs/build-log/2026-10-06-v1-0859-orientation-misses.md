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
A documentation hit takes a share position only through the share's gate: it outscores the lead
(the strongest code hit competing for the fill) or it is one of the first two documentation hits
that do not (`contextDocumentationQuota`, the rc.2 two-row rule kept as the floor); documentation
beyond the ceiling or the gate is appended after all code. The gate is the same day's amendment,
chosen on the five-pull-request replay below. A lexical or documentation row scores
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

The table is the gated share; the half share alone carried 27 and missed 11 (4c05a5cb 9, losing
`help_test.go`; f451a34f 3, losing `cmd/corvint/host_adapter.go` and
`cmd/corvint/host_adapter_test.go` for `agent-harness-integration-v0.md`).

| case | critical carried before -> after | treatment-only misses before -> after | gained | lost |
| --- | --- | --- | --- | --- |
| urfave/cli 277cbea3 | 4 -> 6 | 5 -> 2 | `advanced.md`, `godoc-current.txt`, `godoc-v3.x.txt` | `flag.go` (not in the baseline packet) |
| urfave/cli 4c05a5cb | 9 -> 10 | 1 -> 0 | `advanced.md` | none |
| corvint fc594fa6 | 6 -> 6 | 1 -> 1 | none | none |
| corvint f451a34f | 4 -> 4 | 2 -> 2 | none | none |
| corvint f7f586bd | 0 -> 0 | 3 -> 3 | none | none |
| corvint d867347d | 0 -> 0 | 1 -> 1 | none | none |
| corvint 4c607266 | 1 -> 2 | 1 -> 0 | `daily-change-evidence-workflow-v0.md` | none |
| corvint 57f06a6a | 1 -> 1 | 1 -> 1 | none | none |
| total | 25 -> 29 | 15 -> 10 | 5 | 1 |

Regressions, reported not hidden: `flag.go` was a critical row the rc.2 packet carried and the
baseline did not; the merged order carries stronger documentation in its place. The ten residual
treatment-only misses are seven result-limit drops unchanged by any ordering rule (`flag_impl.go`,
`flag_bool_with_inverse.go`, `README.md` of f451a34f, the three retrieval-bench files,
`proof-carrying-context-optimization-v0.md`), `docs/DOGFOOD.md` (the 22nd row of the same packet
at limit 24: two admitted `pair` rows and the limit keep it out at 20), `ROADMAP.md` (22 stronger
documentation rows, share of 9), and `agent-harness-integration-v0.md` of f451a34f, which the
gate defers: no documentation hit of that task outscores the lead `.taskman/tickets/V1-0298.json`
and two documentation hits precede it, so it sits past the quota as it did under the rc.2 rule. Each after-packet states its omission in `coverage.uncertainty`;
for 57f06a6a the second line names the 22 documentation rows above a code row the fill carried
that the share omitted.

Packets in the small-fixture goldens: `internal/contextindex/testdata/context-recipe-default-golden.json`
keeps its twelve rows (`docs/guide.md` moves from row 6 to row 10 by strength and `docs/needle.md`
from row 6 to row 7, because the head of two code rows no longer counts the `mentioned`
`pkg/parser/parser.go` toward itself; scores 313 to 599);
`cmd/corvint/testdata/core-freeze/context-reserved-rows.json` gains the optional
`coverage.uncertainty` member (one line, limit 1), an additive member under CCF-V1-006 regenerated
in this change; `context-task.json` is unchanged (nothing omitted).

## Held-out replay and the share gate

A read-only replay of five merged pull requests of this repository (#571, #614, #594, #557,
#610; the rc.3 batch integration's survey, 2026-10-06) ran `corvint context --limit 20` at each
merge base with the pull-request title as the task and took as gold the changed files that
existed at the parent. It found the half share regressing recall against main: 18/59 to 15/59,
#594 falling 4 to 2 (`internal/tasks/cli/cli.go`, `internal/tasks/store/release.go`) and #610
2 to 1 (`internal/tasks/dispatch/loop.go`), while halving the pair rows that point outside the
changed tree. The cause: `fill` 18 leaves nine positions to documentation, and this tree's
generic prose (`docs/TASKS-SUPERVISION.md`, decision 0021, the browser specs, the closed
`docs/BUILD-LOG.md`) scores between the strongest code hit (a `.taskman` ticket, bm25 35.85 for
#594) and the head's weakest (23.10), so the share admitted it ahead of the gold code at the
sixteenth code position. #594's gold needs documentation held to two rows; the rc.2 quota did
that for every task, the half share for none.

Rules screened by simulation over the hit dumps (replay recall of 59; public critical hits of
67 over the eight rc.2 cases; the simulation reproduces every measured packet except the pair
victim of two cases): half share 16/28 (measured 15/27); rc.2 two-row quota 18/26 (measured 25);
a floor at the head's weakest code hit 18, but #594 at 3; a floor at the third code hit 16/27
(fc594fa6 loses `local-trace-producer-migration-v0.md`, bm25 32.29, under three code hits); a
floor at the second code hit plus the quota, #594 at 3; the quota plus the lead (chosen) 18/29.
Excluding closed or redirect documents (`docs/BUILD-LOG.md`, `docs/agent-memory/`) changed no
number on either set and was not taken: a path rule for no measured gain. The gate keeps the
rc.2 quota as the floor and opens the half share only to documentation that leads the code, the
shape of a documentation task; the one-third share screened on the rc.2 cases stays weaker on
both sets.

Measured with the branch binary on the survey's worktrees (limit 20, gold rows carried):

| pull request | gold | main | half share (4611f2e5) | gate |
| --- | --- | --- | --- | --- |
| #571 | 10 | 5 | 5 | 5 |
| #614 | 8 | 5 | 5 | 5 |
| #594 | 24 | 4 | 2 | 4 |
| #557 | 8 | 2 | 2 | 2 |
| #610 | 9 | 2 | 1 | 2 |
| total | 59 | 18 | 15 | 18 |

No pull request loses a gold row against main. Pair rows over the five packets: main 16, of
which 10 name a counterpart elsewhere in the tree; gate 14 and 8 (the Go same-directory pair rule
of the next commit takes the rest). On the eight public cases the gate carries 29 critical rows
and misses 10 (the table above) against the half share's 27 and 11, no case losing a row to it.

Sensitivity: in this repository's cases `.taskman/tickets/*.json` count as code and set the
lead; a tree whose strongest hits are prose reads as a documentation task and keeps the half
share, the intended reading, while a tree whose code class scores like prose would gate
differently. The replay is development evidence now that the gate is tuned on it; held-out
validation stays `NOT_RUN` below.

## Go pairs share the directory

The same survey saw `internal/trace/store_test.go` carried as the `pair` of
`internal/tasks/store/store.go`: a stem coincidence across packages, high in score (900) and
wrong, since a Go `_test.go` lives in the directory of the package it tests. Over the five replay
packets main carried 16 `pair` rows, 10 of them naming a counterpart elsewhere in the tree, and
the gated share 14 and 8. The rule (TCP-V0-004 amended): a Go file pairs only with a Go
counterpart in its own directory. It is wider than a rule over two Go files because the replay
also carried two JavaScript tests as the pair of a Go source (`integrations/pi/workflow.test.mjs`
for `internal/tasks/store/workflow.go`, `internal/jstestprovider/testdata/external/retry.spec.cjs`
for `internal/tasks/wire/retry.go`), the same class of wrong row; a Go test is Go. The
mirrored-directory, elsewhere-in-the-tree and module-directory relations stay for the other
conventions, and `pairConfidence` is unchanged. The test slot's stem signal reads
`pairRelation` and carries the rule, so a Go test of another package binds by an import edge, a
declared name or its test name alone (`creditMirrored` now skips a candidate with no relation
instead of crediting it an empty one).

Measured on the replay: every pull request keeps its gold count (18/59, the table above); the
five packets carry 10 `pair` rows, every one a same-directory Go counterpart but
`integrations/pi-protected/runtime.test.mjs` for `integrations/opencode/src/runtime.js`, a
JavaScript pair outside the rule. On the eight public cases the critical rows carried stay 29
and the treatment-only misses 10; f451a34f's four cross-tree `main_test.go` pairs of
`conformance/host-lifecycle-v1/main.go` become that file's own `main_test.go`, `cmd/corvint/help_test.go`
and `script/check-hostile-regressions_test.sh` plus one more lexical row, and f7f586bd and
fc594fa6 each swap one cross-tree pair for a same-directory one. The goldens are unchanged.

Pinned by `TestPairRelationGoCounterpartsShareTheDirectory` (the relation table, both
directions, the two JavaScript-for-Go rows, a Ruby spec beside a Go file) and
`TestTaskContextCounterpartElsewhereIsAMediumPairRow` (decision 0026's packet-level pin, kept
outside Go). `TestCreditMirroredMatchesFullScan`'s fixture gains a Go test of another
directory, which the stem index must not credit; the test-link fixture's `render` pair moves
into `src/render` (the same-directory read of the same signal); the pair-slot omission fixture
is JavaScript; the cochange fixture drops its Go test of another directory, whose medium pair
row the new JavaScript test pins instead.

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
  (new), `TestTaskContextPlacesDocumentationAfterFiveCodeRows` (rewritten for the merged order,
  then for the gate: the two documentation rows past the quota follow the code rows although one
  outscores a carried code row),
  `TestTaskContextKeepsRoutedRowsWhenResultsAreWithheld` (withheld-line assertion added),
  `TestTaskContextShareLineIsCountedThroughTheRecencyReorder` and
  `TestTaskContextShareLineSurvivesPairPromotion` (new; both re-pinned for the gate), the
  recipe and core-freeze goldens, and the context tests of `cmd/corvint` pass; `go vet` on
  `internal/contextindex` and `cmd/corvint` is clean.
- `TestAnalyzerSchemaInputs` (IDX-SNAP-V0-017): `taskcontext.go` is one of the pinned extraction
  inputs, and the audit over-invalidates consumer-only changes by design. The change does not alter
  extraction or pack encoding. With the owner's approval (2026-10-06), `analyzerSchemaID` moves to
  `corvint-analyzer/104` and the audit digest is repinned. Existing opt-in analyzer packs are
  rebuilt once on the next index. The lane's later commits change `taskcontext.go` again and
  leave the pin red on the branch; the batch integration repins once at `corvint-analyzer/105`.
- The five-pull-request replay (the survey's `ctxeval.py`, the main binary against this
  branch's, each on its own `--root` over the survey worktrees) and the eight public cases rerun
  with the gate and again with the Go pair rule: the numbers above.
- `TestPairRelationGoCounterpartsShareTheDirectory` and
  `TestTaskContextCounterpartElsewhereIsAMediumPairRow` (new), `TestCreditMirroredMatchesFullScan`,
  `TestTaskContextLinksTestsByEachSignal`, `TestFrameRelationSignals`,
  `TestTaskContextLexicalPairPromotion`, `TestTaskContextReportsNamedPathPairSlotOmissions` and
  `TestTaskContextAdmitsCoChangedPathsAndIgnoresBulkCommits` (fixtures re-pinned for the Go pair
  rule) pass with the package.

## Review

Independent review (Codex, read-only, on the first commit of this change) returned six findings:

1. P1, `TestAnalyzerSchemaInputs` red: resolved by the owner-approved bump to
   `corvint-analyzer/104` (Checks above).
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
16. P2 (the rc.3 batch integration review, Codex, read-only), raised as a hypothesis: the
    comparison row counted the head as the first `lexicalHead` rows still of kind `lexical` in
    the final packet, so when TCP-V0-004 promoted a head test to `pair` one fewer lexical row
    remained, the count swallowed the code row past the head, and the share line for the
    documentation it kept out was omitted. Confirmed by a test that failed on the committed
    code before the fix: `TestTaskContextShareLineSurvivesPairPromotion` (limit 12, the test
    slot's `pkg/x_test.go` first, a four-row head of `pkg/x.go`, `pkg/a.go`, the unrelated
    `pkg/zz_test.go` and `pkg/a_test.go`, six of seven documentation hits, `pkg/d.go` past
    the head; the packet carried `pair pkg/a_test.go` ahead of `pkg/zz_test.go` with the limit
    line alone). Fixed: `recordLexicalTail` reads the code rows past the head by path once
    the reorder and the reservations have placed them and before the graph slot or the pair
    promotion can change a carried row's kind, and `lexicalCoverage` takes the weakest of them
    the packet still carries (TCP-V0-061 amended). The reading point is the one the final
    packet gave before, so every earlier packet and the goldens are unchanged.
17. Recall regression (the rc.3 batch integration's five-pull-request replay, read-only): the
    half share carried 15 of 59 gold rows against main's 18. Fixed by the share's gate (the
    section above); TCP-V0-013, TCP-V0-059 and TCP-V0-061 amended, goldens unchanged.
18. Cross-directory Go `pair` rows (the same replay): a Go test of another package carried as a
    high `pair` row by stem alone. Fixed by the Go pair rule (TCP-V0-004 amended; the section
    above): zero such rows over the five packets, gold counts and goldens unchanged.
19. P3 (the seventh review, Codex, read-only, on the gate commit and the Go pair change): the
    spec said the gate keeps every gain of the half share, but f451a34f swaps the half share's
    `agent-harness-integration-v0.md` for two code rows. The claim now reads as the count: no
    rc.2 case carries fewer critical rows than under the half share, 29 against 27 in total.
    No other finding.

The sixth review (Codex, read-only, on the fifth fixed commit) approved with no findings: the
carried code rows stay a descending-bm25 subsequence with the reorder off, so the comparison row
past the head equals the earlier minimum whenever one exists (a 50,000-case accounting model
found no difference across held paths, reservations, truncation, withholding and promotion), and
the eight public cases rerun on that commit are byte-identical to the fourth rerun.

## Rollback

Revert this change: the old TCP-V0-013 order (five code, two documentation, remaining code,
remaining documentation), the flat 300 score, no `coverage.uncertainty`, and the two goldens.
Reverting the gate alone (the lead and `contextDocumentationQuota` in `lexicalRows`) restores the
half share and its replay loss. Reverting the Go pair rule alone (the Go branch of
`pairRelation`, the `creditMirrored` guard and the re-pinned fixtures) restores the cross-tree
Go pair rows.
