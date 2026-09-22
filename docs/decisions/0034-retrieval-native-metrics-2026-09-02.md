# Decision 0034 — the trial reports the retriever's own margin beside the agent's outcome

Date: 2026-09-02. Status: accepted. Authority: repository owner, verbatim instruction "do 1 then
2" (2026-09-02) on the candidate list that opened with "report the retrieval-native metrics beside
the model outcome", given after the owner's goal "I want corvint to beat grep by a large margin" and
the finding that the model-mediated any-gold metric cannot show one.

## What is decided

1. CWT-V0-013: every scored arm carries `packet_top_1/3/5/10` (a gold path among the context's
   first k rows in the consumer's order, the subject skipped), `f1` of its distinct valid claims,
   `hard_gold_in_context` and `hard_success` (gold beyond the subject's stem, so the counterpart a
   naming convention guesses does not count), and `success_retrievable` (success over the tasks
   with a gold path present at the base). Each is derived from the record alone; `score` reproduces
   it byte for byte.
2. A run records `subject` and `gold_at_base` at dispatch. `score --tasks FILE --history DIR` fills
   them for a report written before the fields existed and never overwrites what a run observed.
3. The five pooled unseen result files are rescored in place with those fields and reproduce under
   a flag-free `score`. The task manifests stay uncommitted (they carry diff text); the result files
   now carry what the metrics need. Amended by decision 0211: three of the five
   (`cw-trial-unseen-beamfall-apple-run-1.json`, `-apple-run-2.json`,
   `cw-trial-unseen-beamfall-run-1.json`) do not reproduce their `grep` `packet_top_1/3/5`; they
   stay as recorded and are non-reproducible.

## What was measured

Pooled unseen runs (163 tasks: v1 four-arms, v2 without the v1 ids, beamfall run 1, apple run 1;
160 with gold at base), `--access none`:

| arm | success | top 1 | top 3 | top 10 | hard gold in context | hard success | mean F1 |
|---|---|---|---|---|---|---|---|
| none | 105 | 0 | 0 | 0 | 0 | 53 | 0.293 |
| grep | 131 | 43 | 70 | 89 | 95 | 96 | 0.410 |
| aider | 106 | 16 | 28 | 51 | 61 | 74 | 0.329 |
| corvint | 131 | 87 | 116 | 133 | 117 | 92 | 0.469 |

The `grep` row's top 1 and top 3 are the recorded, non-reproducible values; rescored they are 49 and
77, so corvint − grep is +38 and +39 (decision 0211).

Paired corvint against grep: top 3 +46 (56 against 10, p < 0.0001), top 10 +44 (51 against 7,
p < 0.0001), hard gold in context +22 (25 against 3, p < 0.0001), hard success -4 (15 against 19,
p = 0.61), success 0 (11 against 11). The no-context arm's 105 is the counterpart guess: its hard
success is 53. The model reproduces the packet's top rows and adds its diff reading to grep's
listing; the two meet at 131. The retriever's margin is large and the agent's outcome on this
task family cannot show it.

## What is not decided

Which of these rates the bet's kill criteria read; a task family or judging rule whose agent
outcome tracks retrieval (hard gold judged alone, or a repository the agent cannot guess from
names); the reference slot that decision 0035 measures against `hard_gold_in_context`.
