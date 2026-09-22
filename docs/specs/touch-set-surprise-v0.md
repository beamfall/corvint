# Touch Set Surprise V0

Owner: Russell Lewis
Frozen: 2026-09-11
Requirement prefix: `TSS-V0`
Intent status: proposed
Delivery status: experimental
Authoritative inputs: `docs/specs/task-context-packet-v0.md` (the packet whose paths are the
prediction), `docs/specs/go-production-kernel-migration-v0.md` (`impact` receipts and the sanitized
Git execution pattern), `docs/specs/snapshot-batch-v0.md` (snapshot load convention), `AGENTS.md`
invariants 1, 2 and 4.

## Agent digest
- Claim: `corvint surprise` measures, for one completed task, the share of the committed change's paths the task-context packet and its impact expansion never named.
- Status: proposed/experimental
- Exists: `internal/touchsurprise` and `cmd/corvint/surprise.go` with their tests; nothing before this slice.
- Blocked on: CEM binding as the actual set (V0 reads the committed range alone); a reviewer-trial correlation between this score and reviewer-found misses; any consumer, including `internal/outcomecal`.
- Read next: Requirements; Non-goals and authority; Failure modes.

## User and measurable job

An agent that finished a task has two artifacts: the context packet it was given before the change,
and the change it actually committed. Nothing today says how far apart they were. The affected user
is whoever is deciding whether the packet is worth trusting on the next task of the same shape. The
measurable job is one read-only number over committed evidence: of the paths the change touched,
what share did the packet and its own impact expansion never name? A high score is not a defect
claim; it is a bounded, reproducible pointer at where retrieval was blind.

This contract is proposed and its delivery experimental. It claims no ranking effect, no learning
input, and no reviewer-grade judgement of whether a miss mattered.

## Requirements

- **TSS-V0-001:** `corvint [--root PATH] surprise --task TEXT --base REV --target REV
  [--subject PATH] [--limit N]` MUST parse in `cmd/corvint/surprise.go` following the
  `observations`/`depsource` argument conventions (`--flag value` and `--flag=value`, leading
  `--root` only), exposing `runSurprise(ctx context.Context, arguments []string, stdout, stderr io.Writer) int`.
  `ctx` is the process signal context (`TSS-V0-006`). An empty
  `--task`, a missing or `-`-prefixed `--base`/`--target`, an unknown flag, or a `--limit` outside
  1..50 MUST be refused with `invalid-arguments` (`argumentError`) and exit status 2, before the root
  is resolved and before any repository read. `--limit` defaults to 20.
