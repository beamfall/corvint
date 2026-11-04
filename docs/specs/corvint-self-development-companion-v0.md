# Corvint Self-Development Companion V0

Owner: Russell Lewis
Date: 2026-11-04
Requirement prefix: `CSD-V0`
Intent status: accepted
Delivery status: experimental
Authoritative inputs: owner's pk-ci lane charter; V1-4359, V1-4553, V1-4575 and V1-3264;
`corvint-self-development-v0.md`, whose Requirements section has 256 obligations.

## Agent digest
- Claim: Local gate fixtures and batch verification expose platform drift, skipped inputs and unresolved historical evidence.
- Status: accepted / experimental
- Exists: `script/lane-scripts_test.sh`, `script/check-bash32.sh`, `script/dogfood-cache_test.sh`, `script/batch-precheck.sh`, `docs/AGENT-ROUTES.md`.
- Blocked on: hosted/macOS qualification and access to the private history archive; these remain NOT_OBSERVED.
- Read next: Requirements; Acceptance evidence; Limits and rollback.

## User and measurable job

The maintainer needs failures before integration, including on macOS hosts whose temp paths are
aliases and whose process probes cost more than Linux probes. Linux regressions emulate these
conditions. They do not establish a passing macOS nightly. History must remain distinguishable
from current-main acceptance evidence. No hosted run, private archive access or native ticket
completion is implied by local structural evidence.

## Requirements

- `CSD-V0-7510`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4359) The sweep regression MUST canonicalize its scratch root before constructing process/file probe paths, and MUST exercise a symlinked TMPDIR on Linux as well as macOS.
- `CSD-V0-7511`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4553) Bash portability discovery MUST probe arbitrary tracked files in the C byte locale, so a binary PNG cannot abort BSD awk's script enumeration under a UTF-8 caller locale.
- `CSD-V0-7512`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4553) Every dogfood-cache fixture assertion failure MUST identify its line and retain the captured command output. A forced-failure self-test MUST prove this diagnostic. Missing inputs MUST refuse before optional closeout capacity checks or builds. Copying the citation checker into a scratch fixture MUST supply its two minimal inventories without replacing existing fixture inputs. Fake-compiler fixtures MUST use private admission paths and avoid host-load waits.
- `CSD-V0-7513`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4553) The shared-resource gate fixture MUST fit the existing 128-byte argv contract independently of ordinary host scratch prefixes, without raising the product bound.
- `CSD-V0-7514`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4575) Batch verification MUST run the catalog generator's shell fixture, and catalog/document changes MUST select the lane shell fixtures. Catalog assertions MUST use fixtures or compare the relevant entries without depending on unrelated flag order/prefixes.
- `CSD-V0-7515`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3264) AGENT-ROUTES MUST describe the issue-body, archive-file and pre-snapshot commit routes. An unavailable archive MUST remain an explicit access blocker, never an invented evidence summary. Current closeout MUST cite a current-main test or path when historical commits are outside current history.
- `CSD-V0-7516`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4553) Adopter advisory fixtures MUST canonicalize scratch roots before asserting argv equality and MUST emulate a temp creator retaining a doubled separator.
- `CSD-V0-7517`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4553) Companion cleanup's TERM/KILL grace periods MUST count process-probe cost against elapsed deadlines, rather than multiplying a fixed probe count by a sleep. Each TERM/KILL grace MUST allow at least two seconds (a `SECONDS + 3` deadline compensates for whole-second truncation). Slow-probe regression MUST retain the bounded join and descendant cleanup assertions.
- `CSD-V0-7518`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4553) The refused-address helper MUST use a bound non-listening socket on Linux to avoid WSL forwarding, and a closed loopback listener outside Linux, where a bound non-listening Darwin socket can time out. The regression MUST reject timeout and successful connection alike.

## Acceptance evidence

| Requirement | Implementation and regression |
| --- | --- |
| `CSD-V0-7510` | `script/lane-scripts_test.sh` sweep group with nested symlink-TMPDIR case. |
| `CSD-V0-7511` | `script/check-bash32.sh`, `script/check-bash32_test.sh` binary PNG/UTF-8 fixture. |
| `CSD-V0-7512` | `script/dogfood-cache_test.sh` forced diagnostic and missing-input fixtures; `Makefile` dogfood-closeout. |
| `CSD-V0-7513` | `TestLGLV0885_GateRunSerializesASharedResource` in `internal/tasks/store/gate_resource_unix_test.go`. |
| `CSD-V0-7514` | `script/batch-precheck.sh`, fixture-based `script/capability-catalog_test.sh`; `TestCINV07505RepositoryCIRoutes`. |
| `CSD-V0-7515` | `docs/AGENT-ROUTES.md` historical evidence route. Archive access returned HTTP 404 on 2026-11-04; original issue title/body and other unrecovered texts remain BLOCKED until access is granted; the pk-ci build log retains the locally recoverable B3 values and issue-to-commit mapping. |
| `CSD-V0-7516` | `script/adopter-ci-advisory_test.sh` doubled-separator stand-in and full recursive advisory fixture. |
| `CSD-V0-7517` | `script/corvint-companion-release-gate`, its shell test's delayed ps probe with a TERM-ignoring child. |
| `CSD-V0-7518` | `internal/testsupport/refusedaddr_unix.go`, `refusedaddr_other.go`; `TestASDV0919RefusedLoopbackAddrRefusesAtOnce`. |

## Limits and rollback

No product authority, learning, package boundary, gate waiver, task-store write or publication is
added. A diagnostic says which local inputs were skipped and cannot attest their correctness.
Archive commands require operator credentials and network; their output is historical evidence,
not accepted current intent. No replacement task store or invented archive mapping is permitted.
The late-bound process probe can itself stall on a broken host; a local pass does not establish
scheduler bounds for all hosts. Native macOS and the next hosted nightly remain required external
qualification. Rollback reverts these fixture/selection changes and their requirements together;
it preserves failed evidence and restores the prior explicit limitations.
