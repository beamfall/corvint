# Selection baseline gating and replay concurrency — 2026-11-05

Owner-owned intent: lane sbgate, V1-4329 and V1-4724; base
`28a3ff86a2c3d17662765da12cc9251c06466f5f`. The owner requested a diff-based,
fail-closed skip shared by batch and CI, ordered parallel replay, actual timing,
and focused verification. This entry records the design choice and measurements,
not ticket completion or release qualification.

## Decision and scope

Amend ASD-V0-944 and ASD-V0-925 in place, with the matching worker obligation in
DTL-V0-006. ASD-V0-1990 still defers both lane passes without building the baseline;
its non-lane reference now points to the amended ASD-V0-944. No new requirement IDs.

The outer batch selection is deliberately conservative and retains its existing
messages and receipt rules. When it schedules the target it passes its immutable
selected base. The Make target builds only the small replay tool for its shared
preflight, before building corvint and the selector or replaying anything. Direct
CI doc-gates uses the same target, deriving the full base from the PR, merge-group
or push event. A missing base runs; HEAD's parent is never guessed. No workflow edit.

The preflight combines the actual transitive production compilation closure
(`go list -mod=readonly -deps -json`) with Corvint's own `gateselect.Bounds` reader
index and the replay driver's declared scope. Entire compilation package subtrees
conservatively cover new/deleted/embedded files and test data; this is a
safe overestimate, not a claim to isolate individual functions in cmd/corvint.
The current-tree runtime reads are go.mod, shard costs and the fixed change set.
Corvint and the selector themselves execute in detached fixed historical clones,
so their test-time current-worktree unbounded-reader floors do not describe this
replay's current-tree runtime inputs. The driver's declaration records those
runtime reads, and a missing/unbounded driver bound runs rather than assuming
that tests or a plan proved it bounded. Changed declarations, build controls,
benchmarks and costs also run. Ignored matching files run; unavailable/symbolic
compilation inputs, native include/link directives outside the Go input bound,
local external replacements, incomplete closures and vendored
compilation fail closed. Decisions name the first matching path, or the complete
base and disjointness/unknown reason.

The replay already used a clone pool and indexed result/error slots. Retain that
implementation and every report/verdict byte; change only its default admission
to min(CPUs, 16, change count, available GiB), including observable cgroup remaining
memory. Unobservable memory defaults to two. Explicit SELECTION_BASELINE_JOBS
continues to override the default. No affected/dry-run hot-path or golden changes.
Rollback: remove the Make preflight and use SELECTION_BASELINE_JOBS=1; restore the
previous default cap if required. The independent reviewer is orchestrator-owned.

## Measurement method and original cost

Host: 32 CPUs, Go 1.27.3, shared warm build cache. Raw artifacts and probes are
outside the repository in `/home/russell/lanes/sbgate-evidence/`. A foreground
`time -v` run wraps the full 58-change tool; thin external wrappers timestamp each
actual corvint/selector subprocess and preserve its stdout, stderr and status.
The same corvint and selector binaries are used for all replay comparisons,
including the version/provenance field. Times describe this shared host, not a
controlled estimate for every CI runner.

Original two-worker full replay: **600.97 s**, exit 0, 2147.36 CPU seconds
(357% CPU), maximum child RSS 341988 KiB. Initial three-binary build under heavy.sh:
**20.8 s**, zero slot/load wait; a warm replay-tool-only build was 0.4 s.
The timestamp interval is 2026-11-06 01:07:13.866 through 01:17:08.275 UTC
(2026-11-05 local). The remaining outer time includes origin reachability and
startup/cleanup. This run overlapped the later two-worker control in its tail;
retain that shared-host limitation instead of claiming an isolated experiment.

| Phase | Calls | Aggregate worker seconds | Median s | Max s |
|---|---:|---:|---:|---:|
| affected planner | 58 | 198.408 | 3.157 | 8.588 |
| test dry-run | 58 | 863.216 | 5.069 | 93.116 |
| CI selector | 58 | 76.412 | 1.061 | 4.025 |

