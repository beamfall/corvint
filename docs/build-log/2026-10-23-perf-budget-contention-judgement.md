# Read-budget contention judgement (GitHub #1602, #1406)

Date: 2026-10-23. Lane: opencode/perf-budget-gh1602. Follows
`2026-10-23-profile-knobs-review-repair.md`. Spec:
`docs/specs/performance-profiling-and-read-budget-v0.md`, `PRB-V0-038..043`.

## Decision

PR #1608 already delivered the generated fixture (10,000 inputs, 96,350 records, three additional
repositories, a Ruby tree), the six scenarios with p50, p95 and peak resident set, the budgets
file with its baseline and host-contention record, and `make perf-budget`. The remaining gap in
#1602 was flakiness: a contended host still judged the ceilings. This slice adds a run verdict:

- `PASS` or `FAIL` only when the run's own `PRB-V0-028` record shows the host was not contended.
  The threshold is that record's `contended` (1-minute load before or after reaching the core
  count). A run whose load was not observed has no record and is `NOT_JUDGED`, not passed.
- `NOT_JUDGED` keeps every measurement and marks each enforced limit with an observed value
  `not-judged` with the reason; pending targets still report. A failed or unmeasured scenario, or
  an enforced metric that was not observed, still fails, because neither is a timing effect
  (independent review P2, repaired before commit).
- Exit 0 on `NOT_JUDGED` with a stderr line, or 3 under `--require-judgement`
  (`PERF_BUDGET_REQUIRE_JUDGEMENT=1` through make). This keeps one CI step from flaking while still
  letting a dedicated runner fail closed.
- `--record-baseline` refuses an unjudged run, so a contended host never sets the ceilings.

Rejected: a configurable threshold (a second definition of "contended" beside `PRB-V0-028`), and
waiting for a quiet host (a poll inside a gate, unbounded on a shared host).

## Evidence

- Focused tests: `go test -count=1 ./internal/perfbudget/...` passed, including
  `TestPRBV0039ContentionWithholdsJudgement`, `TestPRBV0024HarnessSmokeRun` (now judged under an
  injected quiet load, then rerun under an injected contended load and `NOT_JUDGED`),
  `TestPRBV0041VerdictExitStatus`, `TestPRBV0042MakeTargetRequireJudgement` and
  `TestPRBV0043NotJudgedRunIsNotBaselined`.
- Full scale, `make perf-budget` on this branch (Darwin arm64, 12 cores): 204 s, `PASS`, load
  7.81 before and 7.20 after. Six scenarios (p50 / p95 ms, peak RSS): cold CLI read
  5976 / 6332, 961 MiB; warm CLI read 6176 / 6409, 999 MiB; MCP first call 6653 / 8061,
  1084 MiB; MCP warm call 187 / 634, 976 MiB; full delta 6792 / 7046, 1217 MiB; `affected --path`
  10198 / 11573, 5479 MiB. Every #1598 and #1601 target was `pending-exceeded`.
- Smoke scale through `make perf-budget` with a smoke copy of the budgets: `PASS` at load 10.65 on
  12 cores.

## Limits

- The committed baseline (load 137, contended) predates `PRB-V0-043`; it was not re-recorded, so
  the ceilings still derive from it.
- The CI job is not wired (`.github/` needs owner approval); the proposed step is in the spec's
  Non-goals. Whether GitHub-hosted runners stay below the threshold is `NOT_OBSERVED`.
- `affected-path` covers the Ruby tree only; no TypeScript tree was added.
