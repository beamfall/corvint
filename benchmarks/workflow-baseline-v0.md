# Workflow context/recovery baseline (AT-01 freeze)

This document is the AT-01 acceptance item "freeze context/recovery baselines and scorer edge cases
independently of compatibility" (`ROADMAP.md` AT-01). It preregisters task classes, arms, metrics,
and scorer edge rules for `start`/`investigate`/`resume`/`change-review` workflow tasks so AT-08
("Evaluate the first complete daily workflow") can run a screening comparison without inventing
scoring rules mid-run. It is independent of `docs/specs/compat-trial-v0.md` (`CTR-V0-006..008`),
which freezes the separate compatibility-detection trial; nothing here duplicates or amends that
document.

Pinned commit: `01aa66ad071756f7308bb04b0ec379b051a231e3`.

## 1. Task classes

AT-08's accept line names four classes: "freeze unseen start/investigate/resume/change-review
tasks" (`ROADMAP.md` AT-08). Each class below gives a one-line definition, sourced from the closest
existing BRAIN-DOG requirement where one exists, and a "solved" criterion. No BRAIN-DOG requirement
states a pass/fail bar for any of these four classes — every "solved" line is `OWNER DECISION`.

- **start**: get from one open question to a cited architecture, ownership, and a safe first
  runnable action, without a broad repository read (`BRAIN-DOG-004`, onboarding). **Solved**:
  `OWNER DECISION` — no frozen threshold for "safe first action" exists yet; a candidate bar is a
  correct first edit/command proposal with no critical citation miss, but this is not adopted policy.
- **investigate**: return the direct references/dependencies, bounded consequence candidates, and
  affected specs/tests/contracts for a stated question, naming unexamined dynamic surfaces rather
  than asserting runtime causality (`BRAIN-DOG-001`, butterfly-effect; `BRAIN-DOG-010`, incident
  assistance, for the failure-triage flavor of this class). **Solved**: `OWNER DECISION` — no
  frozen threshold for "found the right cause/consequence set" exists yet.
- **resume**: reconstruct a prior session's working set — evidence handles, decisions, unknowns,
  changed hunks, outcome — well enough to continue or hand it off, and compile that into a
  content-addressed delta an independent updater can verify (`BRAIN-DOG-017`, working-set receipt;
  `BRAIN-DOG-018`, checkpoint/handoff/merge/revert/close). **Solved**: `OWNER DECISION` — no frozen
  rule states how close a reconstructed working set must be to the original to count as resumed.
- **change-review**: combine exact change/CEM evidence, consequences, tests, intent, ownership,
  conflicts, and unknowns for a diff, without asserting correctness or merge safety (`BRAIN-DOG-006`,
  code review). **Solved**: `OWNER DECISION` — no frozen rule states how many of those evidence
  categories must be present, or how a missed conflict/unknown is weighted, to count as solved.

## 2. Arms

