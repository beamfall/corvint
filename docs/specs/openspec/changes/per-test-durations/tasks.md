# Tasks: per-test-durations

Build in this order. Each task names its requirement IDs and the proof that closes it.

- [x] 1 Create `docs/specs/per-test-durations-v1.md` with PTD-V1-001 to PTD-V1-009 from the spec
   delta as proposed, and regenerate `docs/specs/REQUIREMENTS.tsv`. Requirements: PTD-V1-001 to
   PTD-V1-009. Proof: the spec toolchain check passes.
   Done: V1-5449 filed; PTD-V1 reserved; `docs/specs/per-test-durations-v1.md` created and accepted
   under G14 (acceptance-log row `per-test-durations`).
- [x] 2 Add the add-only receipt fields and the `NOT_MEASURED` and `duration-invalid` rules.
   Requirements: PTD-V1-001, PTD-V1-003, PTD-V1-007. Proof: schema round-trip tests; an older reader
   test that ignores the fields.
- [ ] 3 Read durations in the Go, Vitest, Jest, Playwright, pytest and RSpec adapters.
   Requirements: PTD-V1-002, PTD-V1-006. Proof: one report fixture per runner, including a retry.
   Open (V1-5449): the Go adapter of `corvint test` records each package's `Elapsed` in the receipt,
   and the `internal/testrunner` adapters read the Vitest, Jest, Playwright (attempt sum and each
   attempt), pytest and RSpec report times, with a fixture each. A `corvint test` receipt still
   records NOT_MEASURED for the other runners: the Vitest and Playwright provider receipt names no
   per-test time, and Jest, pytest and RSpec run through the declared-command adapter, which reads
   no report. Those receipt adapters are not yet built (changes `jest-provider`, `pytest-provider`
   and `ruby-providers`), so this task stays unticked.
- [ ] 4 Add the time field to the Minitest reporter and read it.
   Requirements: PTD-V1-002. Proof: Minitest fixture with measured durations.
   Open (V1-5449): Corvint's Minitest reporter records `DurationMs` from `Minitest::Result#time` and
   the reader takes it as `minitest-reporter`, with a fixture. A `corvint test` receipt runs
   Minitest through the declared-command adapter, which reads no report, so its Minitest outcomes
   stay NOT_MEASURED until the `ruby-providers` adapter exists; this task stays unticked.
- [x] 5 Mark file-granularity durations and keep invocation time separate.
   Requirements: PTD-V1-004, PTD-V1-005. Proof: a file-level fixture; an invocation time test.
- [x] 6 Copy durations into run evidence rows and wire the consumers.
   Requirements: PTD-V1-008, PTD-V1-009. Proof: run evidence fixture; cost figure fixture with one
   unmeasured test labelled incomplete.
   Done: run evidence rows copy the receipt's duration members; `internal/testduration` `Cost` and
   `Median` are the measured-only rule the cost figure (NMS-V1-002) and the fault-injection harness
   (FIH-V1-012) call. Neither consumer is on this tree; each adopts the rule in its own change.
- [x] 7 Record the trace and the delivery state in the spec.
   Requirements: all. Proof: the trace check maps every PTD-V1 ID to a test.
