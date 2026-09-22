# Outcome Calibration V0

Owner: Russell Lewis
Frozen: 2026-09-11
Intent status: proposed
Delivery status: experimental
Requirement prefix: `OCL-V0`
Authoritative inputs: AGENTS.md invariants 2, 4 and 5;
`docs/specs/learned-trace-admission-v0.md`; `docs/specs/lexical-relevance-floor-v0.md`;
`internal/trace/record.go` (schema version 1); `docs/agent-memory/ideas.md`
(2026-09-01 "abstention needs an answerability signal").

## Agent digest
- Claim: `corvint calibrate` reports, read-only, how recorded outcomes line up with the packet stance before them, and proposes a threshold it never applies.
- Status: proposed/experimental.
- Exists: `internal/outcomecal`, `cmd/corvint/calibrate.go`, and the requirement tests below. The join, the sample bounds, the 2x2, the decile table, the minimum-sample abstention, and the proposal arithmetic are delivered and exercised.
- Blocked on: the schema-version-1 trace record carries **no packet stance and no relevance-floor score**, so every live row classifies `unknown` and `score_coverage` is `0` on real data. Two producer fields are required before the report has signal (see "Missing record fields"). The answerability signal the proposal would calibrate does not exist either.
- Read next: `OCL-V0-003`, then "Missing record fields", then `OCL-V0-006`.

## User and measurable job

A maintainer who has recorded local task outcomes with `corvint record` cannot currently tell
whether the packets that preceded those tasks abstained when they should have. The evidence is
split: the packet decides, the outcome is recorded later, and nothing joins them.

`corvint calibrate` is the read-only join. It is useful when it answers, on one local sample,
how often an answered packet preceded a passing task and an abstaining packet preceded a failing
one, and where on the score scale that relationship turns. It is killed if the join can never be
populated, or if any consumer treats its proposal as an applied configuration change.

This contract does not claim a calibrated abstention rule exists. It reports what the local sample
shows and abstains when the sample is too small.

## Missing record fields

`trace.Record` (schema version 1) holds `SchemaVersion`, `Revision`, `TraceID`, `Task`,
`OpenedPaths`, `ChangedPaths`, `Verification` and `Outcome`. It carries neither of:

- **`packet_stance`** — one of `answered` or `abstained`, the decision the packet that preceded the
  recorded task actually took. Producer: `corvint record` (`cmd/corvint/record.go`), taken from
  the packet the agent consumed. Reader: `outcomecal.Observe`.
- **`relevance_score`** — the bounded numeric support the packet's stance used, on `[0, 1]`.
  Producer: the same `record` path, from the query packet's own relevance-floor statistic. Reader:
  `outcomecal.Observe`, which buckets it.

Both are trace-schema changes and are deliberately **not** made here: this slice ships everything
that works against the existing records and counts what it cannot know. `outcomecal.Observe` is the
single seam that gains them. Until it does, the report's honest output is an all-`unknown` sample
with `score_coverage: 0`, which is the correct statement of what the local store knows.

A populated `relevance_score` would still only calibrate lexical support. The recorded probe in
`docs/agent-memory/ideas.md` (2026-09-01) shows lexical overlap does not separate answerable from
unanswerable tasks; the proposal this command computes is therefore a measurement of the shipped
statistic, never evidence that the statistic is the right one.

## Requirements

- **`OCL-V0-001`.** `corvint [--root PATH] calibrate [--since REV | --window N]
  [--format json|table]` MUST accept exactly those flags and refuse anything else with the
  `invalid-arguments` shape and exit code 2. `--since` and `--window` are mutually exclusive.
  `--window N` is bounded by `1..trace.MaxTraces` and selects the most recent `N` readable records.
  `--since REV` selects only records already pinned to `REV`; because the local store pins one
  revision, any other revision selects nothing rather than inventing history. Default format is
  `json`.
- **`OCL-V0-002`.** The verb MUST be read-only (AGENTS.md invariant 4): it reads the committed index
  and the pinned trace store through `tracerecordrepo.Read` and writes no index, trace, cache, or
  ledger byte on the success path. Its payload declares `mutates: false`. Clarifying amendment
  (2026-09-13, bug hunt): the index load runs under the process signal context, so SIGINT or SIGTERM
  before the report compiles is that same loader failure (the loader's cancellation envelope, exit
  2), never an ignored signal followed by an exit-0 report.
- **`OCL-V0-003`.** Every readable record MUST be joined to a stance. A record that carries no
  stance field MUST be classified `unknown` and counted explicitly with a stated reason; it MUST
  NOT be folded into `answered` or `abstained` and MUST NOT be inferred from opened paths, changed
  paths, or the outcome (AGENTS.md invariant 2). The recorded outcome `passed` is success;
  `failed` and `blocked` are failure. The record has no `reverted` outcome to read.
- **`OCL-V0-004`.** The report MUST carry the 2x2 of `answered`/`abstained` against
  success/failure, the `unknown` row, and abstention accuracy as the exact integer pair
  `calibrated/classified`, where calibrated is `answered_success + abstained_failure`. With zero
  classified rows the accuracy MUST be `null`, never `0`.