The subprocess wall sum is 1138.076 s, versus a two-worker capacity of 1201.94 s.
About 64 worker-seconds remain for checkout, clone setup, Git diff, scheduling,
reachability and tail idle capacity together; they are not all semaphore waiting.
The dry-runs account for 75.85% of measured subprocess wall time. The longest are
pr-41 (93.116 s), ci-af9d23f80 (78.207), pr-39 (77.004), ci-0df97c8c7 (76.088),
ci-cbc790640 (70.525) and ci-4a7bf04c8 (49.794). Compilation is not the dominant cost.

A post-change control accidentally retained the host's inherited
SELECTION_BASELINE_JOBS=2: **645.13 s**, exit 0; it is not the new-default result.
Its complete report, stdout and ordered progress bytes equal the original run.
It overlapped other work, and establishes no cache or concurrency speedup.
The subsequent default probe explicitly clears the inherited setting.

| Full 58-change replay | Workers | Wall s | CPU s | Max child RSS KiB |
|---|---:|---:|---:|---:|
| Original (`before`) | 2 | 600.97 | 2147.36 | 341988 |
| New serial comparison (`serial`) | 1 | 1190.57 | 2115.62 | 349236 |
| New default probe (`parallel`) | 16 | 185.25 | 2771.67 | 419428 |
| Final committed implementation (`qualified`) | 16 | 224.19 | 2916.97 | 411736 |

Every run exited 0 with 58 affected, 58 dry-run and 58 selector calls (plus one
version probe), all subprocesses exit 0. The final implementation is **2.68x**
faster than the original two-worker run, reducing measured replay wall by
**376.78 s (62.7%)**. The earlier default probe is retained to show host variation,
not substituted for the committed-binary qualification. The serial/default probe
predates only preflight hardening, which those full replays do not enter; replay
and memory-admission source are identical to the qualified implementation.
The final rerun uses implementation commit
`0ffe3797a04e5ac9949bf0d1dfee8672b740eeb2`; the later test-helper writer correction
changes no production code.

Complete 914516-byte reports, stdout and all 58 ordered stderr progress lines are
byte-identical across original two-worker, post-change two-worker, one-worker and
both 16-worker runs. No normalized/sorted comparison and no golden regeneration.
The `/usr/bin/time` footer is timing evidence, not tool stderr; only that footer is
excluded when comparing tool progress. SHA-256 digests:

- Each full JSON report: `cdf9032be5f63f254e31cd83a6a13839b1e4ebcd84cfac2b53b95a0e112483db`.
- Each stdout: `b1c8b106770080e77217acd2ec0581f23d97190fd3a22df9bc17bf68db6d95b4`.
- Each tool progress stream: `a15d24cc3358509d1873d57a45d4cb0a777404276834ef042e1d9277eb2f5534`.
- Original timestamp ledger: `3924826c7f7c6dcbab515390f90b8f8f33ed8727b6c732d7bfed6e54a2bbafb5`.
- Final timestamp ledger: `77695316df14d17b673e87ad34b266afb1eaaf4eb0eed0e4668cd9899cbe2f6c`.
- Byte-comparison manifest (`replay-byte-proof.json`): `287e1923ca81214d66d5caf7889f1445eaae0bc7162f024d95032684edc65041`.

Original binaries were built at the fixed base: replay tool SHA-256
`c21301fc08dfef0b308931e499c46da29f678ed20072725b7a1c84bf812bb03c`,
corvint `2e82536b5f2ee07af988b0aaa9e6c6a215e074065b8deb99d9d510c31a5df57c`,
selector `13df16984ee8b57c6be387f4d4653a42dcb828e4de4112aa57d04f737b2d454c`.
Final tool SHA-256: `0f52e08e9fd112384c29fe1a3fab3666786849e5e11ebc4ae0ae8f4d62da044c`.
For reproduction in the evidence directory, use its timestamping corvint/selector
wrappers, `TMPDIR=/home/russell/lanes/sbgate.tmp`,
`SB_TIMINGS=<stem>-timings.jsonl`, and `/usr/bin/time -v <tool> --check --corvint
<evidence>/corvint --selector <evidence>/gate-affected-select --out <stem>.json`.
Set SELECTION_BASELINE_JOBS=1 or 2 explicitly for controls; run through
`env -u SELECTION_BASELINE_JOBS` for the default. Do not regenerate committed outputs.

