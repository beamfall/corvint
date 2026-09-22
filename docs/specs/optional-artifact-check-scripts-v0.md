# Optional artifact check scripts V0

Owner: Russell Lewis
Date: 2026-09-08
Requirement prefix: `OACS-V0`
Intent status: accepted (decision 0105 second amendment 2026-09-12)
Delivery status: experimental
Authoritative inputs: `../../AGENTS.md` invariant 8, `../SPEC-DRIVEN-DEVELOPMENT.md`,
`analyzer-python-native-candidate.md`, `go-production-kernel-migration-v0.md`, and
`release-artifact-integrity-v0.md`. The owner instruction recorded under "Gate tooling is in
scope for invariant 8" in `../BUILD-LOG.md` requires specification of gate tooling; it does not
accept this source-derived backfill.

## Agent digest
- Claim: Optional scripts check Python candidate build/size limits and loose-binary reproducibility, without canonical-gate membership or promotion.
- Status: accepted (decision 0105 second amendment 2026-09-12)/experimental
- Exists: `script/check-analyzer-python-offline-build.sh`, `script/check-analyzer-python-ratchets.sh`, and `script/check-release-artifact-reproducibility.sh`.
- Blocked on: nothing for acceptance. Decision 0105 withheld it on `OACS-V0-002`; its second amendment accepted the spec after the `ccdd39da` repair and a clause-by-clause re-review. Promotion past experimental still needs the remaining `NOT_RUN` cells in Acceptance and traceability. Stub-driven shell tests exist per script (`make analyzer-python-offline-build-test`, `analyzer-python-ratchets-test`, `release-artifact-reproducibility-test`), none in `make gate`.
- Read next: Verified current state; Requirements; Trust boundary and known gaps; Acceptance and traceability.

## User and job

A Corvint maintainer needs to know what an optional check's exit status establishes before using
it as evidence. This proposal gives the three existing shell entrypoints an explicit contract for
their inputs, refusals, exemptions, and evidence. Candidate semantics remain with `PNC-001..010`;
release qualification remains with `GPK-V0-018`, `GPK-V0-019`, and the release-integrity contract.
The measurable job is to distinguish each bounded check from an unrun canonical or promotion gate.

## Verified current state

Source inspected at `2f843df329afb52ebef9516721ead77bec2cf491`:

| Entrypoint | Existing check | Evidence boundary |
|---|---|---|
| `script/check-analyzer-python-offline-build.sh ROOT` | Build the candidate with fresh module/build caches and disabled module proxy/checksum service; inspect Core dependencies and cache files. | Exit status; no durable report or retained binary. |
| `script/check-analyzer-python-ratchets.sh ROOT BINARY` | Check three exact regular, non-symlink files against byte ceilings. | Exit status; no report, build, or binary provenance verification. |
| `script/check-release-artifact-reproducibility.sh` | Select output and invoke the default Go reproducibility command. | Delegated report and summary; no archive subcommand. |

These are source observations, not claims of a successful run. None of these scripts is invoked
by `Makefile` or `.github/workflows/ci.yml` at that revision. `make gate` includes the separate
`script/go-archive-gate`; its execution cannot be reported as execution of this release wrapper.
The Python candidate spec already mentions both Python scripts informally. This proposal owns
only their shell-level checks, not a second candidate or archive specification.

## Requirements

- `OACS-V0-001`: The offline-build entrypoint MUST require a nonempty first argument naming a
  directory and MUST refuse when `ROOT/vendor` satisfies shell `test -e`. It MUST create separate
  temporary module and build caches and a temporary binary under `${TMPDIR:-/tmp}`, change to
  `ROOT`, and request `go build -o BINARY ./cmd/corvint-analyzer-python` followed by
  `go list -deps ./cmd/corvint`. Both invocations MUST set `GOMODCACHE`, `GOCACHE`, `GOPROXY=off`,
  `GOSUMDB=off`, `GOWORK=off`, and `GOFLAGS=-mod=mod`; failure of either command MUST fail the check.
- `OACS-V0-002`: The offline-build entrypoint MUST refuse an exact dependency-list line equal to
  `github.com/corvint-context/corvint/internal/analyzerpython`. It MUST refuse any regular file
  reported below the temporary module cache's `cache/download` directory whose path does not
  contain `/github.com/corvint-context/corvint/@v/`. That main-module path is the sole explicit cache
  exemption; absence or suppressed inspection errors can yield an empty list and MUST NOT be
  represented as exhaustive proof of module-cache integrity or network isolation.
- `OACS-V0-003`: After all three temporary allocations succeed, the offline-build entrypoint
  MUST install cleanup for `EXIT`, `HUP`, `INT`, and `TERM` that removes those caches and binary.
  The `HUP`, `INT`, and `TERM` handlers MUST exit 129, 130, and 143 rather than return, so a
  signalled run never resumes to a later check or a passing exit (decision 0248).
  It MUST NOT claim a durable receipt, verified descendant cleanup, or cleanup of an allocation
  made before a later allocation fails: the current script supplies none of those guarantees.
