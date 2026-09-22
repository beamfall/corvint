# Historical paired performance protocol — retired

Decision 0088 retires execution of this Python-oracle protocol. `run` and `diagnose` now refuse
with `retired-python-protocol`; the owner cancelled retry-2 before it started. Manifests,
diagnostics and reports below remain immutable historical evidence with their original statuses.
They do not provide a current native-only performance qualification. Implementation history is
available at pre-cutover commit `9ca27f9a`.

The previous protocol documentation follows for interpreting those retained artifacts.

# `conformance/perf-v0` — the GPK-V0-016 performance protocol

This is the measurement harness for `GPK-V0-016` and the threshold evaluator for `GPK-V0-017`. It
lives outside both runtimes: it spawns the Python oracle and the Go candidate as ordinary
processes and times them from the outside, because the cost the product actually ships is a fresh
process, not a warm interpreter.

## What the clauses require, and where each requirement lives

`GPK-V0-016` requires an *interleaved, preregistered* harness on the same machine, revision, task
corpus, cache state and sanitized environment; at least 100 measured fresh-process samples per
runtime after five discarded warmups; reported p50/p95/max and failures; and it names Corvint **and**
Beamfall. Measurements with unequal outputs, timeouts, hidden cache differences, or changed
repositories are invalid.

| Requirement | Where it is enforced |
|---|---|
| Preregistered | [`manifest.json`](manifest.json) is read and validated before any process is spawned, and a run never rewrites it. `validateManifest` refuses a manifest that lowers a sample count, a warmup count, or any `GPK-V0-017` threshold. |
| Interleaved | `measureTask` alternates which runtime leads each sample pair, so neither systematically inherits the other's cache and scheduler warmth, and machine drift lands on both. |
| Fresh process per sample | `runSample` starts the clock immediately before `Start` and stops it immediately after the bounded `Wait`. Process creation and pre-reap group termination are inside the measurement; post-`Wait` quiescence proof and output-drain bookkeeping are outside its recorded duration. Nothing is measured in-process. |
| Same revision and task corpus | `materializeCorpus` requires the **pinned commit** to be reachable from the source repository's `HEAD`, archives it, rebuilds it as a real Git repository, and refuses to continue unless the rebuilt `HEAD^{tree}` equals the pinned revision's tree byte for byte. |
| Same sanitized environment | `sanitizedEnvironment` builds one explicit allowlist for both runtimes. The only difference is `PYTHONPATH`, which points the oracle at its frozen `src/` tree; `TestSanitizedEnvironmentIsIdenticalApartFromPythonPath` pins that. |
| Same cache state | Both runtimes share one `CORVINT_CACHE_DIR`. The five discarded warmups settle it, and its digest is taken before and after each measured window. |
| ≥100 samples after 5 warmups | Preregistered and validated. `--samples` can lower it for a smoke run, but that run is stamped `preregisteredRun: false` and can only ever report `insufficient_evidence`. |
| p50/p95/max and failures | Reported per runtime per task, plus a bootstrap confidence interval on the p95. |
| Corvint **and** Beamfall | `validateCorpora` refuses a manifest that does not declare **both**. A corpus that cannot be measured must be typed `NOT_RUN` with a stated reason; it cannot be silently dropped. |
| Unequal outputs invalidate | Every sample pair is compared on stdout, stderr and exit status. An unequal output is not a slow sample — it means the two runtimes did not do the same work, so no timing comparison between them means anything. One named exception is ratified: see [Accepted divergences](#accepted-divergences). |
| Timeouts invalidate | Each sample carries the preregistered timeout; POSIX samples run in an owned process group, terminate the group before reaping the leader with `Wait`, then prove group quiescence and bound output drains by a one-second shutdown deadline. A timeout is typed rather than recorded as a large duration; unsupported platforms refuse before launch. |
| Hidden cache differences invalidate | The `CORVINT_CACHE_DIR` digest must be identical before and after the measured window. |
| Changed repositories invalidate | The corpus digest — working tree content hashes, Git's own porcelain status, and `HEAD^{tree}` — must be identical before and after the measured window. The oracle's frozen `src/` tree is digested the same way across the whole run. |

## Extension row classes

Decision 0059 declares the `adapter-overhead` plugin row class as adapter wall time minus kernel
wall time for the same golden event. Its manifest bound is fixed at 50 ms p95 and 100 ms max; a
row must also declare zero extra Corvint processes per event. A missing or raised bound is refused
before any process is spawned. The class declaration is not a measurement result. No
adapter-overhead task or result is added by this slice, so every published host remains `FALLBACK`
until a later passing row supplies the required evidence.

## Snapshot-miss vs. snapshot-present corpora

Beamfall is measured twice, as two separate preregistered corpora over the same pinned repository
and revision: `beamfall` runs no setup, so every measured invocation compiles its bounded context
index in-process — the snapshot-miss path, reported beside the binding figures since decision 0084
(2026-09-11) named `beamfall-snapshot-present` binding for GPK-V0-017(a) index-building events and
(b); decision 0037 item 1 had kept the cold corpus binding until a refresh shipped and was dogfooded. Its
`indexNote` no longer says "no persisted index artifact exists"; that was true only before decision
0032. `beamfall-snapshot-present` declares one `"setup": [["index"]]` entry, which `measureCorpus`
runs once against the materialized corpus with the **candidate only** — never the oracle, which has
no persisted index to build — after materialization but before the five discarded warmups and
outside every timed window; `runCorpusSetup` records each setup command's argv, exit status and
wall time on the corpus's `corpusRecord.Setup` in the report so a reader can tell which path a
result belongs to without recomputing anything. Because setup runs before the first
`digestCorpus` call, the persisted index it writes under `.corvint/index/` is already part of the
baseline every task's before/after digest compares against, so `requireCorpusUnchanged` stays
meaningful without excluding `.corvint/` from the digest. Every `beamfall` task has a mirrored task
(id suffixed `-snapshot-present`) bound to the new corpus with identical class, command, argv and
stdin, so both paths are measured across the same task set. `validateCorpora` rejects any `setup`
entry that is not exactly `["index"]`.

## Reading a result

Every `GPK-V0-017` criterion is reported as one line:

```
<PASS|FAIL|NOT_RUN|REPORTED> GPK-V0-017(x) scope=<task[:dimension]> required="..." observed="..."
```

- `(a)` per non-query harness task: candidate p95 ≤ 100 ms reference cap **and** ≤ 250 ms portable cap.
  An index-building event (`file-change`, or `session-start` whose stdin carries
  `"startSource":"compact"`) measured on a corpus with an index setup is held to the 250 ms
  portable cap only (decision 0048 item 1); the same event on a cold corpus is `REPORTED`.
- `(b)` per query task: candidate p95 ≤ 500 ms on a corpus with an index setup; on a cold corpus
  the row is `REPORTED` (decision 0084). A manifest with no snapshot-present row for a criterion
  that has a cold `REPORTED` row gets one `NOT_RUN` line with scope `snapshot-present`.
- `REPORTED` (decision 0085): the p95 is in the receipt beside the binding row and the row is
  neither a threshold nor a `NOT_RUN`; it moves no outcome and no slice status.
- `(c)` per non-query harness task: candidate p95 at least 50% below the paired oracle p95.
- `(d)` per other supported command: candidate p95 and candidate peak resident memory each within
  +10% of the paired oracle.
- `GPK-V0-016` per corpus that was not measured.

`NOT_RUN` is not a waiver. A criterion is `NOT_RUN` when its task's measurement was invalid (the
line names the reason), when the corpus was not measured, or when the quantity is not soundly
measurable on this platform. Any `NOT_RUN` sinks the whole run's `outcome` to the typed
`insufficient_evidence`, as does any invalid task and any `--samples` override — a run cannot
certify cutover on evidence it did not gather. The three outcomes are `cutover_eligible`,
`threshold_failed`, and `insufficient_evidence`.

