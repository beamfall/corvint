# Bounded ps stderr (V1-0691)

## Intent, scope and baseline

Owning intent: `docs/specs/process-group-owner-v0.md`, PGO-V0-010,
**accepted, decision 0507**. Base:
`29e1f653486fca13216606683d3a5c2b05cacd36`. This is per-command stderr
capture evidence, separate from V1-0689 cumulative identity-cap evidence.
No decision file, ticket-store mutation, commit or publication is authorized.
The current session model/effort was retained for this narrow resource-boundary
repair; billed tokens, account usage and comparative savings are NOT_OBSERVED.

Earliest proof uses a finite helper emitting at most 256 KiB diagnostic bytes
through the real exec command path. The command seam changes no production ps
path, argv, environment or timeout. The pre-fix run accepted both 16,385 bytes
and 262,144 bytes stderr with a valid parsed identity; a nonzero command exit
reported only snapshot unavailability. This confirms there was no stderr
resource boundary, without attempting host memory exhaustion.
Raw red evidence: `/private/tmp/v10691-red.log`.

## Implementation and observed evidence

Stderr retains at most 16 KiB using the existing bounded capture with truncation;
excess bytes are drained without blocking the producer. Overflow invalidates the
entire snapshot, including when the command also exits unsuccessfully or is
cancelled. No diagnostic payload is exposed. Stdout retains its 4 MiB cap;
PID/start representation, structural ownership and retirement stay unchanged.
`exec.Cmd.Run` retains the 100 ms WaitDelay and joins output-copy goroutines
before capture results are inspected. Rollback reverts this isolated cap and
command seam, restoring the known stderr risk.

Focused race execution on Darwin passes the quiet, exact 16 KiB, limit-plus-one,
finite 256 KiB, overflow-plus-failure, ordinary failure and stdout overflow cases.
The 256 KiB case retained exactly 16,384 bytes and returned an overflow error.
Both cancellation cases acknowledge their stderr writes before cancellation;
the helper is reaped and the snapshot goroutine joined in approximately 1–29 ms.
The test helper emits finite output, creates no descendants, and every blocking
case has context cancellation and a deferred join even on fixture startup failure.
Raw green evidence: `/private/tmp/v10691-focused-race.log`; the final run with the combined-failure assertion also passes (`/private/tmp/v10691-focused-race-final.log`, 22.984 s).
These witnesses do not establish a real ps noisy-output failure, continuous
unbounded producer throughput, Linux runtime or escaped-child containment.

## Context, verification and retained limits

Corvint query and affected were used at the named base; raw context/affected
outputs are `/private/tmp/v10691-context.json`,
`/private/tmp/v10691-affected-baseline.json`, and
`/private/tmp/v10691-affected-red.json`. Affected selected procgroup along with
broad literal-reader consumers and retained language-frontier/dirty-doc unknowns;
static full-gate advice does not override the owner's explicit focused scope.
Index refresh refused repository metadata changed during observation. The
pre-change dogfood-change failed because the linked worktree Git directory is
outside writable roots (`/private/tmp/v10691-dogfood.log`). Post-commit CEM/OCM
binding, dogfood-check and seal are NOT_PRODUCED: commits are expressly forbidden.
Installed Core is 1.0.0-rc.3 build 407; Tasks is the unverified build 407 TCP-01
label. Official update checks for both failed their process smoke with descendant
snapshot unavailable; available-version freshness stays UNKNOWN. No update was
activated and no failed verification was bypassed.

The full focused package race run has existing descendant tests failing because
this sandbox refuses `/bin/ps` with operation not permitted; the direct command
failure is retained in `/private/tmp/v10691-ps-baseline.err`. This environmental
restriction does not affect the injected helper witnesses. Full package output
is `/private/tmp/v10691-race.log` (FAIL, 435.353 s): existing snapshot, lifecycle readiness and additional process deadline/cleanup assertions failed. The ps restriction is directly reproduced; load-related causation of the other failures is not established. Vet passed (`/private/tmp/v10691-vet.log`).
Linux runtime and unsandboxed production-ps qualification remain NOT_RUN.

Real Git staging is blocked by read-only linked metadata. Requirement generation
uses an isolated temporary index copied from the original index, with the amended
spec staged into temporary objects; the real index and object store are untouched.
The macOS default mktemp directory is also outside writable roots; a private
wrapper supplies `-p /private/tmp/claude-501/gotmp` for documentation checks.
The generated TSV must be restaged with the spec when integrating this patch.
Independent read-only native review found no blocking correctness findings. Its suggested combined-failure assertion was added and the focused race tests passed again. The reviewer confirmed unchanged ps arguments/environment, identity parsing, ownership and cleanup, while retaining unqualified Linux and inherited-pipe WaitDelay execution. All requested doc gates passed on the fully staged temporary snapshot; the separate receipt repin check passed with all 18 receipts current. The receipt fixture suite passed all five cases. Raw final output: `/private/tmp/v10691-doc-gates-staged.log`, `/private/tmp/v10691-doc-gates-staged.exit` (0), `/private/tmp/v10691-repin-staged.log`, `/private/tmp/v10691-repin-staged.exit` (0). Earlier unstaged-index notes are retained in `/private/tmp/v10691-doc-gates-final.log` and superseded by that final staged run.
No make gate or repository-wide Go test was run. Native ticket completion remains
unperformed by explicit instruction; V1-0691 is not claimed complete in Tasks.

Final uncommitted source Git blob identities (no final commit binding is claimed):

- `internal/procgroup/descendants_posix.go`: `4395a1d586f255dc14d7d3c5ce057aff2b8c3606`.
- `internal/procgroup/descendants_stderr_posix_test.go`: `e6c7ee3e454e7a9efad5ff0f90f7eabcb706e3e1`.
- `docs/specs/process-group-owner-v0.md`: `c077c06991806d7ae5d0fde76ed52172403e0d59`.

Verification commands (GOTOOLCHAIN=local, TMPDIR=/private/tmp/claude-501/gotmp, GOMAXPROCS=3):

- `go test -race -count=1 -timeout 30m -v ./internal/procgroup -run '^TestDescendantSnapshot(StderrBoundary|CancellationJoinsCopies)$'`: PASS.
- `go vet ./internal/procgroup`: PASS.
- `go test -race -count=1 -timeout 30m -v ./internal/procgroup`: FAIL as retained above.
- `make -s -k spec-requirements-check requirement-definitions-check traceability-tests-check decision-numbers-check line-citations-check error-code-ownership-check unbounded-readers-check use-case-receipts-check use-case-receipts-test`: PASS; the cache override used the permitted Go cache root.
- `sh script/repin-use-case-receipts.sh --check`: PASS.

All owned command sessions and the independent reviewer completed. The final source and tests are unchanged after their passing focused race run; only this evidence record was completed afterward. Real staging, commit-bound dogfood evidence, unsandboxed production ps, Linux runtime and owner acceptance remain outside the demonstrated outcome.

## Unsandboxed rerun (batch lane, 2026-10-10)

The sandbox `/bin/ps` restriction above does not apply outside the builder sandbox.
On Darwin arm64, Go 1.27.1, merged with origin/main, `go test -race -count=1
-timeout 12m ./internal/procgroup` passed in full (86 s), including the existing
descendant and lifecycle tests, so the earlier failures were environmental. The
cross-ticket packages `internal/testacceptance`, `internal/behaviorfalsify`,
`internal/jstestprovider` and `cmd/corvint-tests-accept` also passed under
`-race`. Linux runtime remains NOT_RUN.