- `OACS-V0-004`: The ratchet entrypoint MUST require nonempty `ROOT` and `BINARY` arguments,
  a directory at `ROOT`, and regular non-symlink final entries at
  `ROOT/internal/analyzerpython/analyzer.go`, `ROOT/cmd/corvint-analyzer-python/main.go`, and `BINARY`.
  It MUST measure each with `wc -c`, refuse values above 65,536, 4,096, and 6,291,456 bytes
  respectively, and admit equality. These ceilings have no file-size exemption.
- `OACS-V0-005`: Ratchet evidence MUST be limited to those three files' byte counts and final-entry
  checks. The supplied binary is caller input: a successful check MUST NOT establish stripping,
  executability, target architecture, correspondence to the source, total package source size,
  performance, allocation cost, or improvement over a previous revision.
- `OACS-V0-006`: The release wrapper MUST derive `ROOT` from its script directory and select
  `${CORVINT_RELEASE_ARTIFACT_OUTPUT:-/tmp/corvint-release-artifact}` as output. Before creating it,
  a literal output equal to `ROOT` or matching `ROOT/*` MUST print
  `check-release-artifact-reproducibility: output must be outside ROOT` to stderr, substituting
  the actual root, and exit 2. Otherwise it MUST run `mkdir -p` and replace the shell with
  `GOTOOLCHAIN=local go run ./conformance/release-artifact-v0 --root ROOT --manifest
  ROOT/conformance/release-artifact-v0/manifest.json --output OUTPUT --report OUTPUT/report.json`.
- `OACS-V0-007`: The release wrapper MUST preserve the delegated command's observable output and
  exit result. Its invocation selects the default loose-binary checker, never the `archive`
  subcommand. Evidence consumers MUST retain the delegated `NOT_RUN` smoke states and
  `pendingEvidence`; a successful same-profile build comparison MUST NOT establish full release
  qualification, cross-builder reproducibility, archive verification, or native cross-target execution.
- `OACS-V0-008`: These optional scripts MUST NOT be described as executed by `make gate` while
  no invocation exists in that target's execution graph. This backfill MUST NOT add gate membership,
  change existing checks, accept a candidate profile, or authorize signing, tagging, upload,
  publication, installation, or promotion.

## Non-goals and simpler baseline

The baseline is direct invocation of the existing checks with recorded arguments and exit status.
No shared gate framework, new runner, new profile, expanded source-size accounting, or extra build
is proposed. The documents do not strengthen a weak source check by describing a stronger one.
The exact accepted toolchain and release contracts continue to outrank this backfill.

## Trust boundary and known gaps

The shell, utilities, Go executable, working tree, environment, and temporary-directory parent
are trusted inputs; these scripts are not hostile-process sandboxes. Their `set -eu` failures do
not have a stable named diagnostic taxonomy, except for the release wrapper's explicit refusal.
Neither Python script rejects surplus positional arguments.

The offline script does not itself set or verify `GOTOOLCHAIN=local` or an exact Go version.
`GPK-V0-018` still requires both for evidence commands; callers must supply and verify the accepted
toolchain, and an unqualified invocation is not compliant evidence. It does not directly verify
absence of `go.sum` or module declarations. Its `test -e` vendor check permits a dangling symlink;
dependency output is exact-line matching, not package-prefix matching. Its cache pipeline suppresses
errors and assumes trusted `find`/`grep`. Disabling Go module services is not an OS network sandbox.
The cleanup trap is installed after allocation and contains no process-group cancellation or
reaping logic. Interruption leaving no descendants has not been established.

The ratchet script follows ancestor directories and checks only the final entries for symlinks.
There is no immutable snapshot or race protection between the checks and reads. An empty regular
file can satisfy the binary ceiling; the argument's description as "stripped" is a precondition
for the caller to establish, not a property the checker measures.

The release wrapper does not change the caller's working directory before its relative `go run`;
invoke it from the repository root. Its initial output check is lexical, not a canonical-path or
symlink check, and `mkdir -p` precedes Go preflight. The delegated `checkOutputLocation` normalizes
absolute paths but also does not resolve symlink aliases. Existing output directories are admitted
by the wrapper. It supplies no shell cleanup trap, atomic publication guarantee, private-directory
ownership check, or time/disk budget. The stricter archive command is a separate contract and cannot
be used to imply those properties here. No actual build or long-running command was launched for
this documentation backfill.

## Acceptance and traceability

Acceptance requires owner review of these proposed checks and explicit disposition of the gaps;
source inspection cannot accept intent. The following matrix is evidence to collect, not a claim
that a test mention means a passing execution. Shell syntax checks cannot establish behavior.

