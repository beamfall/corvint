# Lane Dogfood Tooling V0

Owner: Russell Lewis
Date: 2026-11-03
Requirement prefix: `DTL-V0`
Intent status: accepted
Delivery status: experimental
Authoritative inputs: owner dogtools lane charter (2026-11-03); AGENTS.md; immutable-delta-v0.md; daily-change-evidence-workflow-v0.md.

## Agent digest
- Claim: Lane tooling preserves evidence boundaries while making verification, diagnostics and local scratch predictable.
- Status: accepted / experimental
- Exists: dogfood wrappers, host adapters, affected sources and the foreground heavy runner.
- Blocked on: retained qualification and orchestrator review.
- Read next: Requirements; Acceptance evidence; Rollback.

## User and baseline

A lane needs actionable fallback diagnostics and bounded foreground verification without losing tests.
Both daily-change-evidence-workflow-v0.md and affected-plan-v0.md already contain 256 indexed
obligations. This companion uses a new prefix as the charter permits at that limit. It preserves
BASE-only CEM evidence and the opaque immutable backend's bounded-open requirement.

## Requirements

- `DTL-V0-001`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4463; refines IDX-SNAP-V0-008)
  Ordinary dirty edits MUST reuse a matching committed-tree snapshot with the observed dirty set.
  A missing tree or engine snapshot MUST remain unavailable when rebuilding exceeds the deadline;
  its diagnostic MUST name `cause: index-snapshot-missing` and `fix: corvint ... index --if-stale`.
- `DTL-V0-002`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4464; refines AHI-970)
  Hook fallback text MUST name a closed cause code and a `fix:` line. Generic unavailable reads
  MUST report `event-read-unavailable`, never interpolate arbitrary error text or invent a cause.
  Frontier absence MUST name `no-accepted-closing-authority` and explicitly say no automatic fix exists.
  A Claude sibling-repository receipt MUST print its `cause:`/`fix:` diagnostics, one pair per
  degradation code named on the receipt line, before the exact framed sibling attribution.
  The plain stdout text a compaction hook emits MUST carry the same `cause:`/`fix:` lines: a
  PreCompact fallback adds them to its frame before the pin-unavailable prefix, which would
  otherwise hide the frame from the check (malformed hook JSON included).
- `DTL-V0-003`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3761; refines ASD-V0-755)
  Lane orientation MUST check the installed binary against fetched origin/main and use a freshly
  built worktree binary for task reads when installed freshness cannot be established. Build
  failure MUST stop the flow; a dirty build is local evidence only, never a release qualification.
- `DTL-V0-004`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4457; refines DCW-V0-060)
  `make dogfood-finish LANE=1 SEAL=0` MUST bind, commit, rebind and check, leaving the checked
  sidecar at HEAD without a seal. The CLI form is `dogfood finish-lane --seal no`; default `yes`
  retains sealing. Other values MUST refuse `invalid-dogfood-seal-value` before mutation.
  Every rerun or fix hint from a stop-before-seal run MUST keep that mode: `SEAL=0` after BASE for
  a make entry (dogfood-finish, dogfood-closeout), `--seal no` for a direct finish-lane run. The
  `dogfood-inputs-check` entry never seals, so its hint carries no seal option. inputs-check
  accepts `--seal yes|no` only to preserve hints; it binds nothing. The static pre-check MUST skip
  rather than drop SEAL=0 when the installed binary's help does not list `--seal`.
- `DTL-V0-005`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3955; refines DCW-V0-440)
  Same-change requirement proposals MUST resolve a named BASE requirement refined by the new
  definition, including new code files. A definition without historical support MUST stay unknown;
  same-change intent MUST NOT be laundered into BASE evidence. The diagnostic MUST name the
  immutable BASE constraint and the new requirement's defining spec.
- `DTL-V0-006`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3790; refines DCW-V0-813)
  Doc gates MAY run independent targets with bounded concurrency (default four, configurable
  `DOC_GATE_JOBS`); every target MUST still run and failures MUST propagate. Each target MUST
  print elapsed seconds and status. A serial override MUST retain identical gate inventory. Selection-baseline replay defaults
  to at most eight independent workers, bounded by CPUs and the change count; its existing
  SELECTION_BASELINE_JOBS override remains explicit. Every immutable range and result MUST still
  be measured, ordered and checked, with identical floor/miss/fallback semantics.