All three arms share one surface, defined separately from each arm's own experimental surface: the
same repository checkout, the same native tools (shell, grep, file reads), the same source access,
the same initial context text, and the same token and time budget. Only the Corvint surface (arm A0's
admitted verbs, arm C's candidate surface) differs between arms; everything in the shared surface
above, plus model and effort, is held equal for a given task. An arm-specific advantage in any of
those shared dimensions invalidates the comparison for that task.

### Arm A — current admitted Corvint

Arm A is split because `batch` (`cmd/corvint/batch.go`) does not exist at the pinned commit and its
spec is proposed/experimental, not admitted.

- **A0 — admitted verbs at the pinned commit**: `query`, `context` (task-context selection and
  expansion, `cmd/corvint/taskcontext.go`), and `impact`. No other Corvint verb, flag, or unreleased
  branch is in scope for A0. Output is whatever those verbs return under their existing
  budget/refusal contract; nothing is hand-tuned for the task. The pinned commit
  `01aa66ad071756f7308bb04b0ec379b051a231e3` remains A0's reproducibility anchor.
- `batch` is not part of arm A. Until its spec is accepted it is scoped under arm C (candidate); if
  and when it is admitted, it re-enters arm A under the then-current pinned commit.

### Arm B — unrestricted native tools plus competent structured notes

Ordinary agent tool access (read, grep/find, run tests, edit) with no Corvint verb, plus a required
notes template the worker must keep current across the task. The template is fixed for this freeze:

```
intent:            <one line, what this task is trying to establish or change>
handles:            <list of {path, blobHash-or-path:line} pairs for every source location relied on>
decisions:          <list of decisions taken so far>
progress:           <one line, current progress>
outcome:            <one line, outcome so far>
nextAction:         <one line, what happens next>
unknowns:           <list of open questions the worker could not resolve>
failedApproaches:   <list of {approach, whyAbandoned}>
commandsRun:        <list of exact argv, in order>
```

`handles` entries identify a precise source location: either `path` plus the blob's exact Git
content hash (`git hash-object <path>`), or a precise locator of `path:line`. A hash is permitted
but not mandatory; a paraphrase or an unbounded line range is not acceptable. A worker that cannot
supply either form for a claimed handle records that handle under `unknowns` instead of guessing
one.

### Arm C — candidate

Whatever AT-05 (task-context selection/expansion), AT-06, and AT-07 (checkpoint/expansion) deliver
by the time AT-08 runs. This arm's exact surface is not frozen here; AT-08 freezes it against
whatever is admitted at that time, under the same equal-budget rule as arms A and B.

## 3. Metrics

Per-worker metrics are exactly the `METRIC_FIELDS` the all-worker receipt adapter emits
(`benchmarks/dogfood_workers.py`): `turns`, `inputTokens`, `cacheCreationTokens`,
`cacheReadTokens`, `outputTokens`, `sourceOpens`, `broadSearches`, `compactions`, `retries`,
`unparsedLines`. Corpus-level totals additionally carry the adapter's `_totals` fields:
`observedWorkers`, `cancelledWorkers`, `failedWorkers`, `solvedWorkers`. Any of these that the
harness cannot observe is the literal string `NOT_OBSERVED`, never zero or an estimate
(`benchmarks/dogfood_workers.py` module docstring; `DOGFOOD-008`).

This freeze adds, on top of the receipt fields:

- **completion**: whether the worker produced an attempt at all (distinct from `solved`).
- **independent correctness label**: a reviewer who did not build the response scores it against
  the task's frozen expected evidence, not against the worker's own claim.
- **critical misses**: count of required evidence items the independent reviewer finds absent from
  the worker's cited handles.
- **p50/p95 wall latency**: nearest-rank, over the per-class sample, per arm.
- **agent-visible response bytes** (`DOGFOOD-008`): bytes of tool/response content actually placed
  in the worker's context, measured separately from token counts.
- **unnecessary output bytes** (`BRAIN-DOG-013`): bytes returned to the worker that the independent
  reviewer finds were not needed to solve the task.
- **warm first-packet latency** and **expansion latency** (`BRAIN-DOG-015`): measured and budgeted
  as two separate figures, not combined into one latency number.
- **broad-search miss classification** (`BRAIN-DOG-016`): for each broad search the worker issues
  that misses required evidence, one of `missed-evidence`, `vocabulary-mismatch`,
  `unsupported-source`, or `caller-choice`.

## 4. Scorer edge rules

Each rule is one sentence, `MUST` wording, in the style of `CTR-V0-006..008`'s preregistered rules
(preregistered before any judged run, not fitted after seeing results):

- **WB-001**: A scorer MUST refuse the run outright if any transcript is double-counted across
  workers.
- **WB-002**: Cancelled and failed workers MUST stay in the denominator for every per-task and
  per-class rate; they MUST NOT be dropped or excluded.
- **WB-003**: If a class or arm has zero solved tasks, every per-solved ratio (cost or otherwise)
  for that class/arm MUST be reported undefined, and the run MUST be labelled screening-only rather
  than a pass/fail result.
- **WB-004**: A token-savings claim MUST be `NOT_OBSERVED` unless all of the following hold: every
  metric total needed for the claim is observed (no `NOT_OBSERVED` total anywhere in the session),
  every worker in the session was observed (complete all-worker coverage, no `NOT_OBSERVED` total
  in the corpus-level `_totals` fields), the comparator arm's correctness is non-inferior
  (`DOGFOOD-009`), and zero critical misses were added relative to the comparator; a powered
  comparison alone, without all four conditions, MUST NOT support the claim. Partial telemetry MAY
  support engineering discussion but MUST NOT support a complete-cost claim.
- **WB-005**: Pricing MUST be recorded alongside token totals and MUST NOT be applied to produce a
  dollar cost claim.
- **WB-006**: Cancelled and failed attempts MUST stay in the all-attempt cost denominator per
  `WB-002`; a paired arm-to-arm comparison on a task MUST be computed only over tasks where both
  arms produced an outcome, and an unpaired task MUST NOT enter a paired comparison. The report
  MUST publish the all-attempt table and the paired table side by side, so that pairing/cancellation
  cannot remove expensive failures from view.
- **WB-007**: Held-out tasks used for this freeze's comparison MUST NOT be used afterward to tune
  Corvint, arm B's notes template, or the candidate arm; a tuned task is development forever
  (`benchmarks/README.md` first-observation rule).
- **WB-008**: All three arms MUST run under equal model, effort, and shared surface (same repository
  checkout, same native tools, same source access, same initial context text, same token and time
  budget) for a given task; an arm-specific advantage in any of those dimensions invalidates the
  comparison for that task.
- **WB-009**: Arm B's structured notes template MUST be kept current across the task and MUST carry
  intent, handles, decisions taken, current progress, outcome so far, next action, unknowns,
  failedApproaches, and commandsRun; a handle hash is permitted but not mandatory, and a precise
  path:line locator is acceptable in place of a blob hash.
- **WB-010**: The number of tasks per class, the number of repeats per task, and the paired
  statistical test are OWNER DECISION and remain open; a comparison produced before the owner sets
  them is screening-only and MUST NOT be reported as a powered superiority or non-inferiority
  result.

## 5. Sample size and power

No power calculation is frozen by this document. The number of tasks per class, the number of
repeats per task, and the paired statistical test (e.g. a paired bootstrap or sign test analogous to
`CTR-V0-006`'s clustered analysis) are all `OWNER DECISION`, still open. Until the owner sets `n` and
the paired test, any comparison produced under this freeze is screening-only: it may surface
engineering signal (an arm is clearly broken, a metric is clearly `NOT_OBSERVED` across the board)
but MUST NOT be reported as a powered superiority or non-inferiority result.

## 6. What this document does not do

- It does not define or restate any compatibility-detection rule; `docs/specs/compat-trial-v0.md`
  (`CTR-V0-006..008`) owns that trial independently, and this freeze is scoped to context/recovery
  workflow tasks only.
- It does not enumerate or freeze the held-out task list itself; that list is AT-08's own freeze,
  built against these classes, arms, metrics, and rules.
- It does not claim any token, cost, or time savings for Corvint or the candidate arm. No run has
  happened yet under this freeze; any such claim before a powered, paired run with `n` and a test
  set (Section 5) would be exactly what `WB-004`/`WB-005` forbid.

## 7. Accounting diagnostic repair (2026-09-08)

AT-01's explicit-log adapter treats repeated Claude `requestId` usage records as one request,
separately within the existing main and sidechain partitions. Equal snapshots count once and
increasing fields replace earlier values. A missing, invalid or decreasing field becomes
`NOT_OBSERVED` for that request and stays unknown after later valid snapshots; other fields and
requests remain independently countable. This is conservative adapter policy, not host qualification
or a newly established provider streaming guarantee. Mixed sidechain usage remains outside main
worker totals; an all-sidechain transcript remains the worker itself.

The synthetic regression in `tests/test_dogfood_workers.py` preserves the original counterexample:
100 input tokens followed by 1 for one request previously produced 1. The diagnostic now retains
uncertainty instead of silently discounting the request. Existing duplicate-path, cancellation,
zero-solved, malformed-field and ambiguous Codex cumulative tests remain relevant. No complete
builder/reviewer host logs were supplied; absent child rosters and terminal reconciliation remain
unqualified. Retries, cost and unobserved telemetry retain `NOT_OBSERVED`; WB-004/005 are unchanged.
Rollback restores the preceding adapter implementation; retained failing inputs remain development
fixtures and cannot qualify a complete-cost result.