## Last 20 first-parent commits

The audit pins the origin/main observed at lane start,
`28a3ff86a2c3d17662765da12cc9251c06466f5f`, and checks each of its last 20
first-parent commits in a separate shared clone at that exact head against its
first parent, with its own compilation closure and declarations. **6/20 skip**;
all remaining decisions name a matching input (no unknowns).

| Commit | Decision |
|---|---|
| `28a3ff86a2c3d17662765da12cc9251c06466f5f` | matching input .corvint/test-read-scopes.json |
| `cdeed7ee6396a8d6bf6225eb5b2b4cb4d601b419` | matching input .corvint/unbounded-readers.json |
| `8e2c13aa926fcaf1731fae71b27c41c41c8f09af` | matching input .corvint/test-read-scopes.json |
| `39c03198086681fb24f509fbd00b34696d3268c4` | skip: disjoint |
| `d436dff76de1b8e75c427069ae7f80003ae99bc2` | matching input .corvint/unbounded-readers.json |
| `6c33df33bc5670131482f5263b410cb980580657` | matching input Makefile |
| `ea770fa17992b970aeda570ed74686a16dbc441e` | matching input .github/cishards/package-costs.json |
| `24d8c04f9f5ffaaed86759951e520cb775dc2eff` | skip: disjoint |
| `f13f52b1607222033b089144a744fedae0d35239` | skip: disjoint |
| `9fdc2f02efc11ce9ed582d8529b63fe0478fcad2` | skip: disjoint |
| `6f12f4704235dc8f9bea19fc12995bfb68c746c3` | matching input cmd/corvint/affected.go |
| `14e055d69e3d18979f4eb5e33aae8eb4f16848fd` | matching input internal/langplugin/decl/decl.go |
| `48419b754f950e1b4e46deb3e4d09b5e1d908c60` | matching input internal/langplugin/decl/decl.go |
| `64321325b76302fc11fb5b636b45927cb099c56f` | matching input .corvint/test-read-scopes.json |
| `f89db15bd8a1f92887cde4d0c33e0aa26262c2a4` | matching input internal/contextindex/dialect_hooks.go |
| `7169198944f962d90bab5ba69d2bcd3e4d181bed` | matching input .corvint/test-read-scopes.json |
| `317e914371079f5a7458dea99ccf365e2006ee52` | matching input .corvint/test-read-scopes.json |
| `f53d81d8f81ea2994afd313a4062151c899c7b45` | skip: disjoint |
| `787223ab357285a5ab019b9cba937381815c3e40` | matching input .corvint/test-read-scopes.json |
| `463f0dd5fb58132ce532e24e143484aa23ef3713` | skip: disjoint |

Final audit uses the preflight binary from implementation commit
`0ffe3797a04e5ac9949bf0d1dfee8672b740eeb2`. Median decision time for skipped
commits was 2.916 s (excluding the small tool build).
Raw audit (`last20-qualified.jsonl`) SHA-256: `940458cb16b7c399cd32d0a09ca8d46a1b9a482562677be675f929dc03798f30`.


## Acceptance, use evidence and limits

- Whole existing and new tests of `./tools/selection-baseline` ran through
  script/heavy.sh, one package per command, with no -run filter. The actual
  affected plan selected that package. Its retained unknowns include language
  frontiers; focused selection does not turn them into whole-repository coverage.
- TestBaselineInputDecisionASDV0944 uses real Git diffs, compilation dependencies
  and reader declarations: docs, unrelated Go and top-level paths skip; engine,
  changes.json, shard costs, ignored source and a declared runtime input run.
  Malformed declarations and unavailable compilation run. Negative tests cover
  missing/unbounded reader bounds and unpinned external inputs, including valid
  symlink targets. CI base tests cover PR, merge-group, push and invalid events.
- TestBaselineReplayParallelBytesASDV0925 compares complete JSON and progress
  bytes with one and three workers. A FIFO forces the second change to complete
  first without a timing race. Existing assertions and reports are unchanged.
  TestBaselineMemoryWorkersASDV0925 covers bounded defaults and explicit overrides.
