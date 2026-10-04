# Issue 443 transport-adapted held recovery seam

Human-owned intent: GitHub issue443, native V1-0611 and V1-0688. The owner's 2026-10-04
transport-adapter decision admits scoped implementation and testing of the transport-adapted lane
(`ALO-V0-023..026`). Its qualification is `TRANSPORT_ADAPTED_HISTORICAL`; it never satisfies a
pristine historical-binary requirement, and nothing here claims recovery success. On 2026-10-04
the owner approved widening the adapter whitelist to `internal/cem/gitrun/session.go` ("Widen to
session.go") after the first cleanup witness failed. The decision packet itself is unchanged;
this entry records the widening.

## Product change

`dogfood finish --aggregate-outcome-profile corvint-dogfood-aggregate-outcome/0
--transport-adapted-recovery <request>` is a recovery-only route over the existing pending
aggregate transaction. `internal/localcompletion/transport_recovery.go` parses one closed
canonical request, then admits it against the compiled-in #443 row (key `b36b7ba8...`, plan
`3c6bab79...`, BASE `406f9dc3`/tree `e0dfeaa5`, HELD `a79439af`/tree `d6d6b890`, adapter patch
digests `c610b712...` and `6f58ea18...` for the owner-widened adapter, which replaced the
first pins `f926ecc6...` and `c02e535f...`). Admission compares the plan base tree and the current
HELD target and tree before any producer runs. BASE and TREE executables are hashed at admission,
at check start, around every run and at publication. They must equal the request and differ from
the running binary. The strict check runs those two executables with no override, and the report
records exactly their identities. The ordinary Finish, ordinary aggregate Finish and public check
paths are unchanged, and the public check still requires COMMITTED. Provenance (qualification,
request digest and both identities) goes to stdout and is not stored in local-completion state.

## Native evidence (Darwin arm64, go1.27.1)

- `TestTransportAdaptedRecoveryNative` passed (rc=0, 374.66s; rerun after the re-pin, rc=0,
  258.88s). It covers a real legacy failure
  and ten refusals, each with byte-identical evidence: missing profile flag, non-canonical bytes,
  unknown qualification, same path, enrollment, not-admitted, BASE and HELD mismatch, fixed
  running binary, and identity drift. It also covers a disagreeing TREE (`final-check-failed`,
  stays pending), the public check refusing, a successful recovery that records A/B with no
  override, a second attempt refused `not-pending`, and an unchanged ordinary aggregate Finish.
- `TestTransportRecoveryRequestClosedCanonical` and `TestTransportRecoveryAdmissionsClosed`
  passed. The `internal/localcompletion` and `internal/dogfoodflow` packages passed in full
  (rc=0), and go vet passed on the touched packages (rc=0).

## Historical adapter witnesses (scratch harness, not committed)

- Widened adapter: each role's patch changes `cmd/corvint/main.go`, `internal/cem/gitrun/gitrun.go`
  and `internal/cem/gitrun/session.go`, and adds `cmd/corvint/recovery443_verifier.go`. In
  `session.go`, `containChild` and the group reap of the long-lived `cat-file` session now run only
  when `gitstatus.OwnedWorker()` is false; an owned worker waits with `command.Wait()`.
  `killGroupThenReap` is defined in `gitrun.go` and was already guarded. The patches are
  gofmt-clean. Compared with the decision's proposed patches, gofmt changed whitespace, sorted
  one import and expanded one single-line `if`, none of which changes semantics. Patch sha256:
  BASE `c610b712846b044b2c2f2050a08dfe81803a0c90ff909ca1193629316f0dd9b5`, HELD
  `6f58ea184aa98edbc7919f1f0ec23259d869517ac992184c9650d82d9f6e0c0c`.
- Builds: each source was exported from its exact revision and only its patch was applied. Every
  other blob matched `ls-tree`. Binary sha256 values: BASE pristine `bc8f886d...`, BASE adapted
  `0a68e6fe...`, HELD pristine `9729fb94...`, HELD adapted `e308360e...`. The pristine binaries
  reproduced their earlier hashes. HELD's stable-budget `groupreap.StartWith` sites are not
  patched; the witness below shows every Git child in the worker's group.
- Parity: `cem status` (max-unknown 0) and `dogfood-ocm status` against the held input exited 0
  with byte-identical stdout (sha256 `d69ae0a5...`/105402 bytes and `655d2d10...`/6833 bytes) on
  three routes: the adapted worker route of both binaries, and the public route of all four.
  Pristine binaries refuse the worker route with `invalid-arguments`.
- Refusals: adapted workers refuse relative roots, extra arguments, the wrong map, unlisted forms
  and empty argv (`invalid contained verifier invocation`), and an unowned start (`startup
  ownership unverified`).
- Cleanup: **no survivors** on both adapted binaries. A `git` shim in front of PATH logged each
  Git child's process group, and trigger files made it leave a `sleep 600` descendant. Every
  Git child ran in the worker's own group. Cancellation (8.0s) and the aggregate operation
  deadline (8.0s) exited -1, and stdout overflow (335 Git children) failed
  `aggregate-worker-invalid`; a forced Git failure and a failed read with a background descendant
  each exited 1. In every mode, no shim or `sleep` process survived.
- Earlier failure, superseded: with the three-file patches, the first Git child (`cat-file
  --batch`) ran in its own group under cancellation and timeout. It and its descendant survived,
  orphaned to init, because `session.go` still called `containChild` and
  `killGroupThenReap` unconditionally. Those four orphaned groups were retired by group plus exact
  command match and re-checked absent. That entry's claim that stderr overflow left no survivors
  was wrong: no mode ever wrote stderr.

## Limits and NOT_PRODUCED

The held recovery attempt is NOT_PRODUCED. It still needs independent review of the widened
adapter and the owner's go. Stderr overflow is NOT_PRODUCED: the worker wrote no stderr in any
mode, including the forced Git failure, so only stdout overflow was exercised. Linux and Windows
lanes are NOT_PRODUCED. Binary-to-source provenance is attested by this build witness and is not
verified in process. Durable request identity is not stored.