- **TSS-V0-002:** The predicted set MUST be the paths of `contextindex.TaskContext(ctx, index, task,
  subject, limit)` (each result's `id`) unioned with the paths of `contextindex.Impact(index, seeds,
  limit)` over those of them that are tracked sources, capped at `Impact`'s own 100-path bound in
  path order. An `Impact` row counts only when its `id` is a tracked source: a `feature` or
  `scenario` row names a ledger record, not a path, and MUST NOT enter the predicted set. The receipt MUST record the provenance split: `predicted.from_packet`,
  `predicted.from_impact` (impact paths the packet did not already name) and their union
  `predicted.paths`, with `predicted.impact_seeds_truncated` stating whether the cap applied.
- **TSS-V0-003:** The actual set MUST be the changed paths of the committed range, read as
  `git diff -z --name-only --no-renames BASE TARGET --` through a sanitized, closed-environment Git
  execution copying `internal/contextindex/git_execution.go`'s and `git.go`'s pattern (no ambient
  configuration, no prompt, bounded output). Paths MUST be read as the literal NUL-separated
  spelling, never newline output, which C-quotes a non-ASCII or quote-bearing path. Both revisions MUST resolve through
  `git rev-parse --verify --quiet REV^{commit}`, and the receipt MUST carry the resolved commit
  object ids under `range`. An unresolvable revision MUST be refused with
  `unsupported-surprise-revision`, and a worktree with uncommitted tracked changes with
  `unsupported-surprise-dirty-worktree`: this verb compares committed evidence only.
- **TSS-V0-004:** `touchsurprise.Compute(predicted, actual []string) Report` MUST be a pure function
  of its two arguments and MUST report, over the deduplicated sets in path order (paths compared by
  their literal spelling: only an empty string is dropped, and edge whitespace is never trimmed),
  `predicted_only`, `actual_only`, `intersection`, `symmetric_difference` (the two difference sizes
  summed) and `surprise = |actual_only| / |actual|` rounded to four decimals. An empty actual set
  MUST score `surprise: 0` with `empty_actual: true` rather than dividing. Every list the receipt
  emits MUST be sorted, so two runs at one range are byte-equal.
- **TSS-V0-005:** Each `actual_only` path MUST appear in `misses` with `tracked_at_target` (its
  presence in `git ls-tree -r -z --name-only TARGET`, read literally), `test_path` and `doc_path`. The latter two are
  declared heuristics over the path spelling alone (`_test.go`, `test_` prefix, `.test.`/`.spec.`
  infix, a `_spec.rb` suffix, a `test`/`tests`/`testdata`/`conformance` directory segment; a `.md`/`.mdx`/`.rst`/`.txt`/
  `.adoc` suffix or a `docs/` prefix), and the receipt MUST say so in `uncertainty` rather than
  claim an extractor verdict.
- **TSS-V0-006:** The verb MUST be read-only (invariant 4): it loads the snapshot through
  `loadSnapshot` and falls back to `contextindex.BuildContext` exactly as `context defs` does, writes
  no repository, `.corvint/` or trace state on any path, and emits one canonical JSON receipt on stdout
  with exit status 0 or one `emitError` line on stderr with exit status 2. Clarifying amendment
  (2026-09-13, bug hunt): the load runs under the process signal context, so SIGINT or SIGTERM
  before the receipt compiles is that same loader failure (the loader's cancellation envelope, exit
  2), never an ignored signal followed by an exit-0 answer.

## Non-goals and authority

No ranking input: no score, ordering or admission in `contextindex` may read this receipt. No
learning: nothing here is written to a trace, a learned weight, or an evaluation corpus (invariant
5). No CEM consultation in V0 — the actual set is the committed range and nothing else. No claim
that an `actual_only` path *should* have been predicted, that a `predicted_only` path was waste, or
that a low score means the packet was sufficient; a miss is an observation about retrieval coverage,
never a defect verdict about the change or the agent. No cross-task aggregation, no baseline, no
threshold. `internal/outcomecal` may later call `Compute`; this slice wires no consumer, and the
exported function carries no authority the caller does not supply. A receipt carries no more
authority than the packet and the Git range it names.

## Failure modes

Argument failures (empty task, missing or `-`-prefixed revision, unknown flag, out-of-range limit):
`invalid-arguments`, exit 2, nothing read. Root not a Git repository: `argumentError`'s message,
exit 2. No snapshot: the `context defs` fallback builds the index; a build failure is that error,
exit 2. Unresolvable `--base` or `--target`: `unsupported-surprise-revision`, exit 2. Uncommitted
tracked changes: `unsupported-surprise-dirty-worktree`, exit 2. Any other Git failure (unreadable
repository, output over the 8 MiB bound): `unsupported-surprise-git`, exit 2. A packet refusal
(oversize task, untracked `--subject`, out-of-range limit) or an `Impact` refusal
(`unsupported-impact-repository` for a `.go` seed in a repository with no slash-qualified module) is
that `contextindex.Error`, exit 2 — the verb abstains rather than reporting a prediction it could not
compute (invariant 2). An empty packet is not a failure: the predicted set is empty and every changed
path is a miss, which is the honest reading. SIGINT or SIGTERM during the load: the loader's
cancellation envelope, exit 2, empty stdout (`TSS-V0-006`).

## Acceptance evidence and traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| TSS-V0-001 | `cmd/corvint/surprise.go` `parseSurpriseInvocation`, `runSurprise` | `TestSurpriseRefusesIncompleteArguments`, `TestSurpriseIgnoresOtherVerbs` |
| TSS-V0-002 | `internal/touchsurprise/touchsurprise.go` `predictedPaths` | `TestSurpriseSeparatesPacketAndImpactPredictions`, `TestSurprisePredictsOnlyTrackedImpactPaths` |
| TSS-V0-003 | `internal/touchsurprise/git.go` `resolveCommit`, `requireCleanWorktree`, `changedPaths` | `TestSurpriseRefusesUnresolvableRevision`, `TestSurpriseRefusesDirtyWorktree`, `TestSurpriseReadsLiteralNonASCIIPaths`; `changedPaths` over a real range in `TestSurpriseReportsActualOnlyPathsAsMisses` |
| TSS-V0-004 | `internal/touchsurprise/compute.go` `Compute` | `TestComputeReportsSetArithmetic`, `TestComputeAndRenderReportEmptyActualRange`, `TestComputeKeepsEdgeWhitespacePaths` |
| TSS-V0-005 | `missRow`, `isTestPath`, `isDocPath`; `trackedPaths` | `TestSurpriseReportsActualOnlyPathsAsMisses`, `TestSurpriseReadsLiteralNonASCIIPaths` |
| TSS-V0-006 | `runSurprise` load path; no write path in the package | `TestSurpriseWritesNothing`, `TestRunSurpriseHonorsCancellation`; `cmd/corvint`: `TestCLIReadVerbsLeaveTheRepositoryByteIdentical` (CLI-level repository-byte assertion) |

Compatibility and drift: the predicted set is the packet's and `impact`'s own result ids (impact ids filtered to tracked sources), so a change
to either receipt's `results[].id` changes this measurement in the same commit. Unresolved (owner
review): whether the actual set should later come from the committed CEM's bound diff rather than a
supplied range, and whether `predicted_only` deserves its own name once a cost denominator exists.

Rollout: experimental Go verb only; no adapter, evaluation or dogfood step calls it, and
`internal/outcomecal` does not consume `Compute`. Rollback: delete `internal/touchsurprise`,
`cmd/corvint/surprise.go`, their tests, the help entry, this spec and its README/INDEX rows.
Promotion or kill: promote only if, on the CEM reviewer trial, a higher surprise score coincides with
the reviewer-found context misses on the same changes; kill it if the score does not track them.
