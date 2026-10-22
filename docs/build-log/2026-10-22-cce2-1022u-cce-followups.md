# 2026-10-22 lane cce2-1022u: CCE follow-ups and corvint-test fold group 4

Lane `cce2-1022u`, stacked on the CCE lane `claude/cce-compile-1022k` (#1546, seal head
`1aa0691af`). Requirements CCE-V0-048..054 and the CTF-V0-002 amendment are accepted under the
owner's standing lane-requirement auto-accept (2026-10-11).

## Problem

The 1022k CCE lane left follow-up tickets: worker teardown failures read as infrastructure
(V1-2410), hosted proofs silently ran under a different runner config (V1-2409), harvested
proposals were unbound to the map (V1-2411), an edited oracle had no route to lift its cap and an
unresolved locator had no repair hint (V1-2413), `--coverage` had no recorder (V1-2412), and reach
missed modules only imported by an anchored file (V1-1685). V1-1667 still had groups 4 to 6 of
the `corvint-test` fold unmoved, and V1-2376 reported a broken host Node install.

## Decisions

- CCE-V0-048 (V1-2410): a worker teardown error located (PWP-V3-009) inside the staged test tree
  and outside `node_modules` reads `teardown-failed at <file>:<line>`; every other run-level error
  stays infrastructure. Playwright 1.64.0 was observed live to report a worker fixture teardown as
  a located global error and a test-scoped teardown on the test result.
- CCE-V0-049 (V1-2409): a hosted proof lists the adopter-config keys the prove config does not
  apply (`retries`, `workers`, `globalSetup`, `storageState`, `baseURL`, projects); a config it
  cannot read fully is reported as read partially.
- CCE-V0-050 (V1-2411, partial): each harvested proposal is bound to a map flow or screen or
  carries a closed unbound reason. The site-knowledge IR binding waits for #1542.
- CCE-V0-051/052 (V1-2413): a `CHANGED_IN_DIFF` oracle lifts its cap only through an independent,
  digest-bound acceptance recorded outside the repository; an unresolved outcome locator gets a
  proposed, never applied, `data-testid` rename with the diff line pair.
- CCE-V0-053 (V1-2412): the head pool records served application paths per outcome screen,
  pinned to head blobs; `--coverage` consumes that document and a stale pin refuses.
- CCE-V0-054 (V1-1685): `--module-graph` widens reach from a changed module to its importers'
  screens through the head tree's TJAA module graph. It is widen-only and never removes or demotes
  an unknown span. API contract joins and the ANL edge-to-screen hop need their own design.
- V1-1667 group 4: goldens were recorded from the old `corvint-tests-accept` first (`721706587`,
  19 cases). The `plan` request binds the absolute roots of a fixture repository committed per
  run, so CTF-V0-002 now lets the golden hold `@DIGEST@` and requires the test to show the output
  carries exactly that run's request digest. The group moved in `b9613716d`:
  `internal/testacceptance/cli` is the shared entry, the old binary is a shim, and the worker
  re-enters as `corvint-test accept worker` (CTF-V0-006) through `ExecuteWithWorker`.
- V1-1667 group 5 is not moved. Core reads the worktree from the global `--root` and the negate
  parser refuses `--root` by name, so the moved command must read only the working directory or
  gain a root option, which CTF-V0-003 forbids. Recorded as an owner decision in the fold spec.
  Group 6 needs the owner-posted `.github/` change (CTF-V0-009).
- V1-2376: the Node 22.23.2 install under `toolchains-2026-10-17` held 5 of 4753 archive entries.
  The tarball was downloaded again from nodejs.org/dist/v22.23.2; SHASUMS256 verified and
  byte-identical to the retained copy, and `bin/node` byte-identical. The 4748 missing entries
  were restored without overwriting; npx 10.9.8 runs. Manifest row added.

## Evidence

Focused tests through `heavy.sh`, `GOTOOLCHAIN=local`, go1.27.2. Each new test was shown to fail
with its fix removed; the traceability rows in `change-compiled-e2e-v0.md` and
`corvint-test-fold-v0.md` name them. `cmd/corvint-test` (all fold goldens: runner, falsify,
minimize, accept), `cmd/corvint-tests-accept` and `internal/testacceptance` pass; the CTF-V0-006
re-entry test fails with the worker argv reverted to `worker` (`repeat [worker-incomplete]`).
`go vet` and `gofmt -l` are clean on touched packages.

## NOT_RUN

- Live NEA-V0-002 through `corvint-test accept`: `TestNEAV0002ActualBrowserAssessment` fails with
  `provider-infrastructure` both after the move and at the pre-move commit `721706587`, so the
  failure is environmental and pre-existing.
- V1-1684 block vocabulary: not started (about 20 hours estimated; typed locators also need the
  app map to carry locator kinds).
- `make gate`: not run (owner preference for focused tests).

## Rollback

Revert the lane commits; each requirement is in its own commit. Group 4 reverts by restoring
`cmd/corvint-tests-accept/main.go` and dropping the `accept` dispatch; the goldens stay valid for
the old binary. The Node repair rolls back by deleting the paths in
`dl/node-22.23.2/repair-2026-10-22-added-files.lst`.
