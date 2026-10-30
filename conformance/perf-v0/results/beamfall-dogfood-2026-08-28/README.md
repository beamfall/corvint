# Beamfall dogfood performance evidence — 2026-08-28

First `GPK-V0-022` performance evidence measured on the real Beamfall repository.

- Corpus pin: `fa3b1e7fe5bc6c10e4b09b2729f364780f567a48`, tree
  `3a4251e0f0ebe70a489e296b32189634250f69d4`, `tree-identity=proved`.
- Candidate: `4807af2179873530f060ec97e1356f9cbe29b4f3`.
- 100 measured samples per runtime after 5 warmups, interleaved, both corpora.

## Outcome

`insufficient_evidence` overall, because `--candidate-revision` deliberately forces it: the candidate
was built from a commit other than the corpus revision, so the run is explicitly not preregistered
(`preregistered=false`). The per-criterion verdicts below are still the real measurements.

`corpora-measured=2 tasks=10 valid-tasks=8 invalid-tasks=2 criteria-pass=13 criteria-fail=1 criteria-not-run=4`

## The one failure

```
FAIL GPK-V0-017(b) scope=beamfall-query-authority-start
  required="candidate p95 <= 500ms"
  observed="candidate p95 654.1ms (95% CI 649.1-662.6ms)"
```

The confidence interval lies entirely above the threshold, so this is a robust breach rather than
sampling noise. The same task passes on the `corvint` corpus at 346.6ms (CI 341.4-350.8ms): the breach
is a function of repository size, and Beamfall is the actual dogfood target. Query is the per-prompt
hot path, so this is user-visible latency.

## The four NOT_RUN

All four are the two `impact` tasks' p95 and peak-resident thresholds, invalidated by
`unequal-stdout`. That is DR-0004 — a LANDED `python-defect` that under `GPK-V0-033` is never
repaired — so `impact` cannot earn a performance verdict while the protocol's validity predicate
requires byte-equal stdout. Recorded as an operator decision in `docs/agent-memory/questions.md`.

## Coverage caveat

No task measures `user-prompt`, `file-change`, or compact `session-start` — the three events that
build an index (`internal/gokernel/harness.go` `requiresIndex`). The measured harness numbers are
therefore the cheap non-index path. `cmd/corvint/harness_context.go` builds the FULL index for
`user-prompt`, whereas the `query` CLI uses the narrower `contextindex.BuildQuery`, so Beamfall's
real per-prompt cost is heavier than the 654.1ms above and is currently unmeasured.
