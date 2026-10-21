# Lane cem: CEM 1.0 ticket sweep (V1-2136, V1-1329, V1-2106, V1-1470)

Date: 2026-10-21. Base: origin/main c9ca8beff. Spec: `docs/specs/cem-stable-v1.md`, section "Lane
cem sweep", `CEM-V1-242..245`, accepted under the owner's standing auto-accept of lane
requirements (2026-10-11).

## Problem

- V1-2136: the sealed `REPOSITORY-ENVELOPE.md` recipe names `CEM_STABLE_REPOSITORY_CASES` and four
  native tests that did not exist.
- V1-1329: the tamper matrix did not compare `verdictSha256` across the native and portable
  consumers.
- V1-2106: out-of-tree map reads (`CEM-V1-315`) and absolute `cem verify-stable --output` reports
  checked the parent by path, then used the path again; a parent swapped for a symlink redirected
  the read or the write into `.git`.
- V1-1470: the portable op-timeout descendant test raced a 500 ms timer armed before the shim
  launched and failed under host load.

## Decisions

- The sealed recipe and `EXPECTED.json` are not rewritten (sealed rewrites are declined, V1-1296).
  The code is made true to the sealed text instead: new test names, and a digest table carried
  identically in both consumers' tests.
- V1-2106 uses a held `os.Root` on the checked parent (`publish.Root.hold`), with identity equal to
  the `Lstat` taken at the check. Reads go through `ReadBoundedMember` (regular, bounded,
  `O_NOFOLLOW`); absolute reports go through `PublishMember` (exclusive temp member, sync, rename,
  directory sync). The repository-relative publish path is unchanged.
- `CEM-V1-348` (plain-assembly bundle parent) stays a separate open limit.
- V1-1470: a first attempt that waited inside the `opTimeout` hook deadlocked, because the hook
  runs before the control frame that launches the shim. Replaced by a bounded retry ladder that
  still fails when no attempt records a descendant.

## Evidence

- Focused tests through `heavy.sh` pass on Darwin: the four `TestStableRepositoryEnvelope*`
  tests, `TestCEMV1226TamperMatrix{Native,Portable}`, `TestCEMV1244*` (`internal/cem/workflow`),
  the `internal/cem/publish` package, the Core CLI `TestCEMV1315`/`TestCEMV1316`/report and output
  tests, and `TestStableLifecycleOpTimeoutRetiresDescendants` (0.56 s).
- Fail without the fix: the `TestCEMV1244` map test read the redirected file and the report test
  wrote into `.git`; a mutated digest pin fails both tamper tests; a simulated delayed shim fails
  the previous op-timeout test and passes the new one on its 2 s retry.
- `go vet` on touched packages, `gofmt -l` and cross builds for windows and freebsd are clean.

## NOT_RUN

- V1-1470 criterion 2 (the full `interop/cem01-go` package runtime under load).
- V1-1329's original reproduction at 758e2f45, which is not reachable in this clone; stage
  convergence was already delivered by V1-1340 (4e03ff0a1).
- `make gate` (not required for scoped lane work).

## Rollback

Revert the lane commits. No stored state, sealed member or committed map changes.
