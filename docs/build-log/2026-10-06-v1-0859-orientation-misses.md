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
baseline did not; the merged order carries stronger documentation in their place. The six residual misses are five result-limit drops unchanged by any ordering rule
(`flag_impl.go`, `flag_bool_with_inverse.go`, `README.md` of f451a34f, the three retrieval-bench
files, `proof-carrying-context-optimization-v0.md`), `docs/DOGFOOD.md` (the 22nd row of the same
packet at limit 24: two admitted `pair` rows and the limit keep it out at 20), and `ROADMAP.md` (22 stronger documentation rows, share of 10). Each
after-packet states its omission in `coverage.uncertainty`; for 57f06a6a the second line names
133 documentation rows above a carried code row omitted by the share.

Packets in the small-fixture goldens: `internal/contextindex/testdata/context-recipe-default-golden.json`
keeps its twelve rows (`docs/guide.md` moves from row 6 to row 10 by strength; scores 313 to 599);
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
  `TestTaskContextLexicalScoreCarriesStrength` (new), `TestTaskContextPlacesDocumentationAfterFiveCodeRows`
  (rewritten for the merged order), the recipe and core-freeze goldens, and the context tests of
  `cmd/corvint` pass.
- `TestAnalyzerSchemaInputs` (IDX-SNAP-V0-017) fails in this change: `taskcontext.go` is one of
  the pinned extraction inputs, and the audit over-invalidates consumer-only changes by design.
  The change does not alter extraction or pack encoding. Bumping `analyzerSchemaID` to
  `corvint-analyzer/104` and repinning the audit digest is left to the owner; until then the
  `internal/contextindex` package test is red on that audit alone.

## Rollback

Revert this change: the old TCP-V0-013 order (five code, two documentation, remaining code,
remaining documentation), the flat 300 score, no `coverage.uncertainty`, and the two goldens.
