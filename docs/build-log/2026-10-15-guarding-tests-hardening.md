# 2026-10-15 Guarding-test and hunk-revert hardening (V1-1449, V1-1452, V1-1453, V1-1454, V1-1376)

This batch fixes the residuals of V1-1266 (PR #944); see
`docs/build-log/2026-10-15-v1-1266-guarding-tests-k1.md`. It adds no new action and does not touch
the frozen packet or `protocol/cem-1.0/stable/`.

New requirements, all accepted under the owner's standing auto-accept (2026-10-11):

| Ticket | Requirement | Contract |
|--------|-------------|----------|
| V1-1453 | `TCQ-V0-063` | A guarding-test plan row never names a span longer than `cem cite` accepts (1 MiB). |
| V1-1452 | `CEM-V1-240` | `--hunk-revert` and `--guarding-tests` refuse a `TMPDIR` inside the repository. |
| V1-1449 | `DCW-V0-069` | The dogfood bind path records the guarding-test step as `NOT_PRODUCED guarding-tests-not-run`. |

The IDs were checked as free against `origin/main` and every open PR branch.

## Decisions

- **One shared span bound.**
  - `wire.MaxCitationSpanBytes` (`internal/cem/wire/map.go`) replaces the private
    `maxCitationSpanBytes` in `internal/cem/workflow`.
  - `cem cite` and the guarding-test search both read it.
  - The search skips a candidate whose header span exceeds the bound and tries the next guard,
    rather than printing a row that `cem cite` would refuse.
- **One shared temporary-directory helper.**
  - `disposableWorkspace` (`internal/cem/verify/hunkrevert_run.go`) resolves `os.TempDir()`
    through symlinks.
  - It refuses the location when it lies inside the worktree top level or the Git common directory.
  - Both flags call it. A refusal is an infrastructure fault, so every row reads
    `not-run infrastructure` and nothing is exported.
- **The bind path records the step and does not run it.**
  - Running the search inside `dogfood-change` would add up to 1200 s of unconfined test
    execution to every bind. That cost conflicts with the bounded daily loop.
  - Instead the step counts the hunks the search would select: non-mechanical `.go` (not `_test.go`)
    hunks without a `test-claim` basis on a `_test.go` path.
  - When that count is above zero, it reports `guarding-tests NOT_PRODUCED guarding-tests-not-run`
    and prints a NOTE with the exact `cem verify --guarding-tests` command.
  - The row is observed like every other step (SOL-V0-008). `failing()` exempts exactly that
    reason, so the row never blocks completion. finish-lane still lists it as non-PRODUCED
    (DCW-V0-062).
  - `guarding-tests` and `guarding-tests-not-run` join the closed observation allowlists.
- **Status wording.** The V1-1266 section of `cem-stable-v1.md` and the TCQ-V0-060..062 section now
  say `accepted`, matching their recorded acceptance (V1-1454).
- **Help text.** `cmd/corvint/help.go` documents `--guarding-tests`, including its bounds
  (300 s per run, 1200 s total), unconfined execution and the `TMPDIR` refusal.
  - Open PR #932 adds the `--hunk-revert` entry, which is the rest of V1-1376. The two entries do
    not overlap.
  - `help.go` is not a pinned use-case receipt subject.

## Owner decisions (not delivered)

- **V1-1450: cross-package guards.**
  - `TCQ-V0-062` is accepted and defines candidates as tests in the hunk's own directory.
  - Widening the search to importing packages amends that requirement, so it needs an owner
    decision.
  - Proposal: a bounded candidate set of in-module packages that import the hunk's package.
  - That set raises K5 cost, so it should land together with or after V1-1451's narrowing.
- **V1-1451: K5 latency.**
  - The ticket asks for three measured runs before any fix.
  - Each run of the search on a real change takes 500 to 1200 s: PR #934's change took 1203.8 s, and V1-1266's
    own 20-hunk change took 512.3 s.
  - A run that long exceeds this lane's 600 s foreground limit, and background jobs are forbidden.
  - Host load was 52 to 73 during this lane, so a timing taken here would not be a usable baseline.
  - Every narrowing idea (capping or ranking candidates per package) also amends `TCQ-V0-062`.
  - The measurement is left for the orchestrator, using the commands in the lane report.

## Evidence

Load averages were recorded with each run.

- **`TCQ-V0-063`.**
  - `TestTCQV0063PlanRowRespectsTheCiteSpanBound` passes: a header of exactly 1 MiB is cited, and
    one byte over falls to the next guard.
  - Mutations:
    - Removing the check fails the over-bound case.
    - Changing `>` to `>=` fails the exact-bound case.
- **`CEM-V1-240`.**
  - Both tests pass at load 73.39/71.88/69.04, inside a package run of 553.7 s:
    - `TestCEMV1240HunkRevertRefusesRepositoryTMPDIR`, 99.65 s.
    - `TestCEMV1240GuardingTestsRefusesRepositoryTMPDIR`, 54.69 s.
  - Mutation: disabling the inside-repository check fails both tests (rows become held or judged)
    at load 68.58/69.02/69.53.
- **`DCW-V0-069`.**
  - `TestChangeDCWV0069RecordsGuardingTestsNotRun` passes in 26.8 s.
  - Mutations:
    - Removing the step call fails with "report lacks the guarding-tests row".
    - Removing the `failing()` exemption fails with "listed as a failure".

## Independent review (one Codex round)

- **P2, fixed: case aliases bypassed the `TMPDIR` refusal.**
  - The first helper compared resolved path text. On a case-insensitive volume, `TMPDIR=/x/REPO`
    names the repository `/x/repo` and was accepted.
  - The helper now compares the file identity (`os.SameFile`) of the temporary directory and each
    ancestor with the worktree and the Git common directory. This also stops a root on another
    volume from being mistaken for an ancestor, which was Codex finding 3.
  - `TestCEMV1240WorkspaceRefusesEverySpellingOfTheRepository` covers the worktree, a
    subdirectory, the Git directory and a case alias, plus a directory outside that is used.
  - Mutation: a text comparison of the resolved roots fails the case-alias row.
- **P2, fixed: the NOTE command broke on repository paths with spaces.**
  - The artifact root is now quoted as one POSIX shell word (`shellWord`), covered by
    `TestShellWordQuotesOnlyWhenNeeded`.
- **P3, recorded: the fixture sets only `TMPDIR`.** On Windows, `os.TempDir()` reads `TMP` and
  `TEMP` instead. The finding is latent, because the stable verifier refuses non-Unix hosts before
  it reaches the helper.

## Dogfood

- **Pass 0, without inputs.** The guarding-test row worked on this change: the run printed
  `guarding-tests NOT_PRODUCED guarding-tests-not-run hunks=15` with the search command.
- **Full binding.** The run with all four inputs (3 intent specs, a 44-row citation plan) did not
  finish within the lane's 600 s foreground limit at load 56 to 73. It ran twice:
  - The cite step bound all 44 hunks.
  - Each `ocm-prepare` took about 5 minutes per intent spec.
  - One `ocm-prepare` failed with `verification-timeout`.
- **Not produced.** The bind, check and seal are `NOT_PRODUCED` here, with the reason "dogfood
  change exceeds the lane's 600 s foreground limit under host load". They are left for the
  orchestrator.

## Rollback

Revert this change. The help entry, the NOTE and the observation allowlist entries are additive, and
no stored state changes format.
