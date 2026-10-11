# V1-1129: retryable Tasks descriptor exhaustion

Human-owned intent: V1-1129 / [issue 769](https://github.com/beamfall/corvint/issues/769).
Three well-formed pool acquires returned `ERROR`, `MALFORMED`, `retryable:false` during
host descriptor pressure, causing callers to stop instead of retrying.

Contract: CAL-V0-215 and TCP-00 amendment A28 in
`docs/specs/corvint-tasks-agent-leases-v0.md`, **proposed, pending owner acceptance (V1-1129)**.
Delivery is experimental. The base is `97c3a4bd03bb97b29e7378dfa116ffde2280cadb`.
No decision file, commit, push or native ticket-store write is part of this builder task.

## Decisions and failure boundaries

- `wire.CodeOf` recognizes EMFILE/ENFILE by `errors.Is`, including PathError, SyscallError
  and wrapped errors, and emits the registered `RESOURCE_EXHAUSTED`. Explicit wire codes
  retain precedence; unrelated untyped failures still map to MALFORMED. Existing typed
  UNSUPPORTED_FILESYSTEM errors that discarded errno are not reclassified by their text.
- The retry table records bounded-backoff, identical-request/same-request-ID advice.
  `WithoutRetry` now preserves a raw exhaustion error and its errno identity through a
  private wrapper. Explicit error and report overrides still win in envelopes and batches.
- The CLI audit covered init, program, mutate_batch and envelope paths. Program and batch
  mutation already use the shared classifier. Hard-coded I/O MALFORMED sites in run_batch
  and init, and codec error sites in escalation, now use it. Read/fallback envelopes retain
  retry overrides. Run-batch and qualify forbid retry after creating their request directory,
  since the identical request cannot resume it, including an OpenRoot failure after mkdir.
- Receipt publication precedes post files and head. Ordinary journal mutations can recover
  and replay an identical request through redo and the request index without double application.
  Independent review found initial pool preparation, sweep ownership and workflow DISPATCH
  returned receipt-plus-error before setting their non-retryable mark. Those marks now cover
  the partial publication path. Pool regressions inject both errnos after receipt publication,
  assert the mark and check that pool replays do not execute the program. The DISPATCH
  regression distinguishes pre-receipt evidence from the post-receipt attempt write, verifies
  the exact receipt bytes on disk before post-commit injection, and checks the returned error
  and that no reviewer ran.
- Review also found receipt link completion can fail after link but before the caller records
  the receipt name. `authority.Session.Link` conservatively marks that error non-retryable
  using its existing performed-effect wrapper. Its regression reaches pre-link and post-link
  failures independently and checks the actual destination bytes. This can suppress a safe
  ordinary-mutation retry; it avoids suggesting a multi-step replay that cannot finish.
- RLIMIT_NOFILE startup changes are a non-goal: raising the process soft limit does not solve
  system-wide ENFILE and changes limits inherited by child programs. No dependency or global
  resource-limit adjustment was added. Live host saturation is NOT_RUN.

## Numbering and code ownership

Read `origin/main:docs/specs/corvint-tasks-agent-leases-v0.md` (max CAL-V0-211, A27), listed
all 16 open PRs with `gh pr list --repo beamfall/corvint --state open --limit 50`, and inspected
each diff's added CAL-V0 IDs and amendment lines. PR #750 claims CAL-V0-212; #766 claims
CAL-V0-214. No checked PR claims CAL-V0-215 or A28; both were selected before implementation.
The checked PRs were #768, #767, #766, #765, #764, #763, #762, #761, #760, #759, #758,
#757, #752, #751, #750 and #749. No network fallback numbering was needed.

The owning spec and external-agent guide now name RESOURCE_EXHAUSTED. The wire's closed-code
count is 79, and the exhaustive retry classification test includes the new code. Searches of
protocol/, conformance/ and Tasks testdata found no separate Tasks JSON/schema code enumeration.
TCP-00's original §11 contract lives in the separate Tasks repository; A28 is its in-tree amendment.

## Verification and retained limits

Commands used GOTOOLCHAIN=local, TMPDIR=/private/tmp/claude-501/gotmp, GOMAXPROCS=3.

Earlier builder observations (retained from the staged log):

- Pre-change wire race baseline: PASS (4.422s).
- `go test -count=1 -race ./internal/tasks/wire/`: PASS (2.495s).
- CLI race filter `TestCALV0215_|TestCALV0078_|TestCALV0204_|TestTOLV0029_|TestTOLV0030_|TestTOLV0032_QualifyVerdicts|TestIssue502_EscalationCLIWritesAndReads`:
  PASS (61.407s). The first regression run refused fixture initialization because a ticket
  predated init; corrected setup creates the ticket through the CLI after init, then proves
  audit and claim before injection. Both faults leave store/journal unchanged; the identical
  acquire succeeds after the injected pressure clears.
- Authority race filter `TestCALV0215_|TestTMV0009_AS10_FixtureJoinedFailures`: PASS (1.630s).

Resumed investigation and coordinator evidence:

- Coordinator-reported outside-sandbox CALV0215 run (not independently observed here): pool
  store cases, CLI, wire and authority passed. DISPATCH failed for EMFILE and ENFILE with
  `fired=2`, `forbidden=false`. The old hook rejected evidence POSTs, which `applyWithFaults`
  publishes before the receipt; `stage` then called `noExec`, whose separate program write
  hit the same hook again. This did not exercise a durable DISPATCH. The repaired test targets
  the attempt POST and proves receipt durability before injecting; it retains a separate
  pre-receipt case and disarms before cleanup. No production change was needed in this repair.
- Resumed local baseline: `go test -count=1 -timeout 30m ./internal/tasks/store
  -run '^TestCALV0215_CommittedDispatchExhaustion$'` FAILED (42.14s test duration), before
  injection: implementer fixture returned SURVIVORS / BLOCKED_RECOVERY in this sandbox.
  This local limitation does not invalidate the coordinator's outside-sandbox run above.
- Repaired DISPATCH race regression: `go test -count=1 -race -timeout 30m
  ./internal/tasks/store -run '^TestCALV0215_CommittedDispatchExhaustion$'` FAILED
  (99.936s). All four cases stopped at implementer fixture initialization, before injection,
  with SURVIVORS / BLOCKED_RECOVERY. The repaired fault boundary remains unobserved locally.
  Expected from source:
  pre-receipt evidence failure remains retryable; after the receipt is durable, `step` records
  `departed` from `Report.Receipt`, and `RunRole` applies `WithoutRetry` to the returned errno.
  No repaired outside-sandbox pass is claimed.
- Earlier builder `go vet ./internal/tasks/wire ./internal/tasks/cli ./internal/tasks/store ./internal/tasks/authority`:
  PASS. Final CLI repetition with the same filter after authority changes: PASS (36.949s).
- Resumed `go vet ./internal/tasks/store`, gofmt and `git diff --check`: PASS.
- Independent native review: commit-boundary findings above repaired; bounded re-review found
  no remaining safety findings. A fresh read-only review of this repaired DISPATCH test also
  found no blocking findings. Neither review independently executed the repaired tests.
- The coordinator regenerated and staged `docs/specs/REQUIREMENTS.tsv`; readback includes
  CAL-V0-215. Regeneration is complete. Resumed `script/gen-spec-requirements.sh` output
  byte-compared with `docs/specs/REQUIREMENTS.tsv`: PASS (explicit scratch path instead of
  the Make target's denied default mktemp directory). No generated file was changed here.
- Earlier builder documentation targets were run together using make -k -s. The Make wrapper
  `spec-requirements-check` FAILED because default mktemp used a sandbox-denied directory.
  `requirement-definitions-check`, `traceability-tests-check`,
  `decision-numbers-check`, `line-citations-check`, `error-code-ownership-check`,
  `unbounded-readers-check`, `use-case-receipts-check` and `use-case-receipts-test` exited
  successfully. Index-based gates saw the original index, not this patch; line-citations-check
  explicitly reported unstaged specs excluded. This is not final documentation qualification.
  Use-case checks reported 18 current receipts and five passing test cases.
- `sh script/repin-use-case-receipts.sh --check`: PASS, all 18 receipts current.
  cmd/corvint/main.go is unchanged. Full CLI suite, ./... and make gate were NOT_RUN as requested.

## Corvint use and environment

Used the narrow query and affected routes, retaining receipts under `/private/tmp/v1-1129-*`.
Query succeeded with four ranked results omitted and six test-path candidates withheld.
Affected advised package tests while retaining graph/scope uncertainty; the user's focused-only
instruction governs instead of its generic make-gate advice. Index refresh was attempted once
and failed writing the common Git directory outside the sandbox. `make dogfood-change` likewise
failed creating its worktree Git evidence directory. Post-commit CEM binding/seal is NOT_PRODUCED:
this task explicitly prohibits commits, and Git metadata writes are sandbox-denied.
No ranking/learning/format change requires additional frozen retrieval evaluations.

Installed Core is 1.0.0-rc.3 build 407; Tasks is 0.0.0-tcp01-unverified+build.407.
Official release listing showed Core v1.0.0-rc.2 and Tasks tasks-dev-20260929.2 as newest
applicable published channels. Both corvint-update component checks failed their version smoke
with descendant snapshot unavailable; freshness remains UNKNOWN and no downgrade/update was
attempted. Account usage telemetry was unavailable. No cost/credit or savings claim is made.
The parent model was retained for this bounded builder/reviewer task; live effort control and
billed token telemetry were NOT_OBSERVED.

Rollback: revert the code, tests and proposed amendment as a unit; preserve existing retry
safeguards and all store/journal bytes. No state migration is required. Owner acceptance, final
index-bound documentation qualification, the repaired outside-sandbox DISPATCH regression result
and parent integration remain open. The coordinator has regenerated requirements
and supplied the outside-sandbox test results recorded above.

## Outside-sandbox results (lane driver)

These supersede the sandbox-limited entries above.

- `go test -count=1 -race -timeout 30m -run 'CALV0215|CALV0078'` over `internal/tasks/{store,cli,authority,wire}`: PASS, including
  `TestCALV0215_CommittedDispatchExhaustion` for EMFILE and ENFILE.
- `go test -count=1 -timeout 30m -run 'Pool|Workflow|Qualify|RunBatch|Batch|Lease|Escalation|Retry|Sweep|Init|Emit'` over
  `internal/tasks/{store,cli,authority}`: PASS.
- `make -s spec-requirements` regenerated `REQUIREMENTS.tsv`; the listed doc gates and
  `script/repin-use-case-receipts.sh --check`: PASS. The spec delivery header, Agent digest, README row and
  `INDEX.json` carry the same V1-1129 status suffix.

## Follow-up independent-review repairs

The next independent review returned FAIL with two P2 findings. This repair is an
uncommitted diff over `37c6cd449c8d2e46fc8f66e670b4779250ac80a1`; the earlier lane-driver
passes above do not qualify these new paths. CAL-V0-215 remains **proposed, pending owner
acceptance (V1-1129)**. No commit, push, `.taskman` write, decision file or RLIMIT change.

- P2-1: `healthClaimWith` now applies the same receipt-present `Unretryable` safeguard as
  explicit `PoolCommand`, before returning a preparation error. Automatic health for both
  claim and pool acquire is covered. `TestCALV0215_AutomaticHealthExhaustion` exercises both
  CLI verbs and both errnos, checking the encoded caller-visible retry flag. Its post-receipt
  fault targets `pools.json` and proves exact receipt bytes are already on disk; the
  pre-receipt control faults publication of the receipt and proves it is absent. The test
  first initializes/audits the fixture and decodes a real CLI envelope, asserts the injected
  path was reached, and separately checks the health program never ran.
- P2-2: `readRunConfig` wraps only open/read failures with `%w`, leaving JSON decoding and
  validation on `configErr`. `TestCALV0215_RunConfigResourceExhausted` uses a bounded open/read
  seam for EMFILE and ENFILE, tests both run-batch and qualify envelopes, asserts errno
  identity and reader closure, and compares the whole fixture filesystem with no results
  directory created. The existing closed-config regression remains the parse-error control.
- The additional sweep found `archive.verifyLimit`'s deferred source-error wrapper also
  overwrote reader errno with MALFORMED on a read-only path. It now preserves the cause with
  `%w`; framing/parse errors and the size-limit override keep their existing classification.
  `TestCALV0215_ArchiveSourceExhausted` first proves a valid exported archive, injects each
  errno at the initial read, header and final trailer, asserts the fault and original errno,
  checks unchanged fixture bytes and retains a truncated-archive MALFORMED control.

Deliberately unchanged sweep matches:

- `archive/verify.go`'s local tar/header/body/trailer error wrappers: the shared deferred
  source-error handler now supersedes every non-EOF source I/O failure, including errors
  hidden by `tar` or `io.ReadFull`. Genuine truncation and framing errors stay MALFORMED.
- `archive/cost.go:measureHeaderBytes`: tar writes to an in-memory counting writer, so the
  error is header validation, not OS I/O.
- `service/lifecycle.go:readProfile` and `facts`, `ticket/record.go`, `wire/reader.go`,
  `mutation` timestamp/date wrappers, `snapshot` handoff validation and ticket obligation
  decode/fold errors: all inspect already-read values; file-read errors return separately.
- `cli/run_batch.go:readJobReport`: its MALFORMED branch specifically means `errNotRegular`.
  Its OS read/open refusals are explicitly MISSING_EVIDENCE and occur after job creation;
  they are outside this pre-write MALFORMED sweep and were not reclassified.

Verification for this repair (GOTOOLCHAIN=local, TMPDIR=/private/tmp/claude-501/gotmp,
GOMAXPROCS=3):

- Existing CLI CALV0215 and archive bytes-plus-source-error baseline: PASS.
- New config and archive regressions failed before the fixes with lost errno / MALFORMED,
  as intended. Initial test compilation mistakes were corrected before collecting these
  behavioral failures; they were not treated as fault-path evidence.
- Focused CLI race run: PASS (24.443s), including CALV0215, CALV0078, closed configuration,
  run-batch and qualification filters. Exact combined command is retained below.
- `go test -count=1 -race -timeout 30m ./internal/tasks/store
  -run '^TestCALV0215_AutomaticHealthExhaustion$'`: FAIL (12.373s package duration).
  All eight cases initialized and reached automatic health preparation, but `/bin/ps`
  was sandbox-denied before artifact publication (`armed=true`, `fired=0`). Neither the
  pre-receipt nor post-receipt injection is locally verified. The orchestrator must run this
  test outside the sandbox; no skip or passing claim masks the failure.
- Focused archive race run: PASS (44.572s), including the new source-exhaustion regression,
  bytes-plus-source-error, terminal I/O, framing/size bounds and existing archive controls.
- Focused wire race run: PASS (2.167s), including classification and retry overrides.
- `go vet ./internal/tasks/cli ./internal/tasks/store ./internal/tasks/archive`: PASS.
- gofmt readback and `git diff --check`: PASS. The spec changes only clarify CAL-V0-215
  boundaries and witnesses; its definition line and requirement IDs are unchanged.
  No full gate or unstaged-index documentation qualification is claimed.
- Independent read-only review of the production diff, regressions and sweep: PASS, no
  confirmed correctness findings; reviewer did not run tests. It verified receipt marking
  precedes the error return and the CLI honors it, config parsing stays distinct from I/O,
  and the archive source handler retains the existing size-limit precedence.

The focused race command:

```sh
go test -count=1 -race -timeout 30m ./internal/tasks/cli ./internal/tasks/archive ./internal/tasks/wire \
  -run 'TestCALV0215_|TestCALV0078_|TestTOLV0028_RunConfigIsClosed|TestTOLV0029_|TestTOLV0030_|TestTOLV0032_QualifyVerdicts|TestTMV0022_AS09_|TestTMV0022_AS10_'
```

Current Corvint query and affected evidence is retained in `/tmp/v1-1129-review-*.json`.
Query retained four omitted results and 51 withheld test-path candidates. Affected retained
scope/language-frontier unknowns; the owner's focused-test instruction governs. Index refresh
was attempted once and denied a write to the shared Git directory. Core remains
1.0.0-rc.3 build 407 and Tasks 0.0.0-tcp01-unverified+build.407; both official updater checks
failed the version smoke (`descendant snapshot unavailable`), so available-version freshness
remains UNKNOWN. No update or workaround was attempted. Usage telemetry was unavailable.
The parent model was retained for the bounded repair; the independent reviewer used
GPT-6.1 Sol/medium for the known fault boundaries. Billing and live effort telemetry remain
NOT_OBSERVED. No new native queue entries were written under the task's explicit restriction.
Post-commit CEM binding/seal remains NOT_PRODUCED under the no-commit instruction.
