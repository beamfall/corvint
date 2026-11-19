# Line evidence executable fixture repair

Ticket: V1-5697. Requirements: LEV-V1-003, ASD-V0-929.

The host verifier rejected `TestLEVV1003RefreshUnknownStatus` because its Git stub
used `os.WriteFile` with executable permissions. A concurrent fork can inherit the
open write descriptor and cause ETXTBSY when the test executes the stub.

Use the existing `execfile.WriteFile` helper, which excludes forks while the write
descriptor is open. The executable bytes, permissions and all three existing
assertions remain unchanged. This repairs fixture construction, not analysis;
there is no new syntax allowlist. The existing cleanliness allowlist remains
exactly successful Git status with empty output. Unknown records and failed empty
output still fail closed.

The focused executable-write check, line-citation check and changed-checks pass.
The whole `internal/lineevidence` package passes in 12.502s through `script/heavy.sh`,
with native JavaScript fixtures enabled and no test-name filter. Native Ruby
qualification remains unavailable as recorded in the lane handback.
The external package log `lev-evidence/verifier-exec-write-package.log` has SHA-256
`db91dcc9c30736f611f41f950c26e1031dadac08c647616fa4fdb4107c377edc`.
No product behavior or frozen contract changes.
