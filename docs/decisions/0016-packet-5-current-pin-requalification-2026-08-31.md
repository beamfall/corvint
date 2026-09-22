# Decision 0016 — Packet 5 current-pin requalification (2026-08-31)

Owner: Russell Lewis, decided in-session 2026-08-31. Status: accepted. Authority:
`GPK-V0-016`, `GPK-V0-017`, and `GPK-V0-026`.

## Owner amendment

> I authorize Packet 5’s six-row current-pin requalification now, superseding Decision 0011’s
> deferral for Packet 5 only. Packet 4 remains NOT_RUN/incomplete, and no Packet 4 promotion is
> authorized.

This is a narrow amendment to decision 0011 §1. It does not reopen Packet 4 and does not amend its
parity, corpus, comparison, or performance status.

## Evidence basis

The superseded 2026-08-29 Packet 5 results had four valid and two invalid rows. They do not qualify
the current Corvint and Beamfall pins and are not reused as current-pin evidence. The unexecuted
combined Packet 4/5 draft also supplies no result. This decision therefore authorizes a prospective
current-pin requalification, not reinterpretation or promotion of either earlier artifact.

## Requirements

- `P5R-V0-001`: Packet 5 MUST use one prospective, source-closed `corvint-perf-v0/2`
  preregistration containing exactly the six selected Corvint and Beamfall harness rows at Corvint
  revision `0794de05b90bb5d74533568190f847e8e0f1db29` and Beamfall revision
  `1b039e75f9d35348d8ed577e1cb37e4f317ce223`, with five discarded warmups, 100 measured samples
  per runtime and row, alternating interleave, the 30-second preregistered per-sample timeout, and no
  runtime, sample-count, candidate-revision, task, corpus, comparison, timeout, or divergence
  override.
- `P5R-V0-002`: A qualifying `corvint-perf-report-v0/2` receipt MUST retain the exact raw-manifest
  hash, scope, pins, trees, corpus digests, comparison profile, sample counts, criteria, and process
  validity evidence. Incomplete or invalid evidence makes the slice `NOT_RUN`; only complete valid
  evidence may produce `PASS` or `FAIL`.
- `P5R-V0-003`: Packet 4 completion and promotion, combined Packet 4/5 completion, global cutover,
  and global compatibility MUST remain excluded from the Packet 5 criteria, and global outcome
  MUST remain `insufficient_evidence`. Newly executed unscoped `/1` evidence MUST fail closed while
  preserving the historical `/1` serialized report shape.
- `P5R-V0-004`: The first authorized run and its report MUST remain immutable evidence. An observed
  invalidity MUST NOT be retroactively granted, absorbed, or promoted; a retry requires new owner
  ratification, a harness-level discriminator, a source-closed exact grant, a new prospective
  preregistration, and a new Corvint pin.

## Derived implementation constraints

Packet 5 is a separate harness-only slice. Its dedicated preregistration MUST contain exactly the
six already-selected Corvint and Beamfall harness rows: paired `user-prompt`, `file-change`, and
compact `session-start`. The current pins, argv, stdin, workspace modes, thresholds, validity
predicates, five discarded warmups, and 100 measured samples are closed before any sample process
starts. No sample-count, candidate-revision, task-selection, corpus, comparison, timeout, or
accepted-divergence override is admissible evidence.

Under `GPK-V0-026`, any result applies only to these six harness surfaces. It MUST NOT be represented
as Packet 4 completion or promotion, combined Packet 4/5 completion, global cutover evidence, or
global compatibility evidence. Any unequal output, timeout, hidden cache difference, repository
change, process failure, or incomplete sample set invalidates the affected Packet 5 row under
`GPK-V0-016`; it does not authorize a new divergence.

These are derived constraints, not additional owner words: the dedicated manifest uses scoped
`corvint-perf-v0/2` and emits `corvint-perf-report-v0/2`; the report binds the exact raw-manifest
SHA-256 and retains
the closed task IDs and excluded claims. The global outcome is always `insufficient_evidence`.
Only `slicePerformanceStatus: PASS|FAIL|NOT_RUN` evaluates the six included harness rows. Packet 4
exclusions are recorded outside that performance criterion set. Every `/2` override is refused
before corpus-pin validation can spawn Git, and a newly executed unscoped `/1` run cannot qualify.
Duplicate JSON keys are rejected recursively before validation. A performance `PASS` or `FAIL`
requires the complete retained pins, trees, digests, exact sample counts, comparison profile,
criteria, and zero process failures or divergence relaxations; otherwise it is `NOT_RUN`.

## Executed evidence and standing

The single authorized run used exactly:

```sh
GOTOOLCHAIN=local go run ./conformance/perf-v0 run --manifest conformance/perf-v0/packet-5-manifest.json --out conformance/perf-v0/results/packet-5-current-pin-requalification-2026-08-31-formal
```

The authenticated `corvint-perf-report-v0/2` report started at `2026-08-31T15:27:57Z` and finished
at `2026-08-31T16:04:51Z`. It is retained at
`conformance/perf-v0/results/packet-5-current-pin-requalification-2026-08-31-formal/report.json`
with SHA-256 `fc599c1d2fa2126812d2ae2d95e6f413325d4e885e4f90b1ab977adee7f41907` and exact
manifest SHA-256 `3705ff2ced308c350b94c4a25457378d107aec22532d27fe066ccde7a5a3dfe1`.
It binds Corvint/candidate/oracle revision `0794de05b90bb5d74533568190f847e8e0f1db29`
(tree `09891579e88d9bd94a19e0690e8f3a177daa28f3`) and Beamfall revision
`1b039e75f9d35348d8ed577e1cb37e4f317ce223` (tree
`b6de7ab13777d7db02d1c147c9b8ecb024e7efcf`). Both corpora are `MEASURED`; all six rows retain
five discarded warmups and 100 oracle plus 100 candidate measured samples.

Five rows are valid. The exact invalid row/reason is
`beamfall-harness-file-change: unequal-stdout`, so its two criteria are `NOT_RUN`. The ten criteria
total five `PASS`, three `FAIL`, and two `NOT_RUN`; global `outcome` is `insufficient_evidence` and
`slicePerformanceStatus` is `NOT_RUN`. This is valid evidence of an incomplete Packet 5 result,
not a promotion. No rerun occurred. Packet 4 remains `NOT_RUN`/incomplete and unpromoted.

The unequal output is the already-adjudicated `DR-0009` / `GPK-V0-040` coverage-denomination
mechanism: both runtimes admitted the same 205 results and included the same five, while the Python
oracle reported coverage `10/5` and Go reported `205/200`. No exact harness-level grant covered
this invocation. Therefore the exact predicate controls: no grant is added after observation, no
divergence is retroactively absorbed, and neither Packet 5 nor Packet 4 is promoted.

A future retry requires all of: new owner ratification, a harness-level discriminator, a
source-closed exact grant, a new prospective preregistration, and a new Corvint pin. This decision
does not supply any of them.

## Rollback and abstention

Before measurement, rollback is deletion of the unmeasured preregistration change. After an invalid
or interrupted run, retain the typed evidence, keep Packet 5 `insufficient_evidence`, and author a
new prospective preregistration before any retry. Packet 4 remains `NOT_RUN` in every outcome.
