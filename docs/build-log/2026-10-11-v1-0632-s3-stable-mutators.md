# V1-0632 S3: Core mutators and the bind loop refuse cem/1.0 explicitly

Date: 2026-10-11. Base: `5871c380a426046548314e4710dc53513e1a70a6` (V1-0632 S1, under
independent review). Branch: `claude/cem1-s3-producer`. Scope: plan family C02 plus the
two dogfood-flow readers, with proposed requirements `CEM-V1-030` to `CEM-V1-033` in
`docs/specs/cem-stable-v1.md`. No decisions, ticket-store writes, PRs or producer changes.

## Finding

After S1, `wire.ParseMap` refuses an exact `cem/1.0` root with `unsupported-spec` and
names the stable read. The Core mutators still had three silent or misleading paths:

- `cem prepare` found a stable sidecar at `.corvint/change.cem.json` and refused it as
  `map-unavailable`, with the guidance "not a valid CEM document; pass --replace to
  regenerate". It then followed that guidance: `--replace` overwrote the stable
  sidecar with a fresh `cem/0.2` candidate (the plan's C02 downgrade hazard).
- `cem cite`, `mark`, `cover` and `discriminate` overwrote an existing `cem/1.0`
  document that was named as `--output`, as long as their own input was 0.x.
  `TestMutatorsRefuseStableDestination` showed cite doing this before the change.
  `cem begin --output` behaved the same way.
- In `internal/dogfoodflow`, `mapHunks` decoded the `hunks` of any JSON document. The
  public sealed stable positive has two hunks, `supported` and `mechanical`, so
  `citedHunks` counted it as cited. `batch-check` reported a sealed `cem/1.0`
  constituent as `constituent-cem-invalid`, the code for an unreadable or malformed
  0.x map.

## Change

- `workflow.resumeCandidate` refuses an existing sidecar that declares exactly
  `cem/1.0` before it looks at `--replace`. The refusal is `unsupported-spec` and
  nothing is published.
- `lockedMapInput` takes the mutator name. A new `readMutableMap` keeps the earlier
  read precedence. Only a `ParseMap` `unsupported-spec` refusal of an exact stable
  root becomes the mutator refusal. That refusal names the command, says that Core
  mutators produce only `cem/0.x`, and points to `cem status` / `cem verify` with
  `--artifacts`.
- After the input read, a distinct output is checked under the output lock that
  Core already holds. If its existing bytes declare `cem/1.0`, the mutation refuses
  with the same code. `cem begin` runs the same check before it publishes.
- `mapHunks` returns no hunks for a map that declares `cem/1.0`. Every other input
  keeps the earlier decode, including maps with no spec, which the dogfood tests use.
- `verifyConstituent` refuses a sealed stable map as
  `constituent-cem-stable-unsupported` before the loose decode or any verifier run.
  It prints a recovery line. `daily-change-evidence-workflow-v0.md` lists the code.

Every check selects on `wire.DeclaresStable`, the exact and bounded declared-spec
probe from S1. A padded spec, a numeric spec or the experimental candidate spec is
not selected.

## Review fixes

The independent Codex review of `5871c380a..8c040a90d` found four defects. Each is now
fixed, with a regression test that fails against `8c040a90d`:

- Unreadable stable files could be overwritten. The destination probe ignored read
  errors, and `prepare --replace` regenerated a sidecar it could not read. The strict
  probe now proceeds unread only for an absent path, a path that is not a regular
  file, or a file over the map bound. Any other read failure is refused with
  `map-unavailable` (`TestMutatorsRefuseUnreadableDestination`,
  `TestPrepareReplaceRefusesUnreadableSidecar`,
  `TestStrictProbeRefusesDestinationReplacedWhileRead`).
- `begin` neither locked its probe nor its publication. It now holds the output lock
  across both (`TestBeginProbesUnderOutputLock`).
- The refusal came after file system mutations: the lock file, and `prepare`'s backup
  recovery. A non-mutating preflight now refuses a readable stable input, destination,
  sidecar or recognized backup first. The locked check stays
  (`TestStableRefusalPrecedesLock`, `TestPrepareRefusesStableBackupBeforeRecovery`).
- Zero hunks let an empty plan bind and return `PRODUCED`. The cite step now refuses a
  stable map as `cem-map-stable-unsupported` before it reads the plan. The reason is
  admitted in `internal/observations` and has a fix hint (`TestCiteStepRefusesStableMap`).

## Evidence

- Failing before: with the new tests and the S1 implementation, these fail:
  - `TestPrepareRefusesStableSidecar` (`map-unavailable` instead of `unsupported-spec`)
  - `TestMutatorsRefuseStableInput` (the S1 parser text names no mutator)
  - `TestMutatorsRefuseStableDestination` (cite returned success and overwrote the
    stable map)
  - `TestMapHunksRefusesStableMap` (two hunks decoded)
  - `TestBatchCheckRefusesStableConstituent` (`constituent-cem-invalid`)

  The unchanged-behaviour guards pass both before and after:
  `TestPrepareNonStableExistingMapUnchanged` and `TestMutatorsWriteNonStableDestination`.
- After the change, Go 1.27.2 (`GOTOOLCHAIN=local`, host Homebrew, Darwin arm64),
  `-p 1 -count=1 -timeout 30m`:
  - `go test` of `internal/cem/...` and `internal/dogfoodflow`: every package is `ok`
    except `internal/cem/verify`. There `TestStableS0ELinkedControl` fails with
    `unsupported-structural-runtime`, because the qualified runtime tuple pins
    `go1.27.1` and the host runs `go1.27.2`. It fails the same way on the unchanged base
    `5871c380a`, and this slice does not touch `internal/cem/verify`.
  - `go test ./cmd/corvint -run 'TestCEM|TestDogfood'`: `ok`.
  - `go vet` of `internal/cem/...`, `internal/dogfoodflow` and `cmd/corvint`: clean.
- After the review fixes and the merge of S1 review head `41427384f` (merge
  `2b93ab436`), the same commands also cover `internal/observations`. Every package
  is `ok`, except the same Go 1.27.2 runtime-tuple failure of
  `TestStableS0ELinkedControl` in `internal/cem/verify`. `cmd/corvint` filtered is
  `ok` and vet is clean.
- Doc gates: `spec-requirements`, `requirement-definitions`, `traceability-tests`,
  `decision-numbers`, `line-citations`, `unbounded-readers`, `use-case-receipts` and
  `go-format`.
- `corvint affected --base 5871c380a` (Corvint 1.0.0-rc.3 build 407) selected 79
  units and excluded 313. It left 24 unknowns, all `LANGUAGE_FRONTIER`, including
  `go:build-constraint-variants` and `go:nested-module-frontier`. Its scope is
  `UNKNOWN`.

## Limits

- NOT_RUN:
  - The remaining selected units beyond the focused packages above.
  - `cmd/corvint` outside `TestCEM|TestDogfood`.
  - `conformance/cli-parity-v0` and the other conformance suites. They pin 0.x bytes
    that this slice does not change.
  - `make gate`.
  - The repository-pinned Go 1.27.1 toolchain. The host has Go 1.27.2, and the pin
    moves to 1.27.2 in a separate change.
  - Linux runs.
  - Independent review.
  - The dogfood CEM bind and seal, which are left to the integrating lane.
- The destination check binds only Core writers that take the output lock. Another
  process can still replace the file after the check.
- An unreadable regular destination of a 0.x writer used to be replaced and is now
  refused. Directories, links and files over the map bound are still left to
  publication.
- No exclusive create, and no re-check before the rename, guards against an
  uncooperative process that replaces the destination after the strict probe.
- `prepare` refuses when any recognized backup declares `cem/1.0`, even with a
  present sidecar, because resume removes a validated backup.
- The requirement IDs start at `CEM-V1-030`. That keeps them clear of slices proposed
  in parallel, which would otherwise take `CEM-V1-020` next. Integration may
  renumber them.
- Core production of `cem/1.0` is still an owner decision. The plan's default is
  that the candidate assembler stays the only stable producer, and this slice keeps
  that default.
- Rollback: revert this change. No map, packet, golden or 0.x output byte changes.
