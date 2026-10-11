# Tasks store bounds and fatal read failures: V1-0702, V1-0704, V1-0705

Date: 2026-10-10
Base: `2145af619f68698dbb1c097369e29d589e623f7d`
Scope: uncommitted builder handoff; no publication or native ticket-store writes.

## Contract and decisions

The owner requested prove-first repair of these three defects. Existing accepted intent governs:
CAL-V0-033 already limits combined pool-command capture to 64 KiB and requires cancellation joining
and owned-group retirement; executable gates retain `wire.MaxGateOutputBytes` (16 MiB).
Intent's frozen SPEC §1 byte limits, implemented by `intent.BoundFor` and `wire/limits.go`, apply
under archive `intent/` names without changing those archive identities. CAL-V0-026's guarded
audit/cache route permits retry of changed observations, not retry past fatal audit or native-close
failure. `journal.PhysicalObservation.Cleanup` explicitly carries native closure independently.
These repairs add no numbered requirements and change no wire or accepted intent. The specs remain
unchanged. Pool sweep retains its separate `snapshot.MaxPoolSweepOutput` limit.

- V1-0702: configure the shared capture writer with a pool-specific limit; its zero/default limit
  remains the gate limit. Keep the existing overflow kill, terminal `OUTPUT_LIMIT`, cleanliness
  proof and caller refusal semantics.
- V1-0704: translate only the leading `intent/` namespace before intent-bound lookup. Preserve
  returned paths/digests/sizes and evidence/other fallback bounds, for disk and borrowed metadata.
  Correct the existing inventory test's explicitly assumed evidence bound to the true queue bound.
- V1-0705: use the observed audit directly in the pre-read, keeping cleanup separate through the
  retry decision. A guard check may replace only nil or `SNAPSHOT_MOVED` ordinary audit errors;
  audit cleanup and guard-close failures always return. Use an operation-local audit seam rather
  than global hooks. The old `leaseAudit` wrapper synthesized an empty observation and discarded
  the third channel; the pre-read no longer calls it. Journal's native close path publishes an
  `UNSUPPORTED_FILESYSTEM` ordinary error as well as the independent cleanup channel.

Rollback reverts this batch's narrow code/test hunks; no stored data or format migration is involved.
The concurrent V1-0906/V1-0913 journal/store work is outside this lane.

## Prove-first evidence

Commands used `GOTOOLCHAIN=local TMPDIR=/private/tmp/claude-501/gotmp GOMAXPROCS=3 go test -race
-count=1 ./internal/tasks/store/ -run FILTER`. Each defect failed before its fix.

| Ticket | Witness and observed pre-fix output |
| --- | --- |
| V1-0702 | `TestPoolOutputBound/65537`: `bytes=65537 class=EXIT_ZERO want=OUTPUT_LIMIT clean=true digest matches capped output=false`. Exact 65,536 bytes passed. |
| V1-0704 | `TestInventoryArchiveBounds`: all five intent paths used `effective bound=67108864`; expected queue 1048576, policy 262144, import map 8388608, ticket 131072, release 262144. Overflow metadata and real reads were accepted: `present=true err=<nil>`. Exact bounds and evidence/other bounds passed. |
| V1-0705 | `TestLeaseReadCanonical` passed first. Fatal injection: audit `calls=2 proofReturned=true err=<nil>`; cleanup alone `calls=1 proofReturned=true err=<nil>`; cleanup with movement and snapshot with cleanup both `calls=2 proofReturned=true err=<nil>`. Injection followed a successful canonical native audit; tests assert it was reached. |

Original local outputs: `/tmp/b-store-bounds-before.log`, `/tmp/b-store-canonical-before.log`,
`/tmp/b-store-lease-before.log`. The final tests explicitly assert guard movement, preserve fatal
sentinels without retry, and separately verify a clean snapshot-only retry.

## Verification and limits

Focused race verification and the existing inventory parity/fallback, cleanup, audit-cache reuse,
pool cancellation and descendant-retirement witnesses are recorded in
`/tmp/b-store-focused-after.log`. Final new-test verification, including overflow with a live child,
is recorded in `/tmp/b-store-final-tests.log` (passed, 17.195 seconds).
`go vet ./internal/tasks/...` passed (`/tmp/b-store-vet.log`).

