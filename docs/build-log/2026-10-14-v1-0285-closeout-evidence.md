# 2026-10-14 V1-0285 closeout evidence: CCF-V1-007 N-1 premise and frozen-enumeration register

Ticket V1-0285 (panel B6). Base `495eda08cf3dc4d8122b169a4cc16d9c4e599b19` (origin/main). Scope: 1.0 scope
per V1-0020 dependency. The Fix was delivered by commit a6f89b4e7 (decision 0401) and V1-0350 (0668cb13b,
decision 0398, accepted by decision 0422), but the ticket was never closed. This entry audits each Fix
element at the base and repairs the one residual gap the audit found.

## Audit, per Fix element

a. N-1 premise: met, with a stale-baseline residual fixed. The spec states the 0.8.1 baseline in the
   scope section and in CCF-V1-007 and records why it is not 0.7.0 (the `instruction-routed` relation
   added by 2f3bfe6). `grep -n -i "nothing changed"` over the whole file finds nothing; lines 120-130 now
   hold the CCF-V1-004 exemptions. The Codex round found that accepted decision 0434 (item 3) later
   named `v1.0.0-rc.1` the N-1 baseline, which the spec did not say and the Makefile default (which
   excludes pre-release tags) does not select. The spec now cites decision 0434 and notes that runbook
   step 8 passes the tag explicitly; the replay passes against `v0.8.1`, `v1.0.0-rc.1` and `v1.0.0-rc.2`.
b. Every frozen enumeration listed with values and open/closed status: met. The CCF-V1-007 (d) table has
   37 rows; `coverage.unexamined[].relation` lists `instruction-routed` and is `open`.
   `observeCoreEnumerations` / `observeAs` in `cmd/corvint/core_freeze_test.go` parse the table from the
   spec and fail when a frozen mode emits a value outside its row or a row is reached by no mode.
c. Release gate building the previous tag and replaying the Core verbs: gate residual-fixed (it was red,
   see below); coverage residual-open: the N-1 replay reaches 9 of the 12 verbs, not all 12 as the Fix asks.
   `make core-n1-replay` (Makefile) builds `CORE_N1_TAG`, defaulting to the newest non-prerelease tag
   before `HEAD` (resolves to `v0.8.1`; `v1.0.0-rc.1`/`rc.2` are excluded as prereleases), and runs
   `TestCoreVerbsEmitTheFrozenProfiles` with `CORVINT_CORE_N1_BINARY`. It is wired into release-runbook
   step 8 (`docs/RELEASE-RUNBOOK.md`, "It must end `ok`; retain its output"), not into
   `release-gates.yml` or the `script/release-checklist` readiness rows, and not into `make gate`
   (owner decision per CCF-V1-007). The test has 28 cases over 11 of the 12 decision-0332 verbs:
   `init`, `adopt`, `index`, `query`, `context`, `impact`, `affected`, `prove`, `cem`, `ocm`, `dogfood`.
   `frontier` has no success case: its `frontier --json` golden was retired by decision 0525 §6
   (CEM-V1-212) because no admitted `cem/1.0` map produces `frontier/0`; its refusal stays pinned by
   `TestCoreRefusalsKeepTheFrozenEnvelope`, which the replay does not run.
   With the four skips below, `cem` and `ocm` are not replayed either, so the N-1 verbs are `init`,
   `adopt`, `index`, `query`, `context`, `impact`, `affected`, `prove` and `dogfood`. Accepting nine of
   twelve under decision 0525 is an owner call; it is not relabelled as met.

## Residual gap found and fixed

`make core-n1-replay CORE_N1_TAG=v0.8.1` at the base exited 2: the N-1 replays of `cem status`,
`cem verify`, `ocm status` and `ocm verify` failed. V1-1278 (744e8fc44) moved their fixture to a `cem/1.0`
map; 0.8.1 refuses `--artifacts` (`invalid-arguments`) and reports `invalid-cem` for the OCM modes
(observed by printing the N-1 stdout in a scratch run). Decision 0525 §6 superseded the `cem/0.2` freeze
and its N-1 reader, so these replays cannot pass and the release-runbook step 8 gate was red.

Decision: add the four modes to `coreN1Skips` with one reason constant citing decision 0525 §6, and amend
the CCF-V1-007 N-1 skip list in the spec (accepted under the owner's standing auto-accept (2026-10-11);
V1-0285). No new requirement ID; the header status, Agent digest status and Rollback carry the
amendment. Rejected: replaying the N-1 binary over a `cem/0.2` fixture, because the 0.x producers were
deleted (c54db91e6) and this build refuses 0.x maps, so the two runs would not compare one mode.

## Failure modes

- A future accepted break makes a replayed mode fail: `make core-n1-replay` exits nonzero at runbook
  step 8 until the skip list and spec are amended together.
- The skip hides a real `cem`/`ocm` regression against N-1: not caught by the replay; the current-build
  goldens of `TestCoreVerbsEmitTheFrozenProfiles` still pin those modes.

## Acceptance evidence

- `make core-n1-replay CORE_N1_TAG=v0.8.1` at base: exit 2 (four cem/ocm N-1 failures). After the change:
  exit 0, `ok github.com/Beamfall/corvint/cmd/corvint 100.489s`.
- After the change, `make core-n1-replay CORE_N1_TAG=v1.0.0-rc.2` and `CORE_N1_TAG=v1.0.0-rc.1`: exit 0 each.
- Verbose replay with a prebuilt v0.8.1 binary: 9 N-1 skips (5 earlier + 4 new), 18 N-1 PASS; one
  current-build `affected full worktree` case failed with `git-cancelled` at load average 64
  (environment timing, untouched code).
- `go test -run '^TestCoreVerbsEmitTheFrozenProfiles$' ./cmd/corvint`: 28 cases PASS.
- `go test -race` on `TestCoreVerbsEmitTheFrozenProfiles|TestCoreRefusalsKeepTheFrozenEnvelope`: no data
  race; three cases failed twice with `git-cancelled` / `cannot read the worktree status` at load 50+
  (timing only).
- `go vet ./cmd/corvint`, `gofmt -l` empty; `internal/specindex`, `internal/doccorpus/selfcorpus`,
  `script/spec-coverage-audit` PASS; the CI doc gates (listed in the PR).

## Retained limits

- Codex review (one round, read-only): CONCERNS. Finding 1 (decision 0434 baseline) confirmed and
  fixed in the spec; the Makefile default is unchanged because decision 0434 names rc.1 rather than a
  selection rule. Finding 2 (fewer than 12 verbs replayed) confirmed and recorded as residual-open above.

- The replay is opt-in (runbook step 8 only); no CI job or readiness row enforces it. Wiring it into
  `release-gates.yml` needs a `.github/` change outside this lane.
- `frontier` and every refusal envelope are replayed by no N-1 mode.
- The CCF-V1-002 identifier row for `cem status` / `cem verify` still says `verification.spec` =
  `cem/0.2`, while the test pins `stableVerification.spec` = `cem/1.0` (V1-1278); left for its owner.
- `make gate` NOT_RUN here; the orchestrator runs it at closeout.

## Rollback

Revert this change: the four `coreN1Skips` entries, the reason constant and the spec amendment. The
replay against 0.8.1 then fails on those modes again. No stored state changes.
