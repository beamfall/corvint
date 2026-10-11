# V1-0315 — task-context exclusion disclosure

Date: 2026-10-10. Intent: TCP-V0-065 and the TCP-V0-006 amendment are
**accepted, decision 0516** (V1-0315). Delivery: experimental local
implementation; passing tests alone did not accept intent.

## Contract and evidence

Decision 0394 accepted excluding non-UTF-8 tracked paths instead of refusing the
repository, but left context-only disclosure open. The proposed packet amendment
adds an optional top-level `exclusions` object with the impact receipt's total
population (recorded rows plus unsupported suffixes) and at most ten recorded
path/reason samples, sorted by display path then reason. Samples are evidence gaps,
separate from results. The implementation copies before sorting and does not
change ranking, coverage, state, packet schema version or index/snapshot formats. Zero
exclusions produce no member. Rollback removes only the member and its assembly.
The existing analyzer-input audit deliberately includes consumer-only changes;
its required cache identity is bumped from `corvint-analyzer/113` to `/114`, with
the audit schema and source digest repinned. This invalidates derived analyzer
packs conservatively without changing their encoding or extraction behavior.

The requirement index's previous TCP maximum was 064. The 2026-10-10 open PR
inventory (749–759) and all eleven diffs had no additional TCP IDs; TCP-V0-065 was
allocated after that check. The changed spec was staged before generating
`REQUIREMENTS.tsv` with `make -s spec-requirements`.

Frozen evidence in `internal/contextindex/taskcontext_exclusions_test.go` uses the
existing Git-committed Latin-1 path fixture, with clean and deleted-worktree
variants, without naming the excluded path or supplying a subject. It checks the
display path/reason, READY results, and eager/repeated/snapshot packet byte parity.
Constructed exclusions check the ten-row cap, path/reason ordering, total count,
index non-mutation, task independence, old-consumer decoding and unchanged prior
packet members. A suffix-only fixture checks a count with empty samples; the
existing no-exclusion golden is compared byte for byte without regeneration.

## Local verification

- Baseline: focused non-UTF-8 indexing, old-consumer wire, and default-byte tests
  passed before implementation.
- Focused context tests passed, including all four new tests and the existing
  non-UTF-8, additive-wire and default-byte tests. After strengthening the combined
  count assertion, all four new tests passed again.
- Focused CLI wire, LSP-off wire and summary/expand read-only tests passed; existing
  goldens needed no changes.
- A Go overlay of the unchanged base implementation failed the new Latin-1
  disclosure regression for both clean and deleted-worktree variants, specifically
  at the missing `exclusions` assertion after successful fixture indexing and
  packet compilation. The current implementation passes those same assertions.
- Independent native read-only review found no actionable issues in the code,
  proposed requirements, wire compatibility or regression coverage.
- Full `internal/contextindex` package ran for 1500.577 seconds and failed two
  tests: `TestAnalyzerSchemaInputs` required the cache identity/audit update above;
  `TestExpandedRangeImpactGlobalCoverage` hit `Git repository index exceeded its
  deadline` during fixture indexing. This failed run remains retained in
  `/private/tmp/v1-0315-package.log`; it is not reported as a passing package gate.
  A targeted rerun of both failing tests, the analyzer pack cache-identity test,
  and all four new disclosure regressions passed together in 4.844 seconds. The
  range-impact deadline did not recur, and no range-impact code or limit changed.
  The full package was not repeated after this focused repair. Independent
  follow-up review verified the audit pins and digest, with no findings and no
  further requirement amendment needed under the existing cache-audit policy.
- All requested documentation gates passed: spec-requirements, requirement
  definitions, traceability tests, decision numbers, line citations, error-code
  ownership, unbounded readers, use-case receipt checking and the five-case
  receipt-repin test. The complete requested documentation gate command was
  repeated after the cache-audit repair and passed again, as did the final CLI
  wire/LSP-off/summary tests (5.391 seconds) and the separate receipt-repin check.
- `sh script/repin-use-case-receipts.sh --check` passed: all 18 receipts current.
  `cmd/corvint/main.go` is unchanged, so no repin was needed.
- An initial spec-index test rejected a sixth Agent digest field. The disclosure
  note was folded into the existing Status field before repeating the gates.
- The corrected spec-index, requirement definitions, traceability and decision
  number gates passed. Line-citation checking then identified the shifted
  answerability verdict citation in `core-compatibility-freeze-v1.md`; its line
  was relocated from 3417 to 3441 after verifying the cited source bytes were
  unchanged. The existing hash and accepted enum contract remain intact.

## Corvint use and retained limits

Base: `71b2eb8fc3e56a9df368b8be8227bf5ad125f8c9`. Installed Core:
`1.0.0-rc.3 (build 407)`; Tasks: `0.0.0-tcp01-unverified+build.407`, verification
NOT_RUN. GitHub's official release inventory reported Core `v1.0.0-rc.2` and Tasks
`tasks-dev-20260929.2` as the latest applicable release channels. The optional
updater's network-enabled checks failed with `process-after-start-failed:
descendant snapshot unavailable`; verified freshness remains UNKNOWN and no
installation or downgrade was attempted.

**Used:** pre-change `query --task ... --limit 1` and tracked-path `impact
internal/contextindex/taskcontext.go --limit 10`; original outputs are retained in
`/private/tmp/v1-0315-query.json` and `/private/tmp/v1-0315-impact.json`. Query
returned index-snapshot authority plus omissions/uncertainty, not complete task
coverage. `affected --base` was used before tests, retained in
`/private/tmp/v1-0315-affected.json`; it selected plausible units and retained
language frontiers and unowned-document uncertainty. Static advice to run the
full repository gate is superseded by the task's explicit focused-only boundary.
`make gate` and `go test ./...` are NOT_RUN.

**Unavailable:** index refresh and `make dogfood-change BASE=...` could not write
derived evidence under the external shared Git directory in this sandbox. The
original errors are retained in the task transcript and
`/private/tmp/v1-0315-dogfood-start.log`. Final immutable CEM/OCM binding, dogfood
checking, sealing and local outcome recording are NOT_PRODUCED: this task
explicitly prohibits commits. No success claim is made for that workflow.

The requested make targets use a scratch `mktemp` wrapper that supplies an
explicit writable template only for no-argument calls; macOS's default otherwise
chooses an unwritable system temporary directory despite TMPDIR. Gate logic is
unchanged. All Go checks use GOTOOLCHAIN=local, TMPDIR under the requested scratch
root and GOMAXPROCS=3. Native inherited model/effort handles this bounded change
and its independent review; no specialist escalation is needed. Account usage,
billed tokens and comparative savings are NOT_OBSERVED through available tools.

Owner acceptance, integration and native ticket completion remain outside this
local patch. No commit, push, decision file or `.taskman` write is authorized here.
Observed tooling friction remains unfiled because the task prohibits ticket-store
writes; it is retained here for the coordinating owner.
Scratch wrapper and overlay cleanup was rejected by automatic approval review
because `rm -f` style commands are disallowed. Those files remain under
`/private/tmp`; no source or repository state was removed.