Required doc gates were invoked; no-argument macOS `mktemp` selected an unwritable system temp
directory even with TMPDIR set. Retrying uses a scratch PATH shim that supplies an explicit authorized
template only for no-argument `mktemp`; all other arguments pass through unchanged. No repository
gate was edited. Results: `/tmp/b-store-doc-gates.log`; receipt repin check:
`/tmp/b-store-repin.log`. No spec was edited, so regeneration is not required.

Independent native reviewer: no blocking findings in the bounded source/test review or the subsequent
bounded review of the movement and child-retirement assertions. It confirmed pool/gate cap separation,
archive identity preservation, and fatal cleanup returning before retry. Its nonblocking fixture-cleanup
suggestion was applied: a failed cleanliness assertion still retires the overflow fixture's owned group.

The doc gates all passed, as did `sh script/repin-use-case-receipts.sh --check` (all 18 current).
`spec-requirements-check` passed the generated-index comparison and `internal/specindex` tests;
traceability reported three existing planned tests. `git diff --check` passed.

The initial post-fix focused race run failed only the existing `TestPoolProcessDescendants` cases:
success `UNKNOWN clean=false`, timeout `TIMEOUT clean=false`, interruption `INTERRUPTED clean=false`.
All selected inventory tests (including the 5,000-receipt parity/fallback fixture), fatal inventory
cleanup, audit-cache reuse, new byte-bound and lease-read witnesses passed in that run. No existing
real fixture exceeded its true intent byte bound.

To distinguish regression from environment, an explicit Go overlay substituted the immutable base
versions of `pool_process_unix.go` and `lease_gate.go` without altering working-tree files. The baseline
race run of `TestPoolProcessDescendants` also failed: success and timeout `descendants did not start`,
interruption `INTERRUPTED clean=false` (`/tmp/b-store-process-baseline.log`). The sandbox independently
denies process-table `ps` reads. Complete existing descendant/cancellation cleanliness qualification
therefore remains unresolved here. The new overflow-child witness has also observed `OUTPUT_LIMIT
clean=false cappedDigest=true`; it keeps the cleanliness assertion and retains this earlier
qualification failure. It passed on the final race run, including the group-liveness assertion,
but that single pass does not erase the earlier host-dependent failure. The overflow output class
and capped digest are correct, and uncertain cleanup still refuses.
The boundary fixture was simplified from child-spawning `head`/`tail` to single-process `cat` after
the initial fixture also hit intermittent unproved child cleanup. No assertion was weakened.

Final race filter: `TestPoolOutputBound|TestLeaseRead|TestInventoryArchiveBounds|TestCALV0026_(VerifiedAuditReuse|AuditInventoryFatalCleanup)`.
All new tests and these related witnesses passed; the separately run larger inventory parity witness
passed too. No race report was emitted. Cancellation/descendant qualification in the existing fixture
remains a host-bound limitation, not claimed passed. The fault test uses the same Darwin/Linux build
constraint as its canonical observation helper.
After that build-constraint change, the focused `TestLeaseRead` race rerun passed
(`/tmp/b-store-final-lease-tag.log`), and final `go vet ./internal/tasks/...` passed again.

Corvint context query and affected advice were used (local outputs `/tmp/b-store-context.json` and
`/tmp/b-store-affected.json`); omitted ranked/test evidence remains uncertainty. `make gate` and
repository-wide `./...` are explicitly excluded by the owner; affected's generic gate advice does
not override that scope. Index refresh failed because its shared Git-directory snapshot destination
is outside this worktree's write permission. Post-commit CEM binding/seal and native ticket completion
are deferred under the explicit no-commit/no-store instruction, not claimed satisfied.

Installed Core: `1.0.0-rc.3` build 407; Tasks: `0.0.0-tcp01-unverified+build.407`.
Official component checks failed `version smoke failed: process-after-start-failed: descendant
snapshot unavailable`; available versions/freshness remain UNKNOWN. No verification bypass or
installation was attempted. New defect filing is deferred because this lane must not write `.taskman`.
No account usage tool was exposed; billing telemetry is NOT_OBSERVED. No savings claim is made.
