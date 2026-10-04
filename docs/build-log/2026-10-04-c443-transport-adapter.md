# Issue 443 transport-adapted held recovery seam

Human-owned intent: GitHub issue443, native V1-0611 and V1-0688. The owner's 2026-10-04
transport-adapter decision admits scoped implementation and testing of the transport-adapted lane
(`ALO-V0-023..026`). Its qualification is `TRANSPORT_ADAPTED_HISTORICAL`; it never satisfies a
pristine historical-binary requirement, and nothing here claims recovery success.

## Product change

`dogfood finish --aggregate-outcome-profile corvint-dogfood-aggregate-outcome/0
--transport-adapted-recovery <request>` is a recovery-only route over the existing pending
aggregate transaction. `internal/localcompletion/transport_recovery.go` parses one closed
canonical request, then admits it against the compiled-in #443 row (key `b36b7ba8...`, plan
`3c6bab79...`, BASE `406f9dc3`/tree `e0dfeaa5`, HELD `a79439af`/tree `d6d6b890`, adapter patch
digests `f926ecc6...` and `c02e535f...`). Admission compares the plan base tree and the current
HELD target and tree before any producer runs. BASE and TREE executables are hashed at admission,
at check start, around every run and at publication. They must equal the request and differ from
the running binary. The strict check runs those two executables with no override, and the report
records exactly their identities. The ordinary Finish, ordinary aggregate Finish and public check
paths are unchanged, and the public check still requires COMMITTED. Provenance (qualification,
request digest and both identities) goes to stdout and is not stored in local-completion state.

## Native evidence (Darwin arm64, go1.27.1)

- `TestTransportAdaptedRecoveryNative` passed (rc=0, 374.66s). It covers a real legacy failure
  and ten refusals, each with byte-identical evidence: missing profile flag, non-canonical bytes,
  unknown qualification, same path, enrollment, not-admitted, BASE and HELD mismatch, fixed
  running binary, and identity drift. It also covers a disagreeing TREE (`final-check-failed`,
  stays pending), the public check refusing, a successful recovery that records A/B with no
  override, a second attempt refused `not-pending`, and an unchanged ordinary aggregate Finish.
- `TestTransportRecoveryRequestClosedCanonical` and `TestTransportRecoveryAdmissionsClosed`
  passed. The `internal/localcompletion` and `internal/dogfoodflow` packages passed in full
  (rc=0), and go vet passed on the touched packages (rc=0).

## Historical adapter witnesses (scratch harness, not committed)

- Builds: each source was exported from its exact revision and only its three-file patch was
  applied. Every other blob matched `ls-tree`. Binary sha256 values: BASE pristine `bc8f886d...`,
  BASE adapted `dfb10ec3...`, HELD pristine `9729fb94...`, HELD adapted `e28c476c...`. The proposed
  patch files are not gofmt-clean; the builds used their exact bytes.
- Parity: `cem status` (max-unknown 0) and `dogfood-ocm status` against the held input exited 0
  with byte-identical stdout (sha256 `d69ae0a5...`/105402 bytes and `655d2d10...`/6833 bytes) on
  three routes: the adapted worker route of both binaries, and the public route of all four.
  Pristine binaries refuse the worker route with `invalid-arguments`.
- Refusals: adapted workers refuse relative roots, extra arguments, the wrong map, unlisted forms
  and empty argv (`invalid contained verifier invocation`), and an unowned start (`startup
  ownership unverified`).
- Cleanup: **FAILED for cancellation and timeout.** For both adapted binaries, a `git` shim in
  front of PATH showed the first Git child (`cat-file --batch`) running in its own process group.
  It and its sleeping descendant survived the owned launcher's group reap, orphaned to init. The
  cause is `internal/cem/gitrun/session.go`, which still calls `containChild` and
  `killGroupThenReap` unconditionally (BASE line 128, HELD line 131). The decision's whitelist
  guards only `gitrun.go`. Stdout and stderr overflow left no survivors. The four orphaned groups
  were retired by group plus exact command match and re-checked absent.

## Limits and NOT_PRODUCED

The held recovery attempt is NOT_PRODUCED. It needs a corrected adapter, whose whitelist would
add `gitrun/session.go` and give new patch digests, plus independent review. Linux and Windows
lanes are NOT_PRODUCED. Binary-to-source provenance is attested by this build witness and is not
verified in process. Durable request identity is not stored.
