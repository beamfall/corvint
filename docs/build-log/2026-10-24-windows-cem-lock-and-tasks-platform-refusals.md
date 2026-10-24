# Windows: CEM update lock and Tasks refuse up front, tests skip on the exact refusal

Date: 2026-10-24. Tickets: V1-2522 (CEM-PILOT-033), V1-2523 (CTS-V0-025). Scope: the accepted
release spec `docs/specs/corvint-1.0-product-and-release-v1.md` lists windows/amd64 `UNSUPPORTED`
and decision 0061 defers Windows; V1-2485 (the Windows 1.0 scope decision) is still open. Both
changes are reversible by revert if V1-2485 widens the scope.

## V1-2522: root cause

`publish.Root.Lock` (`internal/cem/publish/lock.go`) created the lock's parent directory, then
called `openLockFile`. On every GOOS but darwin and linux, `lock_other.go` returns
`serialized CEM map updates are unsupported on this platform`, but `Lock` discarded that error and
returned only `publish-failed: update lock for ".corvint/change.cem.json" cannot be opened`. The
deliberate platform refusal was already listed in `testplatform.PlatformRefusals`, so the lost
cause is why `conformance/ocm-v0`, `internal/cem/workflow`, `internal/frontiernextrepo` and
`internal/lrfrepo` failed on Windows instead of skipping.

## Decision

- `Lock` asks `LockRefusal()` first. It is nil on darwin and linux. Elsewhere `Lock` refuses before
  `MkdirAll`, keeping the existing `publish-failed` code and adding the platform refusal as the
  message's last clause. No new cemcode: `publish-failed` is the documented code for this lock, and
  the cause is what the helper and an operator need.
- Tasks gets `wire.PlatformRefusal()` with the existing closed code `UNSUPPORTED` and the text
  `Corvint Tasks is unsupported on this platform`. `cli.Run` returns it after help and before the
  read-only check and any store access; `version` still answers. PathText is not widened.
- Tests skip through `testplatform.SkipWindowsRefusal` on the production refusal value only. The
  lrfrepo, frontiernextrepo and ocm-v0 sites pass the real `Prepare` error. `internal/cem/workflow`
  has more than a hundred `Prepare` sites, so its shared `openSession` helper passes
  `publish.LockRefusal()`, the value `Lock` wraps. `fixture.TempRepo` passes `wire.PlatformRefusal()`,
  so every Tasks package using it skips, including mutation, snapshot and store, which the source
  Windows run did not name.

## Limits

- No native Windows run was made. `make windows-vet` type-checks the Windows-only tests
  (`TestLockRefusesUpFrontWithoutAdvisoryLocks`, `TestCTSV0025_UnsupportedPlatformRefusesUpFront`);
  the host rerun is `NEEDS-HOST`.
- A Windows test that opens a workflow session without taking the lock now skips too, and a Tasks
  CLI test that does not use `TempRepo` but runs a non-help command now meets the refusal. Both
  leave Windows paths unqualified, as the release spec already does.
- Taking the lock on Windows was not attempted: `LockFileEx` locks byte ranges, there is no
  `O_NOFOLLOW` open, and an open file cannot be removed without share-delete, so `Lock`'s
  path-identity release would need a new design; `PublishSecure` (`secure_other.go`) refuses
  Windows anyway.
