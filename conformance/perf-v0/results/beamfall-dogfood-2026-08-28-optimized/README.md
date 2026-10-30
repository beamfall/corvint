# Beamfall dogfood performance — 2026-08-28, after the query-path optimization

Same corpora and protocol as `beamfall-dogfood-2026-08-28`, re-run at candidate
`0d99b2dad21d72f6ddc587933d55a0a0cbbdf88d`. Both corpora pinned identically, both
`tree-identity=proved`, so this is a like-for-like comparison of the same workload.

`corpora-measured=2 tasks=10 valid-tasks=8 criteria-pass=14 criteria-fail=0 criteria-not-run=4`

## The prior failure is closed

```
PASS GPK-V0-017(b) scope=beamfall-query-authority-start
  required="candidate p95 <= 500ms"
  observed="candidate p95 392.4ms (95% CI 366.2-413.7ms)"
```

Previously 654.1ms (CI 649.1-662.6ms), the run's only FAIL. The interval now lies
entirely below the threshold, as it previously lay entirely above it.

| Task | Before | After |
|---|---|---|
| `beamfall-query-authority-start` | 654.1ms FAIL | 392.4ms PASS |
| `query-authority-start` | 346.6ms | 259.4ms |
| `beamfall-harness-session-start` | 63.6ms | 63.9ms |
| `harness-session-start` | 57.1ms | 57.3ms |

The harness tasks are unchanged because they build no index, which is the coverage
caveat below.

## Still NOT_RUN

The same four: both `impact` tasks' p95 and peak-resident thresholds, invalidated by
`unequal-stdout`. That is DR-0004, a LANDED `python-defect` never repaired under
`GPK-V0-033`. Pending operator decision in `docs/agent-memory/questions.md`.

## Coverage caveat, unchanged

No task measures `user-prompt`, `file-change`, or compact `session-start` — the three
index-building events. The hook path was measured ad hoc at ~3350ms before and ~378ms
after this change on Beamfall core, but that is not protocol evidence.

## Outcome line

`insufficient_evidence` overall is forced by `--candidate-revision` (`preregistered=false`),
because the candidate was built from a commit other than the corpus revision. The
per-criterion verdicts above are the real measurements.
