# Decision 0211 — three beamfall cw-trial reports do not reproduce; committed reports must

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

## Finding

The `grep` arm's `packet_top_1/3/5` in `benchmarks/results/cw-trial-unseen-beamfall-apple-run-1.json`,
`-apple-run-2.json`, and `cw-trial-unseen-beamfall-run-1.json` do not reproduce under `score`
(`docs/BUILD-LOG.md`, 2026-09-13). The cause is not tool nondeterminism and not input drift. The
recorded values were scored by something other than the tool committed with them.

- Not nondeterminism. `orderedRows` and `anyGold` are pure functions of the record. The `cw-trial`
  built from the recording commit `a3659e3d` and the one built from `571c1922` rescore all five
  CWT-V0-013 reports to identical bytes.
- Not input drift. A flag-free `score` reads only the report: `context`, `subject`, and `gold`.
  Nothing outside the file can change the result.
- The recorded values are what scoring gives when the subject row is not skipped. That holds in
  every arm-metric cell: 160 of 160, 160 of 160, and 240 of 240. With the subject skipped, as
  `orderedRows` has done since `a3659e3d`, 10, 10, and 5 cells differ. `corvint-v2-run-1` and
  `corvint-v1-four-arms` match both ways, so their reproduction said nothing about the skip.
- The pre-commit scratch analysis skipped the subject. It read grep's pooled listing as
  49 / 77 / 79 / 89 at k = 1 / 3 / 5 / 10 (`docs/agent-memory/ideas-2026-09.md`, 2026-09-02
  entry), and those are the rescored values. Decision 0034 published 43 / 70 / 77 / 89.
- Inferred, not confirmable: an uncommitted intermediate build scored the three files, and nobody
  rescored them before `a3659e3d`. No build identity was recorded, so this cannot be checked.
  Decision 0034 item 3 says the files "reproduce under a flag-free `score`", but no test checked
  that for the committed files.

## The call

1. The three reports stay exactly as recorded. CWT-V0-013 names them as non-reproducing. A citation
   of their `grep` `packet_top_1/3/5` gives the rescored value beside the recorded one.
2. CWT-V0-013 now requires every committed report that carries the metrics to reproduce byte for
   byte under a flag-free `score` of the committed tool.
   `TestCommittedReportsReproduceTheirRetrievalMetricsUnderScore` (`tools/cw-trial/main_test.go`)
   rescores each one. It failed on exactly the three reports before they were listed. It also
   requires each listed report to still differ, so the list stays exact.
3. The pooled figures in decision 0034 are corrected here without editing its table. Pooled set:
   163 tasks, v1 four-arms, v2 without the v1 ids, beamfall run 1, apple run 1. Corvint's values
   are unchanged. Grep's rescored values, with the exact two-sided sign test on discordant tasks:

| k | corvint | grep recorded | grep rescored | corvint − grep | corvint-only / grep-only | p |
|---|---|---|---|---|---|---|
| 1 | 87 | 43 | 49 | +38 (was +44) | 54 / 16 | 5.9e-06 |
| 3 | 116 | 70 | 77 | +39 (was +46) | 49 / 10 | 2.7e-07 |
| 5 | 123 | 77 | 79 | +44 (was +46) | 51 / 7 | 2.4e-09 |
| 10 | 133 | 89 | 89 | +44 | 51 / 7 | 2.4e-09 |

The reading does not change: corvint's margin over grep stays at p < 1e-5 at every k.

## Not decided

No scorer build identity is added to the report schema. The reproduction test catches the
mismatch at commit time instead.

Rollback: revert the commit. That removes the test and the CWT-V0-013 sentence; the reports
are untouched.
