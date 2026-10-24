# Corvint Tasks lease-gate latency V0

Owner: Russell Lewis
Date: 2026-10-24
Intent status: proposed
Delivery status: experimental

Authoritative inputs: native tickets V1-2531, V1-2537, V1-2539 and V1-2203. These requirements
amend the write rounds of the [agent lease contract](corvint-tasks-agent-leases-v0.md), whose
Requirements section is at the 256-obligation OCM limit (`maxObligations`), so they live here.
They change no lease semantics, stored state, wire format or receipt.

## Agent digest
- Claim: Lease rounds skip the cleanup lock on a ready writer route, keep fdinfo files open and parse each canonical ticket once.
- Status: proposed; experimental; LGL-V0-001, 002 and 004 are implemented with focused tests; LGL-V0-003 is a measured target that is NOT MET (claim gate-wait p99 1.7 s against 500 ms).
- Exists: the writer-route staging decline, retained Linux fdinfo files, the single-parse ticket loader and the wire ASCII scan, each with a focused test.
- Blocked on: V1-2539 (claim wait) and V1-2203 (heartbeat median needs an owner decision on fsync coalescing).
- Read next: Requirements; Failure modes; Acceptance evidence.

## User and current state

Human-owned input: native tickets V1-2531, V1-2537, V1-2539 and V1-2203 (owner rule: performance
issues are 1.0 work, V1-0887 targets a heartbeat median of at most 50 ms and a p99 of at most
250 ms). On the WSL2 host at 84b08d351, `TestCALV0263_LeaseGateLoad` (40k receipts, 15 writers,
5 x 120 s) showed heartbeat and renew time outside the gate, in the round's guards, at p99
169-187 ms. The guards' orphan cleanup took the writer lock whenever `staging/` was non-empty, and
a live peer's commit keeps its slots there for its whole hold, so a ready writer-route write
queued on the lock for that hold before it reached the gate, then queued again inside the route.
A per-syscall count of one heartbeat at 40k receipts put about 85 % of the journal-write CPU in
the session's pinned-parent checks, each re-observing about eleven mounts by opening, reading and
closing `/proc/self/fdinfo/<fd>` (decision 0411).

## Requirements

- `LGL-V0-001`: (proposed; V1-2531) A lease write round MUST clear orphan stages under a cleanup
  lock before the preparation gate only when its writer-checkpoint route is not ready. When the
  route is ready the round MUST enter it with no cleanup lock; the route, holding the gate and the
  writer lock, MUST decline `staging` when the staging directory holds any entry (under the lock
  every entry is an orphan, because only the lock holder stages), writing nothing, and the round
  then clears orphans under a lock of its own exactly as before and continues to the resume or
  complete route. What a round commits is unchanged.
- `LGL-V0-002`: (proposed; V1-2203) On Linux a session MUST open each pinned descriptor's procfs
  `fdinfo` file once, on the descriptor's first mount observation, retain it for the session's
  life, and re-read it from offset 0 on every later observation (procfs regenerates the content
  on each read from offset 0), closing every retained file when the session closes its handles.
  Every observation still re-reads the mount ID decision 0411 requires; only the open and close
  per observation are removed.
- `LGL-V0-003`: (proposed; V1-2539) The claim latency target under `TestCALV0263_LeaseGateLoad`
  at 40k receipts and 15 writers on the WSL2 quiet host is a claim gate-wait p99 of at most
  500 ms (at most four claim holds of the measured 125 ms p99 queued ahead) and a claim wall p99 of
  at most 1500 ms. Every claim run reports both against this target, labelled with its platform
  and host load; a run that misses it is reported NOT MET, never rounded. The harness starts all
  15 writers' claims together, so claim gate wait is first-in-first-out serialization: the k-th
  claim waits for about k earlier claim holds plus the liveness holds interleaved with them, and
  a 500 ms p99 over 14 queued claims needs a claim hold near 36 ms, against a measured fsync and
  commit floor near 70 ms. The target therefore cannot be met by trimming in-gate CPU alone.
- `LGL-V0-004`: (proposed; V1-2531) Lease-gate validation MUST NOT parse a canonical ticket twice:
  `loadTickets` relies on `ticket.Decode`, whose `wire.Parse` already refuses any bytes that differ
  from the encoding of the parsed value plus one trailing LF, and drops the second parse and
  re-encode it performed. The wire hostile-code-point check, and the string parser before it
  formats a refusal position, MUST first scan the string's bytes and accept without decoding runes
  when every byte is below 0x80 and none is a control hostileRune refuses (TAB, LF and CR only
  where prose permits them); any other string takes the existing rune-by-rune check. Which bytes
  are accepted and which refusal, code and position are reported are unchanged.

## Non-goals

no fsync is coalesced or removed (decision 0549 F2 stays: staging-directory sync
coalescing needs its own decision and crash-matrix proof); decision 0411's per-check mount-ID
observation is kept; no round count, lock wait or gate order changes; no stored state, wire format
or receipt changes.

## Failure modes

with LGL-V0-001 a round that finds staging under the route's lock pays one more
lock acquisition (the cleanup lock) than before, only when a stage is present; a write killed
between the route's decline and the cleanup leaves the orphan for the next writer's cleanup, as
before. With LGL-V0-002 a retained fdinfo file whose read fails, or that reports a different
mount, fails the observation exactly as a fresh open would; a descriptor number reused after a
close cannot alias a retained file, because the retained file is keyed by the session's own
pinned `*os.File` and closed with it. LGL-V0-004 is safe only while `wire.Parse` keeps its
re-encode comparison and trailing-LF check; a change that relaxes either must restore the
`canonical` call in `loadTickets`, and `TestLGLV0004_NoncanonicalTicketBytesStillRefused` then
fails first. The ASCII scan accepts nothing the rune check refuses, because every byte below 0x80
is a whole rune and the scan refuses every C0 control the rune check refuses.

## Acceptance evidence

`TestLGLV0001_ReadyRouteTakesNoCleanupLockOutsideGate`
(`internal/tasks/store/lease_orphan_route_test.go`) holds the writer lock 600 ms over an orphan
slot: the heartbeat spends under half that hold in its guards and at least half waiting inside the
route, declines `staging`, clears the orphan, commits once, replays the same request and serves
the next heartbeat from the route; without the change it fails on the guards time.
`TestLGLV0002_RetainedFDInfoObservesCurrentState`
(`internal/tasks/authority/fixture_session_linux_test.go`) seeks a pinned file to three offsets and
sees each retained re-read report the current `pos` and equal a fresh observation, with one
retained file per descriptor and none after the handles close. `TestLGLV0004_ASCIIFastPathAgreesWithHostileRune` (`internal/tasks/wire/plain_ascii_test.go`)
checks every ASCII byte in both modes, alone and inside a string, against `hostileRune`, keeps
non-ASCII strings on the rune check, and keeps a parsed hostile string's position and code point
in its refusal; `TestLGLV0004_NoncanonicalTicketBytesStillRefused`
(`internal/tasks/transaction/canonical_ticket_test.go`) refuses a canonical ticket with a missing
or extra trailing LF, insignificant whitespace or an escape form as malformed and accepts the
canonical bytes. LGL-V0-003 and the LGL-V0-004 hold reduction are evidenced by the
before/after load and hold-profile reports in the build-log entry `docs/build-log/2026-10-24-tasks-lease-latency.md`.

## Rollback

revert the source; the guards again take the cleanup lock whenever staging is present
and each observation opens its fdinfo file afresh; `loadTickets` again re-parses each decoded
ticket and every hostile check decodes runes.