A scoped `corvint-perf-v0/2` manifest (`packet-5-retry-manifest.json`, decision 0085) is admitted
by `readManifest` only when its raw SHA-256 is in `admittedScopedManifests`; any edit un-admits
it. `run` refuses `--oracle`, `--samples`, and `--candidate-revision` for it. Its receipt is
`corvint-perf-report-v0/2` and carries `manifestSha256`, `claimScope`, and
`slicePerformanceStatus` (`NOT_RUN` when any row is invalid or any threshold criterion is
`NOT_RUN`, `FAIL` when any is `FAIL`, else `PASS`) while `outcome` stays `insufficient_evidence`
(`P5R-V0-003`). The retry mirrors all six Packet 5 rows on snapshot-present corpora, so it adds
`corvint-snapshot-present` (the Corvint pin with `"setup": [["index"]]`) beside
`beamfall-snapshot-present`.

Peak resident memory is read from the kernel's own high-water mark for the finished process
(`ProcessState.SysUsage()` → `syscall.Rusage.Maxrss`; bytes on darwin, kibibytes on linux). Where
no sound stdlib source exists, the criterion is reported `NOT_RUN` with that reason. It is never
estimated.

## Why the oracle is still measured

**Operator decision, 2026-08-28:** the paired Python measurement stays in the protocol even though
Python's own performance is not a goal, because `GPK-V0-017(c)` is a preregistered threshold and an
unmeasured threshold is `NOT_RUN`, not waived — dropping the oracle samples would forfeit a
criterion that currently passes.

## Running it

```sh
GOTOOLCHAIN=local go run ./conformance/perf-v0 run --out conformance/perf-v0/results/<name>
```

The harness verifies up front that the local toolchain is exactly `go1.27.0` and fails rather than
selecting another (`GPK-V0-018`). It builds the candidate itself, from the pinned revision, with
`GOTOOLCHAIN=local CGO_ENABLED=0 go build -o <workspace>/corvint ./cmd/corvint` — never as
`corvint`, which `GPK-V0-015` forbids. Everything the run creates lives in one private temporary
workspace that is removed on exit; the harness writes to no path inside any measured repository.

Flags: `--manifest` (preregistration path), `--oracle` (Python command override), `--out`
(directory for `report.json`), `--samples N` (smoke override; forces `insufficient_evidence`),
`--candidate-revision SHA` (build the candidate from that commit instead of the corpus revision).