- `DTL-V0-007`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4465; refines AFP-V0-041)
  A foreground package shard recipe MUST enumerate every top-level test, example and fuzz seed,
  deterministically partition names with no omission or overlap, and execute one package through
  heavy.sh. Invalid or empty inventories MUST fail closed; an empty valid shard MUST explicitly
  report no selected tests. Benchmarks are outside the ordinary test inventory. The inventory
  call MUST pass heavy.sh's real selection policy without HEAVY_WHOLE_REASON: a go test whose last
  `-list` value before `-args` is non-empty runs no test and is logged `whole reason=list-only`;
  an empty or post-`-args` `-list` MUST stay under the whole-package policy. A `-test.`-prefixed
  argument, any argument after `-args`, a non-empty `-exec`, or an inherited GOFLAGS `-exec` not
  cleared by an empty command-line `-exec=` MUST also keep the whole-package policy. A `-run`
  selection that also carries a `-test.run` alias MUST stay under the whole-package policy, and so
  MUST a `-list` or `-run` command with a `--` argument after `go test`: go passes `--` to the test
  binary, which stops parsing flags there and runs the whole package. The
  shard recipe MUST clear inherited test-selection flags (`-skip= -short=false -c=false -n=false
  -fuzz= -exec=`, plus `-list=` when executing) and MUST refuse a GOFLAGS word outside its build,
  output and test-control allowlist with `inherited-goflags-unsupported`, or an unreadable GOFLAGS
  with `goflags-unreadable`, before running any test. GOFLAGS read from a remote host's go env
  file beyond the cleared flags are not checked.
- `DTL-V0-008`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4467; refines AFP-V0-023)
  Remaining cmd/corvint and internal/tasks/cli floor reasons MUST be classified with original
  source evidence. Without a confined complete passing run and falsification evidence their
  unbounded-reader declarations MUST remain; no measured saving may be inferred.
- `DTL-V0-009`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4460; refines STR-V0-062)
  `corvint test --format json` MUST accept its existing JSON output explicitly. Unsupported
  formats MUST name json as supported; --summary or --pins with explicit --format MUST refuse.
- `DTL-V0-010`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4458; V1-4459; refines AFP-V0-023)
  The lane guide MUST document CORVINT_CONFINE_ROOT for local confinement and the explicit
  `affected --worktree --path PATH` negative-control recipe. Missing confinement root MUST name
  the variable and give a pasteable export command; it MUST NOT silently run unconfined.
- `DTL-V0-011`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4461)
  companionrelease runWithStdin MUST preserve the bounded subprocess error with stderr once.
- `DTL-V0-012`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4462; refines DCW-V0-349)
  Doc gates MUST reject tracked paths matched by ignore rules using `git ls-files -ci
  --exclude-standard`; Git failures MUST fail closed. A diagnostic lists paths and recovery.
- `DTL-V0-013`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4277; refines DLT-V0-002)
  ReadBounded MUST support an explicitly admitted in-memory fstest.MapFS with the same path,
  regular-file, size, read-growth and sticky-error rules. Opaque backends without OpenBounded
  MUST still refuse before opening; Git source pre-allocation admission MUST remain intact.
- `DTL-V0-014`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3824; refines ASD-V0-062)
  Lane scratch MUST support CORVINT_SCRATCH_ROOT and default to the user cache on tmpfs hosts.
  heavy.sh MUST check /tmp headroom as well as chosen scratch, GOTMPDIR and build caches before
  load waiting or scratch indexing; a low-space run MUST refuse with paths, floor and recovery.
  The check before host choice MUST use the local defaults the run would apply (argv overrides,
  then CORVINT_SCRATCH_ROOT or tmpfs defaults, then the TMPDIR-derived cache), never an unused
  cache path, and MUST check heavy.sh's own transport TMPDIR separately. A relative
  CORVINT_SCRATCH_ROOT MUST be resolved against the working directory before the check, and the
  free-space walk to an existing ancestor MUST terminate at `/` or `.` for any path.
  Per-run scratch MUST be private and cleaned on exit; no other caller's scratch may be removed.
