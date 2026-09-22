# 0008 — Re-preregister the perf corpus revision after the AGPL history squash

- Status: accepted
- Date: 2026-08-30
- Clauses: `GPK-V0-016`, `GPK-V0-017`
- Supersedes: the `registeredAt: 2026-08-28` corvint-corpus pin in `conformance/perf-v0/manifest.json`

## Problem

`conformance/perf-v0/manifest.json` pinned the corvint corpus at
`58a2806e4b621ebd13a439ff661b8d0d875133b8`. The AGPL relicense squashed this repository's history
(the root commit is now `33d926fe61710cc0cfb92b89232f932407a593a0`), which left that pin **not an
ancestor of `HEAD`**. The commit object survives only because unmerged pre-squash branches still
reference it, so nothing failed loudly.

The pin is load-bearing three times over in `conformance/perf-v0/runner.go`: it selects the oracle
source (`materializeOracleSource`, :197), the candidate build (`candidateRevision :=
corvintCorpus.Revision`, :201), and the corvint corpus tree (`materializeCorpus`, :274). So every
`corvint-perf-v0` run since the squash built its **candidate** from orphaned pre-relicense code.

Consequence: the `GPK-V0-017(b)` Beamfall query result recorded on 2026-08-29
(`beamfall-query-authority-start`, 737.1ms, 95% CI 727.0-752.7, FAIL) measures a candidate that
predates both the relicense and the query-path optimization. That number could never move, because
no amount of work on current history was reachable from the pinned revision.

This was independently confirmed by measurement, not left as inference. A diagnosis of the
`beamfall-impact-path` divergence rebuilt the candidate at `58a2806e` and found it emits
`8a1d6ecc...`, byte-equal to the `agreedStdoutSha256` the 2026-08-29 report records (which
`runner.go:376` sets from **candidate** stdout). The runner demonstrably did build from the orphaned
revision. The same diagnosis showed that revision predates DR-0009's Go repair, so the 2026-08-29
`impact` result reflects a candidate two repairs behind current history.

## Decision

Re-pin the corvint corpus to `7964b530b0702695cc78fdb23855d618a79566f3` (current `HEAD`). Leave the
Beamfall corpus pinned at `fa3b1e7fe5bc6c10e4b09b2729f364780f567a48`, which **is** an ancestor of
Beamfall's `HEAD` and is therefore valid.

Holding Beamfall fixed is deliberate, not neglect. The Beamfall corpus is the measurement *subject*;
holding it constant while the candidate advances is what makes the next run comparable to the
2026-08-29 run on the slice we actually care about. Re-pinning both would confound a candidate
change with a corpus change and answer nothing.

## What this does and does not move

**The oracle does not move.** `src/` is byte-identical across the squash — tree
`240044265f2c0e1be38af599ca8455b7b761ddd3` at both the old pin and `HEAD`. `GPK-V0-002`'s frozen
oracle authority is therefore untouched by this repin, and no expected bytes can shift because of
it. This was verified before the edit, not assumed.

**The candidate moves, which is the point.** 306 files, +51950/-2658 across `cmd/corvint` and
`internal/**`.

**The corvint corpus tree moves**, because the corvint corpus is this repository: 1204 to 1773 tracked
files. Corvint-slice measurements are consequently **not comparable** to the 2026-08-29 run — a 47%
larger corpus is more work, so corvint-slice absolute numbers may worsen even where the candidate got
faster. Any comparison across this boundary on the corvint slice is invalid and must not be drawn.
The Beamfall slice is comparable.

## Standing

The 2026-08-29 results under `conformance/perf-v0/results/corvint-beamfall-2026-08-29/` are retained
as the record of what was measured, and are **superseded, not withdrawn**. They remain the honest
statement of that revision's behaviour. Every `GPK-V0-017` threshold reverts to `NOT_RUN` under the
new preregistration until a run under this manifest produces one; per `GPK-V0-017`'s closing
sentence, a threshold not measured is `NOT_RUN`, never waived and never inherited from the
superseded run.

The pending DR-0013 encoding repair will move the candidate again. That will warrant a further
repin by the same route; a repin is a decision, never something a run performs.

## Gap this exposed

`manifest.go:275` validates only that a pinned revision is a well-formed lowercase SHA-1
(`fullGitSHA1`). Nothing checks the revision is reachable from the repository's current history, so
an orphaned pin passes validation and measures silently. Recorded in `docs/agent-memory/tests.md`;
not fixed here, because that is a guard with its own test and not part of this decision.