- **`OCL-V0-005`.** Where records carry a relevance score, the report MUST bucket them into the ten
  fixed deciles of `[0, 1]`, each bucket carrying its record and success counts in fixed ascending
  order including empty buckets, plus a `score_coverage` count of scored records. Scores outside
  `[0, 1]` clamp to the end buckets before any integer conversion, so the bucket is identical on
  every architecture; a NaN score is not a score and counts in no bucket, `score_coverage`, or
  proposal.
- **`OCL-V0-006`.** When a proposal is emitted it MUST report the decile edge that would have
  maximised abstention accuracy on the scored sample under the rule "abstain when score <
  threshold", with ties resolved to the lowest threshold, and MUST be labelled `applied: false`
  with its sample size and the fixed sentence that it is a proposal requiring the learned-trace
  admission evaluation gate (`LTA-V0-001..LTA-V0-003`) and is never applied by this command. The
  command MUST NOT read, write, or alter any shipped threshold (AGENTS.md invariant 5).
- **`OCL-V0-007`.** A joined sample smaller than 20 rows MUST set `insufficient_sample: true` and
  emit no proposal. A sufficient sample with fewer than 20 scored rows MUST emit no proposal either
  and MUST state `proposal_state: no-scored-records`.
- **`OCL-V0-008`.** Output MUST be deterministic for a fixed store and invocation in both formats:
  canonical JSON (integers and integer pairs only, matching the canonical wire encoding, which
  carries no floats) on stdout with exit 0, or the fixed-order text table. Any refusal goes to
  stderr in the shared error shape with exit code 2.

## Non-goals and authority

- No automatic threshold change. This command proposes; the learned-trace admission gate decides.
  Nothing in the local product reads this report as configuration.
- No learning. No trace is admitted, promoted, weighted, or retained as a result of running it.
- No shared or remote data. The sample is the one local pinned trace store; nothing leaves the
  machine, no network call is made, and no cross-repository aggregate is formed.
- No trace-schema change in this slice. The two missing fields are named above, not added.
- No claim that lexical relevance is the right abstention signal; that remains an open probe.

## Failure modes

- **Unreadable trace state.** A drift, history, repository, or trace-state failure refuses with the
  existing `unsupported-query-*` code rather than reporting a partial sample.
- **Silent unknown collapse.** If a future producer writes the stance for some records only, the
  mixed sample still separates `unknown` from classified rows; accuracy is computed over classified
  rows alone and `unknown.total` stays visible. The risk is a reader quoting accuracy without the
  denominator, which is why accuracy is an explicit integer pair.
- **Overfitted proposal.** The candidate grid is eleven decile edges on one local sample; a
  proposal at 20 rows is weak evidence by construction. It is labelled, gated, and never applied.
- **Mistaking `blocked` for abstention.** `blocked` is a recorded task outcome, not a packet
  stance; it counts as failure and never as `abstained`.
- **Cancellation during the load.** SIGINT or SIGTERM before the report compiles: the loader's
  cancellation envelope, exit 2, empty stdout (`OCL-V0-002`).

## Acceptance evidence and traceability

| Requirement | Go test function |
|---|---|
| `OCL-V0-001` | `TestCalibrateInvocationFlags_OCL001` (`cmd/corvint/calibrate_test.go`) |
| `OCL-V0-002` | `TestCalibrateWritesNothing_OCL002`, `TestRunCalibrateHonorsCancellation` (`cmd/corvint/calibrate_test.go`) |
| `OCL-V0-003` | `TestUnknownClassificationIsCountedExplicitly_OCL003` (`internal/outcomecal`), `TestCalibrateJoinsRecordedOutcomesAsUnknown_OCL003` |
| `OCL-V0-004` | `TestMatrixAndAbstentionAccuracy_OCL004` (`internal/outcomecal`) |
| `OCL-V0-005` | `TestScoreDecileBuckets_OCL005`, `TestScoreClampIsArchitectureIndependent_OCL005` (`internal/outcomecal`) |
| `OCL-V0-006` | `TestProposalIsReportedButNeverApplied_OCL006` (`internal/outcomecal`) |
| `OCL-V0-007` | `TestInsufficientSampleSuppressesProposal_OCL007` (`internal/outcomecal`), `TestCalibrateInsufficientLocalSample_OCL007` |
| `OCL-V0-008` | `TestCalibrateOutputIsDeterministic_OCL008` (`cmd/corvint/calibrate_test.go`) |

`OCL-V0-003` and `OCL-V0-005` are covered against synthesized observations for the populated case
and against real recorded traces for the live all-`unknown` case. No dogfood receipt claims a
populated 2x2; none can exist until the producer fields land.

## Rollback

Remove the `calibrate` dispatch line from `cmd/corvint/main.go` and its `help.go` entry, then
delete `cmd/corvint/calibrate.go`, `cmd/corvint/calibrate_test.go`, and `internal/outcomecal`.
Nothing else reads the package, no stored state is produced, and no shipped threshold or ranking
input depends on it, so removal is a pure deletion with no migration.

## Unresolved decisions

- Whether the stance and score belong on the trace record at all, or in a separate packet-decision
  ledger that the record references by packet identity.
- Whether abstention accuracy should weight a wrong abstention and a wrong answer differently; the
  current report weights them equally and exposes the raw cells so a reader can reweight.