- `DTL-V0-015`: (accepted under the owner's standing auto-accept (2026-10-11); dogtools observed blocker; refines ASD-V0-952)
  Requirement-block checks MUST admit the charter's four-digit numbers and preserve exact
  boundaries, collision detection and three-digit compatibility, both in the base tree and in
  the open-ref filter of `requirement-id-collisions.sh --block`.
- `DTL-V0-016`: (accepted under the owner's standing auto-accept (2026-10-11); dogtools observed blocker; refines DCW-V0-020)
  Check report replacement MUST stage a private mode-0600 temporary beside the report and rename
  it atomically, including when the Git directory is on another filesystem or sandbox mount.
  `.corvint/.dogfood-report-*.tmp` MUST be documented and ignored as unfinished local output,
  never evidence. Creation, write, close or rename failures MUST preserve the previous report,
  clean up only this invocation's temporary, and retain the cause with `dogfood-report-write`.
  Successful replacement MUST also remove its temporary. Verifier agreement and bindings stay unchanged.

## Non-goals and failure modes

No authority from same-change specifications; no narrowing of unbounded readers; no cache reuse or
new skip policy. Independent review, native ticket writes and publication belong to the orchestrator.
The shard recipe emits `invalid-arguments` or `empty-inventory` on invalid/unavailable input,
and `inherited-goflags-unsupported` or `goflags-unreadable` on an unsafe or unreadable GOFLAGS,
and its inventory admission is logged as `whole reason=list-only`;
worktree building emits `worktree-build-failed`, and disk refusal emits `low-disk`. The existing
bounded-output fallback `corvint-output-too-large` also carries a cause/fix. Other validated
frame codes retain their own cause code; an unclassified frame uses `adapter-fallback`.
A `--` after `go test` keeps a `-run` selection under the whole-package refusal with the cause
`a -- argument ends the test binary flags` (DTL-V0-007). A missing relative scratch root never
hangs the disk check (DTL-V0-014).
Unavailable metrics remain NOT_OBSERVED. Existing operation locks are never stolen by a lane.

## Acceptance evidence

Tests cite the requirement in their name or the source comment. Retain actual commands, exit codes,
positive and negative controls, failed fault injections and timings in the dogtools build log and
handback. Full repository and external interoperability qualification are outside this lane.

## Traceability

| Requirement | Implementation | Proving evidence |
|---|---|---|
| DTL-V0-001 | cmd/corvint/local_completion_event.go; host_adapter.go | TestDTLV0001DirtyEditsReuseSnapshot |
| DTL-V0-002 | cmd/corvint/host_adapter_diagnostics.go; host_adapter_projection.go; host_adapter_compaction.go (`compactionPlainOutput`) | TestDTLV0002FallbackDiagnostics; TestDTLV0002PreCompactPlainFallbackDiagnostics; TestClaudeCompactionDegradationIsPlainText; TestAHI970DogfoodEventUnavailableNamesCauseAndRecovery; TestAHI803SiblingRepositoryEditIsAttributedToItsOwner |
| DTL-V0-003 | script/lane-corvint.sh; docs/LANE-ORCHESTRATION.md | TestDTLV0003WorktreeBuild; retained worktree binary and path-control receipts |
| DTL-V0-004 | internal/dogfoodflow/finishlane.go; finishlane_entry.go; cmd/corvint/dogfood_flow.go; script/dogfood-finish.sh; Makefile | TestDTLV0004FinishStopsAfterCheck; TestDTLV0004RetryKeepsStopBeforeSeal; script/dogfood-cache_test.sh |
| DTL-V0-005 | internal/dogfoodflow/propose.go; propose_refresh.go | TestDTLV0005SameChangeRefinementNewFiles |
| DTL-V0-006 | Makefile; script/doc-gate-timed.sh | TestDTLV0006TimingPreservesFailure; measured per-target timings |
| DTL-V0-007 | script/go-test-shard.sh; script/heavy.sh | TestDTLV0007ShardCoverageAndFailure; TestDTLV0007ShardInventoryRealPolicy; TestDTLV0007InheritedGoflagsKeepCoverage; TestDTLV0007ListAliasAndArgsRefused (including `--` after `go test`); retained real selected-failure control |
| DTL-V0-008 | .corvint/unbounded-readers.json | Source audit in docs/build-log/2026-11-03-dogtools.md; confined admission NOT_PRODUCED |
| DTL-V0-009 | cmd/corvint/test.go | TestDTLV0009TestFormatJSON |
| DTL-V0-010 | .github/testconfine/cmd/main.go; lane guide | TestDTLV0010MissingRootNamesRecovery (PLANNED) in .github/testconfine/cmd/main_test.go, landing with the testconfine change; retained uncommitted path controls |
| DTL-V0-011 | internal/companionrelease/export.go | TestDTLV0011StderrOnce |
| DTL-V0-012 | script/tracked-ignored-check.sh; Makefile | TestDTLV0012TrackedIgnoredFault |
| DTL-V0-013 | internal/liveverify/affected/source.go | TestDTLV0013MapFSBounded; TestDTLV0013BuildMapFS; TestDTLV0013FakeLanguageBuildCached |
| DTL-V0-014 | script/heavy.sh; lane-binary-check.sh; lane-stage-inputs.sh | TestDTLV0014DiskHeadroom; TestDTLV0014ScratchRootRecovery; TestDTLV0014RelativeScratchRootTerminates; lane admission fixtures |
| DTL-V0-015 | script/requirement-ids-free.sh; requirement-id-collisions.sh | TestDTLV0015FourDigitBlocks; TestDTLV0015FourDigitOpenRefCollision; retained charter-block checks |
| DTL-V0-016 | internal/dogfoodflow/check.go; .gitignore; docs/DOGFOOD.md | TestDTLV0016ReportPublication; retained cross-mount check |

## Rollout and rollback

All changes remain experimental. Remove SEAL=0 support and shard recipes to restore prior flow;
restore serial doc gates with DOC_GATE_JOBS=1; restore the closed FS admission switch to refuse
MapFS. Revert diagnostic and scratch changes independently. No old receipts or traces are erased.
Reverting DTL-V0-016 restores Git-directory staging and its cross-filesystem refusal; keep any
unfinished local report temporary ignored until its operator disposes of it.
