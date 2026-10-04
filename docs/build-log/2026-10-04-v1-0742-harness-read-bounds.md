## 2026-10-04 V1-0742: bounded harness reads and honest token observations

Human-owned intent: the owner asked to start V1-0742. The ticket covers three defects:

- The agent_cost readers parse an operator-supplied observations file without a bound and copy invalid rows into results.
- The trial dispatchers allocate a codex reply whole, drop the stdout overflow signal, merge an empty reply into `ABSENT` and accept negative token counts.
- dogfood-workers counts any echoed `usage` object as Codex telemetry.

### Change

- **`benchmarks/daily-loop-v1/harness.py` reader.**
  - `read_observations` reads at most 1 MiB and 10,000 rows.
  - It reports `observations-unparseable` (prose, non-UTF-8 or nesting past the parser),
    `observation-not-object` and `observations-over-bound` instead of raising.
  - An invalid file records only its class names. Well-formed rows are projected onto their four
    cost fields, so no operator-supplied extra field reaches the result.
  - Amended in `PCCO-V0-017`.
- **`tools/cw-trial` and `tools/cem-trial`.**
  - The codex `-o` reply is read through a prefix reader capped at one byte past the 1 MiB reply bound.
  - `runCommand` now returns the supervisor's `StdoutOverflow`. A cut stream records
    `stdout_truncated: true`. A cut codex event stream makes `tokens` and `tool_calls`
    `NOT_OBSERVED`, and a cut script reply is `reply_truncated`.
  - A whitespace-only reply is the new `EMPTY` state. It still scores as an abstention (CWT-V0-002)
    or as cited nothing (CRT-V0-006), so the metric definitions are unchanged.
  - A usage event with any negative, non-finite or non-numeric count is `NOT_OBSERVED`, and
    cw-trial's token totals refuse it.
  - Amended in CWT-V0-001 and CWT-V0-004, and in CRT-V0-006 and CRT-V0-011.
- **`benchmarks/dogfood-workers`.** Codex usage is read only from `token_count` and `turn.completed`
  records, matched on the record type or its `msg` wrapper's type. A fixture echoes a token-count
  usage object inside an `item.completed` record and proves it is not counted.

### Exclusions

- `benchmarks/daily-loop-v0/harness.py` (digest `4a27e8ef…`) and
  `benchmarks/untouched-repository-v1/harness.py` (digest `b67cc95a…`) are unchanged.
- Both digests are sealed in committed preregistrations. For untouched-repository-v1 these include
  the held-out cobra preregistration that V1-0019 has not yet run, and a private beamfall/core
  preregistration that cannot be resealed from this repository.
- Their contracts make any harness change a new numbered preregistration. Editing them here would
  silently invalidate the seals.
- Acceptance criterion 1 is therefore met only for daily-loop-v1. It stays open for these two
  harnesses until the owner decides to apply it at their next numbered preregistration. V1-0742
  stays open for that step, and V1-0369 still owns daily-loop-v0's savings semantics.

### Evidence and limits

- The new Go tests are in `tools/cw-trial/bounds_unix_test.go`,
  `tools/cem-trial/bounds_unix_test.go` and `benchmarks/dogfood-workers/main_test.go`. The fake
  `codex` in these tests writes a 1 MiB + 4 KiB reply and 8 MiB + 1 KiB of stdout.
- The daily-loop-v1 reader has no maintained test: decision 0088 retired the repository's Python
  test drivers. An ad hoc check covered prose, a non-object row, an over-byte file, an over-row
  file, invalid UTF-8, deep nesting, negative tokens, and a valid file whose extra field was
  dropped. It is not retained.
- Reused cw-trial records that held an empty reply now rescore with `block_state: EMPTY` instead
  of `ABSENT`. Their metrics are unchanged.
