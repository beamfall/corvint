# Linux tracing service slice (V1-5106)

Date: 2026-11-07
Intent: decision 0592; RSP-V0-020, RSP-V0-021, RSP-V0-022, RSP-V0-024.
Lane base: `f42df32d12bae40c3bfcc6bb8a48442966e4a71d` (accepted spec).
Affected-selection base requested by the owner: `8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734`.

## Delivered boundary

`internal/proctrace` is an isolated, native-Go service with plain observation types;
it imports no test-evidence store and no command imports it. The Linux/amd64 backend
starts a traced child, installs a seccomp-BPF filter at its initial exec stop, restores
its instruction/register/stack bytes, and follows descendant ptrace events on one
locked OS thread. Known non-observational calls bypass stops; unknown calls trap and
make every component incomplete. Cancellation kills and reaps the observed tree.
No executable, CLI, wire format, frozen output or release-manifest entry changes.

Paths retain primary/sibling repository identities, lexical/resolved names and
intermediate symlink dependencies. Outside paths leave only class counts. Exclusive
creates and their renamed replacements suppress later own-read edges; append,
positioned overwrite and truncate preserve prior-content dependencies. Components
remain incomplete for unknown calls, malformed arguments, unsupported matching,
privileged executables, policy refusal and configured caps. Unknown network/descriptor
peers conservatively prevent exclusion. Exec digests are SHA-256 observations, not
Git blob identities; a later adapter owns immutable evidence binding.

Implementation choice within the accepted fail-closed contract: the first backend
supports the native amd64 ABI. Other architectures and operating systems report
`absent`. Shared-memory and concurrent filesystem pathname races are not proven
atomic, so descendant observations are retained but components are incomplete.
Casefold/unknown filesystems widen instead of fabricating case-sensitive matches.
Scripts with implicit interpreter semantics also widen. These limits grant no runner
qualification and no production narrowing. Revert this isolated package and its
traceability/read-scope entries to roll back; no data migration is needed.

The parity helper retains runner identity, native exit/signal outcomes and both
wall times. Mismatch or unobserved outcome changes components to absent. A caller
must persist/admit those observations and compare native per-test outcomes before
runner qualification; no caller is wired in this slice.

## Measured evidence

Linux syscall fixtures cover openat2, statx, faccessat2, getdents64, execveat,
renameat2 and connect; fork/vfork/clone/exec descendants; setuid mode; unknown and
malformed calls; event/path caps; own-write provenance; sibling and symlink reads;
complete single-process collection; cancellation; and import isolation. The full
new package was run through `script/heavy.sh go test -count=1 -timeout 30m -v
./internal/proctrace/` without a test filter. This is focused package evidence,
not qualification of the complete Corvint Go suite.

One local run on a shared Linux host (nanoseconds, no performance-budget claim):

| Fixture runner | Untraced | Traced | Overhead |
|---|---:|---:|---:|
| native-fixture/child | 977809 | 3471459 | 2493650 |
| native-fixture/fork | 816735 | 4223534 | 3406799 |
| native-fixture/vfork | 1196903 | 7798134 | 6601231 |
| native-fixture/clone | 781976 | 4120744 | 3338768 |
| native-fixture/exec | 978424 | 5533122 | 4554698 |
| native-fixture/signal | 655416 | 3407910 | 2752494 |
| go-runtime-fixture | 2057497 | 9438473 | 7380976 |

Raw test log stays outside the repository at
`/home/russell/lanes/rspt4-evidence/tests-final.log`.
SHA-256: `a84444f15a4713f67956bed81dae4e7e3627c9cb7a4665d609dae7020945c7e2`.

## Failed evaluation and limits

The first combined syscall fixture timed out in a blocking loopback connect after
10 seconds. The fixture now uses a nonblocking socket, retaining the connection
attempt assertion; the backend conservatively records failed attempts too. The
failed log remains outside the repository (`tests-2.log`), not discarded as a pass.

Full Corvint-suite traced/untraced parity, production runner overhead, non-amd64
runtime evidence and independent review are NOT_RUN here: the owner reserves broad
verification and independent review to the host orchestrator and forbids whole-suite
runs in this lane. Native task completion, publication and sealing are not authorized.
V1-5106 therefore remains pending host qualification and completion; focused fixture
parity does not establish whole-suite parity. Retrieval/learning frozen evaluations
are not applicable: this package is unreachable from those paths. Billed-token,
quiet-host and external-outcome telemetry are NOT_OBSERVED.
