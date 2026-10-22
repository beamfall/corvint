# 2026-10-21 miscfix-1021w: P3 sweep (V1-2213, V1-2215, V1-2164, V1-2184, V1-2145)

Lane `claude/miscfix-1021w`, base `origin/main` at `f29ddc0db09c01c370cd3c9bcc5022a296ae3876`.
AHI-173 and the AHI-020 Spec Kit/OpenSpec amendment are accepted under the owner's standing
auto-accept (2026-10-11). No other requirement ID is new.

## V1-2213: `internal/behaviorfalsify` package time

Problem: the package ran every test sequentially, each re-running hook subprocesses, and took
124 s at load 24-32 here (156 s at load 70-97 in the ticket). Every test called
`t.Setenv(helperEnvironment, "1")` and the provider tests `t.Setenv("GOCACHE", ...)`, and
`t.Setenv` forbids `t.Parallel`.

Decision: the helper marker is process-wide (`TestMain` sets it once, since every test runs its
hooks as this binary in helper mode), and `GOCACHE` is resolved once per process with
`sync.OnceValues`. 48 top-level tests and four table loops now run in parallel. Tests that still
change process state (`TMPDIR`, a profile log variable, ambient Git, termination timing, or the
BBF-V0-214 swap seams) stay sequential; Go runs parallel tests only after the sequential ones, so
they never overlap. No assertion, case or fixture was removed.

Evidence: package time 124.07 s before and 65.67 s after, both at load 24-31 through
`heavy.sh`; `-race` passes in 108.68 s.

## V1-2215: GOROOT tools and the RSpec toolchain run in place (confirmed, not fixed)

`TestBBFV0214LimitToolsRunInPlace` swaps a GOROOT tool after `stageVerified` and the RSpec
toolchain after its digest check (new seam `afterToolchainChecked`); both swapped programs run.
The BBF-V0-214 Limit now cites the test. A fix needs a design decision: GOROOT tools have no
digest pin, and private staging of the RSpec wrapper breaks wrappers that resolve paths from
their own location. Residual to file.

## V1-2164: inventory elements screened twice

Problem: `InventoryTest.UnmarshalJSON` called `Decode`, so each of up to 16,384 elements was
secret-screened and parsed again after the whole provider was. Decision: elements use
`decodeElement` (strict decode, unknown keys and trailing input refused) inside the provider's
single `Decode`. `TestAFUV1065InventoryDecodeScreensOnce` counts one screen for a 16,384-test
provider (16,385 before the fix) and keeps a secret, an unknown key or a missing project inside
an element refused. `BenchmarkAFUV1065InventoryDecodeAtCeiling`, six runs each: 622,850 to
393,450 allocations and 66.3 MB to 46.9 MB per decode. Wall time fell from 323-893 ms to
173-258 ms, but the load moved between 11 and 156 during the runs, so only the allocation
counts are a stable measure.

## V1-2184 and V1-2145: package versions

The AHI-020 gate now reads YAML versions (`yaml:PARENT.CHILD`) and covers the Spec Kit extension
and the OpenSpec skill. The skill file stays in its own shipped set because it is also the main
content. Its failure message names `make host-package-version-bump PKG=<name>`.
`script/host-package-version.sh` (AHI-173) bumps or verifies all locations of a package. It uses
format-preserving edits checked against jq `setpath`, and it leaves recorded evidence versions
alone. The new gate cases fail against the previous check script.

## Not delivered

V1-1742 was skipped. `flows export` omits qualified targets by design: AFU-V1-071 notes that the
record has no revision for an application repository. Docs maintenance has no repository
bindings. Round-tripping needs an EEP record design for repository revisions and binding inputs
for maintenance.

## NOT_RUN

`make gate`, and tier-2 affected units, are left to batch CI. Live Spec Kit and OpenSpec installs
were not run.

## Rollback

Revert the lane commits. Each ticket is a separate commit, apart from V1-2184 and V1-2145, which
share one.