| Requirement | Implementation / existing witness | Deterministic acceptance evidence still needed |
|---|---|---|
| OACS-V0-001 | Offline script argument checks, temporary allocation, and two Go invocations | `script/check-analyzer-python-offline-build_test.sh` (stub `go`) cases 1-3: missing root and vendor presence refused before any `go` call, both invocations carry the pinned environment, a failing build fails. Case 6 (added 2026-09-12) refuses a root that is not a directory. Case 7 (added 2026-09-12) fails a stubbed `go list -deps` and confirms cleanup still ran. Real fresh-cache success ran directly against this checkout: `PATH=/opt/homebrew/Cellar/go/1.27.0/bin:$PATH GOTOOLCHAIN=local script/check-analyzer-python-offline-build.sh "$(pwd)"`, exit 0, 2026-09-12. |
| OACS-V0-002 | Offline script dependency and cache filters | Same test cases 4-5: an exact `internal/analyzerpython` dependency line exits non-zero with a `Core build depends on` message; foreign-cache rejection and main-module exemption pass. Missing-cache characterization is already exercised by case 2 (no `STUB_CACHE` set, so no `cache/download` entries are created there, and the check still passes). Unreadable-cache characterization stays `NOT_RUN`: the wrapper allocates and removes its own module cache internally with no hook for a caller-supplied path, so a test cannot make that directory unreadable before the check reads it without adding one to the production script, which is out of scope here. |
| OACS-V0-003 | Offline script `cleanup` and `trap` | Same test cases 2-3: caches and binary removed after a passing and a failing run. Case 8 (added 2026-09-13) stubs `mktemp` to fail the first cache allocation and confirms the check exits before any `go` call with nothing to clean up. Case 9 (added 2026-09-13) backgrounds the wrapper with a stubbed `go` that blocks on `build`, sends it and the blocked stub `TERM`, and confirms the trap still removes all three allocations. Case 10 (added 2026-09-13) covers the `INT` half of `trap ... EXIT HUP INT TERM`: under `set -m` it sends `SIGINT` to the backgrounded wrapper's process group once the blocking `go` stub has started, and confirms a non-zero exit, the stub gone, and the temporary caches and binary removed. Case 11 (added 2026-09-13, decision 0248) sends `TERM` to the wrapper alone while its `go list` stub still succeeds and requires exit 143 with nothing left behind; the returning trap let the wrapper exit 0. Ran: `sh script/check-analyzer-python-offline-build_test.sh`, exit 0, 2026-09-13. |
| OACS-V0-004 | Ratchet script file guards and three `wc -c` limits | `script/check-analyzer-python-ratchets_test.sh` cases 1-4: missing binary argument, equality passing and one byte over failing for each ceiling, symlinked and missing final entries refused. Case 6 (added 2026-09-12) refuses a directory in place of the binary as the non-regular, non-symlink case. |
| OACS-V0-005 | Ratchet script's closed file inventory | Same test case 5: an empty regular binary passes, characterizing the evidence limit. |
| OACS-V0-006 | Release wrapper output selection, case refusal, and `exec` | `script/check-release-artifact-reproducibility_test.sh` (stub `go`) cases 1-3 and 5: root and child refused with exit 2 before `go` runs, a prefix-sharing sibling admitted, exact delegated arguments under `GOTOOLCHAIN=local`, caller-CWD characterization. Case 6 (added 2026-09-12) fails a `mkdir -p` blocked by an existing file at the output path and confirms `go` never ran. Default and empty-override output stay `NOT_RUN`: both fall through `${CORVINT_RELEASE_ARTIFACT_OUTPUT:-default}` to the literal `/tmp/corvint-release-artifact`, so exercising either would create that directory on the shared host outside this fixture (this file's own header comment already excludes the default case for that reason; an empty override hits the same path). |
| OACS-V0-007 | `conformance/release-artifact-v0/main.go`; `TestOutputInsideRepositoryIsRefused` and `TestSummaryCountsNotRunSmokeSeparately` in `conformance/release-artifact-v0/gate_test.go` | Existing tests cover delegated behavior, not shell invocation; wrapper stdout and exit-status propagation are cases 3-4 of `script/check-release-artifact-reproducibility_test.sh`. An end-to-end reproducibility run stays `NOT_RUN`: it needs ten full cross-compiled `cmd/corvint` builds (five targets, twice each), and AGENTS.md records a single such build at roughly 591 s on a quiet host that "panics under any load"; running it on this shared, loaded host risks a hang for one evidence cell and is deferred to a dedicated run instead. |
| OACS-V0-008 | `Makefile`, `.github/workflows/ci.yml`, and the three unchanged scripts | Recheck invocation references when changing gate membership; current source inspection establishes absence, not a passing gate. |

## Rollout, rollback, and maintenance

Rollout adds this proposed ownership document, its index/requirement locators, and one stub-driven
shell test per script with an opt-in `make` target outside `make gate`. Rollback removes those;
it does not delete or alter any script. Keep the
owning-spec backlog open until the owner accepts or otherwise disposes of the proposal. A future
behavior change must update the owning clause and add appropriate boundary evidence in the same
change. Do not silently promote the Python candidate or reinterpret a previous artifact receipt.

## Unresolved decisions and promotion

Decision 0105's second amendment accepts the eight requirements as written, with the gaps above
disposed of as disclosed limits. Gate membership, stronger toolchain enforcement, path containment,
and failure cleanup are separate future decisions; none is implemented or accepted here, and
acceptance does not decide them. The shell tests characterize current behavior. Promotion to implemented requires evidence for every accepted in-scope requirement, with
unrun and failed cases visible. A reproducibility pass cannot close unrelated release obligations.
