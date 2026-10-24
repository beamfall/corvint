# Shared test-platform helpers for native Windows test runs

Date: 2026-10-23. Tickets: V1-2490 (this entry), V1-2486 (follow-up section). Source run:
`docs/build-log/2026-10-23-windows-native-test-qualification.md`.

## Decision

Test harness portability to native Windows goes through one standard-library-only package,
`internal/testplatform`, instead of per-file `runtime.GOOS` checks:

- `ExeSuffix()` / `Executable(name)` give a `go build -o` spy the platform suffix, so the binary
  the test then runs or puts on PATH is found.
- `SkipPOSIXShell(t, reason)` skips on Windows only, with a stated reason, when the test needs a
  `#!/bin/sh` spy, `sh -c`/`bash` steps or a POSIX tool (`cat`, `/bin/sleep`).

The package imports only the standard library because `internal/testsupport` imports
`internal/procgroup`; a helper there would create import cycles for packages under it.
`interop/cem01-go` is a separate module and keeps an inline `runtime.GOOS` check.

Each package was fixed in test code only. A spy that is a Go program got the suffix; a test whose
subject is a POSIX shell script or shell spy was skipped on Windows rather than rewritten, because a
rewrite would change what the test proves. No darwin/linux test is skipped, and no assertion changed.

## Limits

- No native Windows run was made for this change. The per-package fixes come from reading each
  test, guided by the V1-2441 classification; the owner reruns the listed packages on the host.
- Skipped tests leave those harness paths unqualified on Windows, the same as before.

## V1-2486 follow-up: deliberate platform refusals skip on Windows

`testplatform.SkipWindowsRefusal(t, err)` skips only when `GOOS` is `windows` and `err` contains
one of the exact production refusal messages in `testplatform.PlatformRefusals`; the skip reason
quotes the refusal and its source file. The list is explicit rather than a pattern such as
"unsupported", so an unrelated Windows failure still fails. `TestPlatformRefusalsMatchTheirSources`
rereads each source file, so a reworded refusal fails that test instead of silently no longer
matching. On darwin and linux the helper returns without effect, proved by
`TestSkipWindowsRefusalDoesNothingOffWindows`.

The first call site is `testsupport.RunGit`: fixture git runs under `procgroup`, which refuses
Windows with `process-platform-unsupported`, so every test using that fixture now skips there.
The remaining V1-2486 packages hit refusals through production call chains at individual
`t.Fatal(err)` sites. Wiring those needs the per-test failure text from a native Windows run and
stays open on the ticket.
