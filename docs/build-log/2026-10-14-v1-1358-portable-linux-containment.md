# 2026-10-14 V1-1358: portable Linux process containment in `interop/cem01-go`

Lane V1-1358 (BUG, P2). Requirements `CEM-V1-180..184` (`docs/specs/cem-stable-v1.md`) and
`TCB-V0-030..031` (`docs/specs/tasks-criterion-binding-v0.md`), accepted under the owner's standing
auto-accept of lane requirements (2026-10-11). Base `adf587fdb65f4d87dc94a5e374a4fda1f87c5e7f`
(origin/main after PR #861).

## Intent

`interop/cem01-go` qualified its owner/keeper process containment only on Darwin, so
`verify-stable-canonical` answered `UNSUPPORTED` (`unsupported-process-containment`) on Linux. The
experimental Tasks verdict gate (TCB-V0-016/017) treats a differing portable outcome as
`verifier-disagreement`, so it could grant only on Darwin. Reproduction: PR #863, CI run
38463640466, `go-product-shard (0)`, `TestTCBV0016GateVerdictRunsBothConsumers`: native ACCEPT
`14ac383d...`, portable UNSUPPORTED `5de4c58a...`. Commit d30f894fa skipped that test off Darwin
as an interim.

## Decisions

- One shared owner/keeper file. `stable_process_darwin.go` became `stable_process_unix.go`
  (`darwin || (linux && (amd64 || arm64))`), which holds the frames, keeper, run, close and
  retirement. Two per-OS files supply four seams: `stableOwnerSupported`,
  `stableGroupQuiet` (step 1 of retirement), and the keeper's owner watch with
  `awaitControl`/`awaitChild`. Darwin keeps kqueue `EVFILT_PROC` and signal 0. Behaviour on
  Darwin is unchanged: the step-1 loop was rewritten as `for !stableGroupQuiet(pgid)` over the
  same `kill(-pgid, 0) != nil` test.
- The Linux keeper uses pidfds (`pidfd_open`, syscall 434 on amd64 and arm64) in one
  close-on-exec epoll set. These watch the owner (its parent, re-checked after the open) and the
  unreaped Git child, as the Darwin kqueue does. Owner exit or any failure takes the existing
  fail-closed path: the keeper SIGKILLs its own group and exits.
- Linux step 1 reads `/proc` instead of signal 0 (V1-0668: Linux answers signal 0 successfully on
  a zombie-only group). With the keeper unreaped, the group is quiet only when the keeper is a
  zombie child of this owner leading the group, and one full listing shows every member of the
  group as a zombie with `Threads:` at most 1, includes the keeper, and the keeper still qualifies
  afterwards. The reads are bounded at 4,096 bytes for stat, 16,384 for status and 2^22 listing
  entries. Any failure means not quiet, which ends in HOLD. Step 2 (reap once, then signal-0
  ESRCH within the two-second post-reap cap) is unchanged. This proves quiet before the reap,
  which is stronger than the native `ReapAfterSuccessfulSignal` order.
- The route is explicitly bounded. It covers only Linux amd64/arm64 (the build constraint and the
  syscall number). The owner must not be PID 1, and pidfd, epoll and `/proc/self/stat` must probe
  successfully before admission. Every other OS and architecture keeps
  `stable_process_other.go`'s refusal.
- Licensing. The module is Apache-2.0 (`LICENSING.md`). The Linux code was written from the spec
  text, `REPOSITORY-ENVELOPE.md` and the public V1-0668 build-log design description. No code was
  copied from the AGPL native reaper.
- The TCB-V0-016 real-consumer test now runs on Darwin and Linux amd64/arm64. It still skips
  `NOT_RUN` on another Linux architecture. It logs both digests. The `internal/taskman` stand-in
  helper is unchanged, because those tests never read the interop module.
- Scope is cem/1.0 only. No byte under `protocol/cem-1.0` changed, and no closed code, result
  field, bound or native verifier semantic changed.

## Verification

Linux runs used colima's arm64 VM (kernel 6.8.0-117-generic, already running; left running).
The repository subset was streamed into the container by tar, because the VM does not share
`/private/tmp`. Tests ran as uid 1000 through `setpriv`.

```sh
tar --no-xattrs --no-mac-metadata --exclude=./.git -C clone -cf - . | docker run --rm -i --init \
  -e GOTOOLCHAIN=local -e GOFLAGS=-buildvcs=false golang:1.27.2 sh -c 'mkdir -p /src && \
  tar -C /src -xf - && cd /src && setpriv --reuid=1000 --regid=1000 --clear-groups env HOME=/tmp/h \
  GOCACHE=/tmp/gocache GOPATH=/tmp/gopath go test -race -count=1 -timeout 30m -v \
  -run TestTCBV0016GateVerdict ./cmd/corvint/'
```

- Core evidence. `TestTCBV0016GateVerdictRunsBothConsumers` passed on linux/arm64 (go1.27.2,
  git 2.47.3, `-race`) with native `verdictSha256`
  `14ac383d61741ab12c6a422d086701302b230295ff97babcb443aead1cb40476` and portable ACCEPT with the
  same `14ac383d61741ab12c6a422d086701302b230295ff97babcb443aead1cb40476`. The same test passed on
  darwin/arm64 (macOS 26.6.2, go1.27.2, git 2.54.0, `-race`) with both digests equal to that
  value. linux/amd64 is the CI `go-product-shard` run.
- `interop/cem01-go` on linux/arm64 (golang:1.27.2, uid 1000, `-race`, the command above with
  `interop/cem01-go protocol` and `go test ./...`) gave 371 PASS, 2 SKIP and 2 FAIL:
  - The skips are full-result cases 21 and 22, which run only under go1.24.13.
  - The failures are envelope cells `relative-sha1` and `relative-sha256`. Their fixture setup
    calls `git worktree add --relative-paths`, which git 2.47.3 in the golang image rejects
    (`unknown option 'relative-paths'`, needs git 2.48). This is an environment limit, not a
    verifier result. Both cells pass on Darwin with git 2.54.0.
  - Everything else passed. That covers all 54 applicable `TestStableCanonicalFullResults`
    cases, `TestStableCanonicalLedgers` (boundary-336/337), `TestCEMV1226TamperMatrixPortable`
    (20 cases), the five `TestStableLinux*` tests and every `TestStableLifecycle*` test. Those
    include the new hostile test, in which a TERM/HUP/INT-trapping shim forks a trapping
    grandchild that starts a great-grandchild, then execs Git, and both descendants are gone.
  - Six `process containment not proven` stderr lines come from the hold-seam tests, the same
    count as on Darwin.
- An earlier linux/arm64 go1.27.1 run had one more environmental failure when run as root: case
  23 `optional-attributes-EACCES`, because root bypasses `chmod 000`. It passes as uid 1000.
- `TestStableLinuxZombieOnlyGroupRetires` observes the V1-0668 defect directly. Signal 0 still
  succeeds on the zombie-only group, while the `/proc` proof marks it quiet and retirement
  completes with post-reap ESRCH.
- linux/amd64 locally: NOT_RUN. Under colima's qemu emulation the Go toolchain itself crashes
  (`marked free object in span`, and also without `-race` at `GOMAXPROCS=1`), before any test
  code runs. `go vet` for linux/amd64 passes. The CI `go-interop` and `go-product-shard` jobs on
  ubuntu-24.04 amd64 are the amd64 evidence.
- Darwin `interop/cem01-go` with `-race` (go1.27.2):
  - The full module run hit the 30-minute per-package timeout at load average about 58, with
    `TestStableCanonicalLedgers/boundary-337` still issuing Git operations (stack in `stableRun`,
    not in retirement). There were no failures before the timeout.
  - The tests that run had not reached (26 top-level, including Ledgers, the envelope cells and
    the tamper matrix) then passed alone in 544 s.
- `go vet` of `interop/cem01-go` passes for darwin/arm64, linux/amd64, linux/arm64, linux/386,
  linux/riscv64, windows/amd64 and freebsd/amd64. The last four build the refusal file. `go vet
  ./cmd/corvint/` passes. `gofmt -l` prints nothing.
- `corvint affected --base adf587fdb` selected 67 units, mostly path-literal readers of the test
  file. `tools/gate-ledger` timed out at Go's 600 s default under load. It passed alone with
  `-timeout 30m` in 62.6 s.

## Failure modes

- A kernel older than 5.3, a missing `/proc`, a seccomp policy that denies `pidfd_open`, or a PID
  1 owner refuses before admission.
- A surviving member, a zombie leader with live threads, or an unreadable `/proc` entry ends in
  HOLD (`unsupported-process-containment`).
- No member can fork after the group SIGKILL, because Linux refuses `fork` to a task with a
  pending fatal signal. Any process that joins the group later is still caught by step 2's
  signal-0 absence check.

## Rollback

Revert the change commit. The Linux files go away, `stable_process_unix.go` returns to
`stable_process_darwin.go`, Linux refuses again, and the TCB-V0-016 test skips off Darwin. No
packet, map, seal or result byte depends on the route.

## NOT_PRODUCED and limits

- The pass-0 dogfood run reported `prechange-query` and `prechange-impact` NOT_OBSERVED
  (`agent-receipt-absent`). Expected pass-0 rows: `cem-prepare git-diff-failed`, `cem-cite
  cem-map-not-produced`, `missing-intent-scope`, `map-unavailable` and
  `outcome-input-not-provided`.
- linux/amd64 local execution NOT_RUN (qemu toolchain crash); CI is the evidence.
- Linux architectures other than amd64/arm64, and other systems, stay refused by design.
- Escaped sessions, cross-device topology and external consumers are not claimed.