`--candidate-revision` exists because the corpus revision drives three things at once: the measured
repository, the oracle's frozen `src/` tree, **and** the source the candidate binary is built from.
Advancing the pin to measure an unlanded kernel change would therefore move the workload underneath
the comparison. The flag moves only the candidate, so a before/after pair can be measured against
one identical corpus and one identical oracle. Such a run is stamped `preregisteredRun: false` and
can only ever report `insufficient_evidence` — the corpus, the oracle and the binary no longer share
one revision, so it is a latency comparison, never cutover evidence. Every `GPK-V0-017` criterion is
still evaluated and reported, and the console prints a `CANDIDATE` line naming the revision built
and whether it was the corpus's.

Expect roughly 15 minutes for the committed Corvint manifest. Run it on an otherwise idle machine:
the protocol's interleaving makes drift hit both runtimes equally, but it does not make a loaded
machine's numbers meaningful.

## Accepted divergences

`GPK-V0-016` invalidates a task whose runtimes disagree on stdout. That is the right rule for an
unexplained disagreement. It is the wrong outcome for a divergence that is **diagnosed, recorded,
and deliberately never repaired**: `src/` is frozen, so such a task is invalid on every run, for
ever, by construction — and the criteria it carries report `NOT_RUN` for ever with it. An
unmeasurable threshold is a worse outcome than a measured one whose caveat is disclosed.

**Decision 0007 item D9** therefore ratifies one narrow relaxation. A task may name a
divergence-register entry in `acceptedDivergence`, and the reasons that entry's grant lists are
recorded rather than invalidating:

```json
{ "id": "impact-path", "class": "other-command", "acceptedDivergence": "DR-0004", ... }
```

The narrowness is the mechanism, not a convention around it:

- **The set of ratifiable entries lives in Go, not in the manifest.**
  `ratifiedAcceptedDivergences` (`manifest.go`) holds exactly `DR-0004` and the decision that
  ratified it. A manifest may *select* from that set; it can never extend it. `validateManifest`
  refuses an unrecognised key before any process is spawned, so admitting a second entry is a source
  change that has to arrive with its own ratification.
- **A grant absorbs only the reasons it names.** `DR-0004` relaxes `unequal-stdout` and nothing
  else. `unequal-stderr`, `unequal-exit-status`, `timeout`, `corpus-changed`, `cache-state-changed`,
  `sample-failure` and `oracle-source-mutated` still invalidate a keyed task.
- **Each runtime must still agree with itself.** A runtime whose own stdout varies across samples is
  non-determinism, not the stable divergence the entry describes, and is typed
  `unstable-oracle-stdout` / `unstable-candidate-stdout`. Those reasons appear in no grant.
- **The declaration is a permission, not an assertion.** If the divergence stops occurring, nothing
  is absorbed and nothing is disclosed.
- **It is never silent.** The console prints an `ACCEPTED-DIVERGENCE` line carrying both runtimes'
  stdout digests, `report.json` carries `acceptedDivergences`, the `SUMMARY` line counts them, and
  every affected criterion's `observed` text ends in
  `[measured across accepted divergence DR-0004: unequal-stdout]`. A criterion measured across a
  relaxed predicate must never read as an unqualified pass.

**This is deliberately not `structuralComparison` (decision 0005).** That mechanism governs a field
that is a *measurement of the run* rather than a *result of the computation*, and it still compares
the field — present, numeric, non-negative. `DR-0004` is the opposite case: a result of the
computation, in which the two runtimes genuinely disagree, with no structural predicate that both
orderings satisfy while still catching a regression. Reusing structural comparison here would
relabel a real disagreement as a shape check, which is exactly what decision 0005 forbids when it
says structural comparison is not "a general escape from byte-exactness".

## The Beamfall half — measured since 2026-08-30

`GPK-V0-016` names both corpora. Beamfall was carried as a typed `NOT_RUN` for as long as the only
available checkout — `~/projects/beamfall-workspace/beamfall` — was under
continuous concurrent write by a live agent fleet, which cannot hold the same-revision,
same-cache-state and repository-unchanged predicates for the duration of a run.

W12 / `GPK-V0-022` supplied the pinned clean revision, so the corpus is now `enabled` at
`fa3b1e7fe5bc6c10e4b09b2729f364780f567a48` (tree `3a4251e0f0ebe70a489e296b32189634250f69d4`). The
live checkout is never measured directly: `materializeCorpus` archives that exact commit into the
run's private workspace, rebuilds it as a real Git repository, and refuses to continue unless the
rebuilt `HEAD^{tree}` equals the pin byte for byte. The first run to measure both corpora is
`results/corvint-beamfall-2026-08-29/`.

## Recorded evidence

`results/<name>/report.json` is the full receipt — machine identity, resolved Python version and
Go toolchain, the candidate binary's SHA-256 and the revision it was built from, the frozen oracle
source tree digest, corpus and cache digests before and after, per-task distributions with
bootstrap intervals, and every criterion verdict. `results/<name>/run.txt` is the console
transcript, ending in the `SUMMARY` line.
