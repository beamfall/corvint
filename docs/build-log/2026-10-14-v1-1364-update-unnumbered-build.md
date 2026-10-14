# 2026-10-14 corvint-update check reports the available release for an unnumbered local build (V1-1364)

## Intent

`corvint-update check --allow-network` printed its header and exited with only `invalid numeric build`
on this machine, whose Core (`Corvint 1.0.0-rc.3 (build 0)`) and Tasks
(`0.0.0-tcp01-unverified+build.0`) are local builds from `origin/main`. `version` in
`internal/update/update.go` rejected build 0 and a missing build number as an unknown identity, so
the operator learned nothing about the available release. Ticket V1-1364 (P3 BUG) asks that check
report the latest release and say plainly that the installed build is unnumbered, while apply keeps
refusing to replace it without an explicit operator choice. Scope: 1.0 scope per V1-0020 dependency.

## Decisions

- New requirement `UPD-V0-010` in `docs/specs/operator-update-v0.md`, accepted under the owner's
  standing auto-accept of lane requirements (2026-10-11); acceptedBy column `standing`.
- `version` now checks the Core identity first, then returns the sentinel `errUnnumbered` for build 0
  or a label with no build marker. A marker that does not read as digits (`build -1`) or a build that
  overflows stays `invalid numeric build`.
- Stable token: the new result field `installedBuildStatus` is `UNNUMBERED`. `relation` stays in the
  closed `UPD-V0-009` set as `UNKNOWN`, with no basis and a `relationReason` saying the build is
  unnumbered or local and cannot be ordered against releases. For Tasks the bound declared build is
  still reported as `availableBuild`; for Core the version label is not compared, because a local
  build of a released label would otherwise read `CURRENT`.
- Apply refuses with `refusing to replace unnumbered installed build: ...` after discovery and before
  any download or transaction. No flag exists for this and the spec lists force-downgrade as a
  non-goal, so none was added; the operator replaces a local build by hand.
- A candidate whose own label is unnumbered stays refused (`unknown or zero candidate numeric build`).
  Rollback does not call `version` and is unchanged.

## Failure modes

- A local build that reuses a release's build number is compared as that number.
- An unnumbered build newer than every release keeps reporting `UNKNOWN` until it is replaced.
- Under host load the owned-process probe's `/bin/ps` snapshot (1 s bound, `internal/procgroup`) can
  time out, and check then exits with `version smoke failed: ... descendant snapshot unavailable`
  before any release request. This is pre-existing and outside this change.

## Acceptance evidence

- `TestUPDV0010UnnumberedLocalBuild`: Core and Tasks, each with build 0 and with no build number,
  check as `UNNUMBERED` / `UNKNOWN` with the available tag (Tasks declared build 500), write no
  state; apply refuses with the destination digest unchanged and no transaction created; a
  `build -1` label stays `invalid numeric build`; a build-0
  candidate over an installed 163 is refused as an unknown candidate build.
- `GOTOOLCHAIN=local go test -count=1 -run UPDV00 ./internal/update/`: PASS (156.9 s), and
  `go test -race -count=1 ./internal/update/ ./cmd/corvint-update/`: PASS (88.4 s), both with the
  scratch-only widening below; after the review fixes, `-run UPDV00` PASS (47.2 s) and `-race -run
  'UPDV0010|UPDV0009|UPDV0004|UPDV0001'` PASS (44.0 s) under the same widening; `go vet` on both
  packages: clean.
- Host-load limit: with load averages near 60 the probe's 1 s `ps` bound timed out (`ps` took about
  1.5 s), and every `version`-probing test in the package failed, including unchanged tests such as
  `TestUPDV0009CheckStatesRelation` and `TestUPDV0005ApplyRollbackAndPreparedReceipt`, with
  `descendant snapshot unavailable`. The passing runs above used an uncommitted local change of
  `descendantSnapshotTimeout` to 10 s and `descendantSettle` to 30 s in `internal/procgroup`,
  reverted before commit. CI runs the package with the committed bounds.
- Live, read-only (2026-10-14), updater built from this branch into the lane scratch directory,
  installed tools untouched, no apply, PATH unchanged. Committed bounds; the first Core attempt hit the
  `ps` timeout above, the retry succeeded:

```
$ corvint-update check --allow-network --component tasks
{"component":"tasks","installedVersion":"0.0.0-tcp01-unverified+build.0","installedBuildStatus":"UNNUMBERED","availableTag":"tasks-dev-20260929.2","availableBuild":202,"relation":"UNKNOWN","relationReason":"the installed build is unnumbered (build 0 or no build number, a local build); its freshness cannot be ordered against releases","freshness":"RELEASE_OBSERVED","action":"check","qualification":"release evidence only; no broad qualification; atomic visibility, no power-loss durability guarantee","publisherIdentity":"NOT_VERIFIED"}
$ corvint-update check --allow-network --component core
{"component":"core","installedVersion":"Corvint 1.0.0-rc.3 (build 0)","installedBuildStatus":"UNNUMBERED","availableTag":"v1.0.0-rc.2","relation":"UNKNOWN","relationReason":"the installed build is unnumbered (build 0 or no build number, a local build); its freshness cannot be ordered against releases","freshness":"RELEASE_OBSERVED","action":"check","qualification":"release evidence only; no broad qualification; atomic visibility, no power-loss durability guarantee","publisherIdentity":"NOT_VERIFIED"}
```

## Independent review

One Codex read-only round (CONCERNS, two findings, both confirmed and fixed):
- A refused apply still runs the `UPD-V0-007` stale-transaction sweep before the refusal, so "state
  unchanged" was too strong. The sweep is intended lock-held housekeeping for every apply; the
  requirement now says the refusal creates no transaction and leaves the destination unchanged.
- A label with a malformed build marker (`build -1`) was classed as unnumbered. It now stays
  `invalid numeric build`; only build 0 or no build marker is unnumbered, with a test case.

## Retained limits

- Apply refusal on the live host is NOT_RUN by design (the lane may not run apply); it is covered
  by the fixture test only.
- Linux and other hosts: NOT_RUN.

## Rollback

Revert this change: `version` again rejects build 0, `installedBuildStatus` disappears, and check on
an unnumbered install exits with `invalid numeric build` as before. No stored state changes shape.