- Existing batch-precheck groups 15 and 17 passed, covering outer selection,
  receipt reuse, both lane deferrals, no planner, no base, non-lane rerun and
  deferred-receipt refusal. Actual full lane-closeout wall is NOT_OBSERVED:
  the charter prohibits running doc-gates/closeout here. The measured avoided
  replay cost above is not presented as a measured full closeout total.
- The specs were staged before make spec-requirements; focused
  spec-requirements-check and line-citations-check passed. Repinning found no
  citations whose unchanged content moved; the existing checker noted 180 citations
  with non-root basenames that it skips. No whole-repository selection or gate.
- Self-development routes used: pre-change context/query via dogfood, affected
  planning. Final CEM/reviewer report/check and explicit local outcome are produced
  after the material commits; their actual status belongs in the handback. Initial
  prechange-impact was NOT_PRODUCED because the mixed Go/shell/Makefile path list
  is unsupported; its actual refusal is retained. No invented impact receipt.
  Ranking/learning and external interoperability evaluations are not applicable
  to this tooling-only slice; frozen 58-change replay is the relevant evaluation.
- NOT_RUN: doc-gates, selection-baseline-check target, full repository tests/vet,
  selection baseline whole-repository selection helpers, model-backed review,
  push/PR, seal and native Tasks writes, as the charter requires. Independent
  review, host verifier and native ticket completion remain orchestrator-owned.
  Structural CEM checking is not an independent correctness review.

## Friction retained

1. `corvint-tasks ticket show V1-4329` refused: this checkout has no journal and its
   committed projection lacks the ticket. Read-only CORVINT_TASKS_SNAPSHOT fallback
   at `/home/russell/lane-inputs/ticket-bodies-fr-scripts/queue-snapshot.json` served
   V1-4329. The snapshot is dated 2026-11-04 and has no V1-4724; that ticket's body
   remains NOT_OBSERVED after `ticket show V1-4724` refused. The explicit owner
   charter governs the delivered V1-4724 scope; no Tasks write attempted.
2. `make dogfood-prechange BASE=... PATHS="tools/selection-baseline/main.go script/batch-precheck.sh Makefile"`
   retained context/query but refused impact with unsupported-impact-path-suffix.
   The initial directory-list attempt failed identically. Bind/finish must keep
   that NOT_PRODUCED visible rather than manufacture a pre-change result later.
3. Installed `corvint affected --base ...` reported self-binary-stale (d436dff76de1).
   Repeated selection with the current-base binary built from this worktree;
   retained both outputs, including the original warning.
4. `printenv SELECTION_BASELINE_JOBS` returns `2` in the lane environment. Omitting
   an assignment did not exercise the new default. Use `env -u SELECTION_BASELINE_JOBS`
   to qualify the default; an intentionally explicit two-worker override remains
   authoritative and must be removed by its operator to obtain the larger default.
5. `script/check-line-citations.sh --repin` initially noted unstaged Makefile edits;
   staged the implementation and repeated it before the focused final check.

6. The shared origin/main ref advanced during the second audit (from the pinned
   base to c5379b4cf039fa8dd2df2aa4ee6e5b7b897cf514), changing the 20-commit window.
   Reproduction: compare `git rev-parse origin/main` with the charter base.
   Repeated the final audit against the immutable lane-start main above; the
   intermediate moving-window output is retained separately, not mixed into it.

7. The implementation commit succeeded as 0ffe3797a04e5ac9949bf0d1dfee8672b740eeb2
   but Git printed `Unable to create /home/russell/projects/corvint/.git/packed-refs.lock:
   Read-only file system`. That path is outside the permitted metadata roots.
   No permission override or packed-ref write was attempted. Later explicit
   commits disable automatic maintenance/GC per command, without changing Git config.

8. The current-tree dogfood binary build through `script/heavy.sh --admit-only go build`
   queued for host load 57 > HEAVY_MAX_LOAD=40; the normal runner waited 60 s
   until load 39, then built in 4.4 s with zero slot wait and exit 0. No semaphore,
   lock-directory or load override was used. This queue is separate from replay wall.
