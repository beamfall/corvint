# nd2-1021: bind-range guarding-tests NOTE (V1-1487) and locator-free top checks (V1-2162)

Date: 2026-10-21. Lane nd2-1021. Base: origin/main c9ca8beff43d8f548b054d3b3d40e3054cecf816.
The requirements below are accepted under the owner's standing auto-accept (2026-10-11).

## V1-1487: staged binds print the guarding-tests NOTE (DOGFOOD-BIND-008)

Problem: the single-shot `dogfood-change` path prints the DCW-V0-069 guarding-tests NOTE, but
`script/dogfood-bind-range.sh` (the staged/retroactive bind path) did not. Counting the selected
hunks in shell would have meant parsing the CEM map with awk.

Decision: `internal/dogfoodflow.GuardingTestHunksFile` exposes the existing hunk rule and read
bound. The new `tools/dogfood-guard-hunks MAP` prints that count. The script builds the tool (or
takes `CORVINT_GUARD_HUNKS_BIN`) after a successful bind and prints one NOTE naming the hunk count,
the binding and the `cem verify --guarding-tests` command. If no count is produced, it prints
`guarding-tests-count-unavailable` and the bind still passes. DOGFOOD-BIND-008 is in
`docs/DOGFOOD.md`.

Evidence: `script/dogfood-bind-range_test.sh` passes (15.5 s at load 14 to 58). Without the script
change it fails at the new NOTE assertion. go vet and gofmt are clean.

## V1-2162: worktree-top checks without the root locator (AFP-V0-156)

Problem: `internal/appmap` and `internal/appflows` used `rev-parse --show-toplevel` only to
compare the result with the given root. The literal still made each package a rule (d) root
locator (AFP-V0-012), so 63 packages inherited unresolved status from `internal/appflows`.

Decision: `rev-parse --is-inside-work-tree --show-prefix` must print exactly `true`, a newline, an
empty prefix and a newline. That proves the root is the worktree top without naming a top path.
The refusal reasons stay unchanged. `internal/extevidence.resolveCheckout` had the same pure
comparison and was changed too. The detector is unchanged. Packages that read through a located top
stay readers.

Evidence:
- New tests pin the plain-directory, Git-directory and subdirectory refusals and the absence of the
  literal. Each locator assertion failed before the change.
- The unresolved count did not drop. `tools/gate-affected-select -unresolved` gave 104 at base and
  104 after, not counting the new `tools/dogfood-guard-hunks`, which adds one inherited row. The
  detector names only the first culprit, so the 63 packages now inherit from
  `internal/jstestprovider`. `internal/worksource` and `internal/postmergeconnector` are also
  reached. All three use the located top for later Git reads, so they are real readers.
- One-package selection, `-bounds` for `internal/appmap/gitread.go`: 130 bound and 105 unresolved,
  against 130 and 104 at base.
- Load averages during the measurements were 55 to 120 on a 12-core host. Each `-unresolved` run
  took 1.9 to 2.4 s.

The ticket's second criterion, a 62-package drop, is therefore not met. The remaining floor needs
those three packages bounded or declared under AFP-V0-023.

## Not delivered

- V1-1444 is already delivered: `docs/build-log/2026-10-16-s0e-go1.24.13-and-linux-lifecycle.md`
  records case 21 passing under go1.24.13, and `FULL-RESULT-CASES.json` is unchanged since then.
- V1-2150 is skipped. Pi's own argument conversion coerces the null. The fix needs an adapter bump
  to 0.3.3, which re-opens the owner-accepted Pi 0.99.1 / 0.3.2 tuple (decision 0452, LCP-V0-008/009,
  AHI-024, the `local_completion_event.go` admission) and needs a live Pi re-qualification.
- V1-0438 is skipped. The spec, conformance cases 364 to 380 and the README define the degraded
  `UNVERIFIED_IMPORT` import as current behaviour. Removing the command or building a verifier is an
  owner choice.
- V1-0233 is skipped: its required gate is the full gate, which lanes do not run.
- V1-1803 is not started (240-minute estimate, over the lane budget).

## NOT_RUN

- `make gate` and the full test suite; only the focused tests were run.
- A go1.24.13 rerun for V1-1444, which would need a network toolchain download.

## Rollback

Revert the lane commits. The script NOTE is advisory, and the top checks keep their refusal
reasons, so no stored state depends on either change.
