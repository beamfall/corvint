# 2026-10-15 V1-1266: guarding-test search (`TCQ-V0-062`) and the K1 tally

V1-1266 is decision 0523 P2. The consumer falsifier `cem verify --hunk-revert` (`CEM-V1-088`/`089`)
and the offered citation step (`TCQ-V0-060`/`061`) landed in #893; see
`docs/build-log/2026-10-14-v1-1266-hunk-revert-held.md`. Nothing here amends those rows, the frozen
packet or `protocol/cem-1.0/stable/`.

This change adds the producer step that finds the guarding test, plus the K1 tally. The new
requirement is `TCQ-V0-062`, accepted under the owner's standing auto-accept (2026-10-11).

**The ticket stays open.** K1 needs 30 dogfood changes after P2 lands, and this entry records only
the first observations.

## Decisions

- **The step is a flag on `cem verify`.**
  - It cannot be a new action: the `cemActionOrder` is parity-frozen.
  - It cannot go on `cem status`: the `CEM-V1-088` non-goals forbid changes there.
  - `--guarding-tests` adds the envelope member `guardingTests` (profile `cem-guarding-tests/0`).
  - `status` and `report` refuse the flag. `verify` refuses it beside `--hunk-revert`, so one run
    never both suggests a test and credits it.
  - The stable result stays byte-identical with and without the flag.
  - A prepared (all-`unknown`) `cem/1.0` map ACCEPTs without `--max-unknown`, so the producer can
    run the search before it cites anything.
- **Observation, not guess.**
  - Candidates are base `TestX(t *testing.T)` declarations in the hunk's directory whose bodies
    mention the edited declaration or its same-directory closure.
  - Each candidate is judged by the consumer's own `judgeCandidates`/`classifyRevert`.
  - Consequently, a revert that breaks the build guards nothing, and neither does a test that fails
    without the revert.
  - The plan row cites the guard's base header lines. It is emitted only when the frozen drift
    classifier keeps those lines stable or relocated, so `cem cite` admits it as written.
- **It sets no tier.** `held` still comes only from `--hunk-revert`.
- **Non-Go runners are a recorded limit.** No runner other than `go test` is supported or
  qualified. The rc.3 text is in `docs/RELEASE-NOTES.md` ("1.0.0-rc.3 (unreleased): recorded
  limits"), and the spec limits paragraph follows `TCQ-V0-062`.

## K1 tally definition

K1 (decision 0523 §5) asks whether fewer than 25% of non-mechanical Go code hunks are `held` across
the next 30 dogfood changes after P2 lands.

The tally is taken per sealed map `.corvint/changes/<bind>.cem.json`, where `<bind>` is the map's
bind commit:

- **Denominator.** Count the hunks whose `path` ends in `.go` but not `_test.go` and whose
  `disposition` is not `mechanical`:
  `jq '[.hunks[] | select((.path|endswith(".go")) and ((.path|endswith("_test.go"))|not) and .disposition != "mechanical")] | length'`.
- **Numerator.** Take `.hunkRevert.held` from the following run, made in a clean clone at `<bind>`:
  `corvint cem verify --map .corvint/changes/<bind>.cem.json --expected-base <base> --target "$(git rev-parse <bind>^)" --artifacts "$PWD" --hunk-revert`.
  - A hunk with no `test-claim` citation cannot be held.
  - So the citation count is an upper bound, and the consumer run is needed only when it is
    nonzero.
- **Which changes count.** Count changes whose seal lands after this change merges. A chore
  change, meaning a bind or seal only, has no Go hunk and adds nothing.
- **Share.** The share is the sum of the numerators over the sum of the denominators. Record each
  change's pair as well, so that one large change cannot hide the rest.

## First observations

All three observations below are in the pre-producer window.

| Window | Maps | Maps with Go code hunks | Denominator | `test-claim` cited | `held` |
| --- | --- | --- | --- | --- | --- |
| Sealed after #893 (`3fcc1d11c`) up to `883caa3d2` | 35 | 20 | 136 | 0 | 0 |

- **Sealed maps.** No dogfood change has cited a guarding test since the offered step landed, so
  the held share is 0. That is the expected baseline without a producer step.
- **Probe on a real change.** The probe ran `--guarding-tests` on PR #934's change (base
  `85ba6321a`, target `95622955d`), whose prepared map has 105 hunks. It searched 42 non-test Go
  hunks in 1203.8 s, which is the full 1200 s budget, at load 18.6 to 55.6.
  - Results:
    - 1 guarded, with one plan row: `internal/gateselect/readscopes.go`, guarded by
      `TestSelectPackagesHonorsDeclaredReadScopes`.
    - 7 unguarded.
    - 12 `revert-breaks-build`.
    - 9 with no candidate.
    - 2 `timeout`.
    - 11 `total-limit`.
  - Two cost drivers:
    - **Large packages.** `internal/tasks/store` (394 candidates) and `internal/tasks/cli` (362)
      each ran to the 300 s per-run bound.
    - **Interdependent hunks.** Reverting one hunk of a change whose hunks depend on each other
      breaks the build.
  - The 1-in-42 rate and the 1200 s cost are the first K1 and K5 data points. They suggest K1 is
    at risk unless candidate runs are narrowed.
- **This lane's own change.** The search ran on the committed change, base `883caa3d2`, which
  has 20 hunks, 8 of them non-test Go code. It took 512.3 s at load 33 to 42.
  - Results:
    - 0 guarded.
    - 3 unguarded: the two `cli.go` flag hunks and the `goTestDecl` refactor.
    - 4 `revert-breaks-build`: `read.go`, both `stable_read.go` hunks, and one `hunkrevert.go`
      hunk.
    - 1 `not-a-modification`: the new file `guards.go`.
  - So the pair for this change is (denominator 8, `held` 0). That is expected for a change whose
    behavior is pinned only by tests the change itself adds, which `cem cite` cannot cite as base
    evidence.

## Citation repin

The rc.3 section moves `docs/RELEASE-NOTES.md` down by 25 lines. As a result, the two line
citations of decision 0375 were repinned from 239-240 to 264-265 and from 230,253 to 255,278. The
content hashes `@6a77dbd2` and `@1528a86b` are unchanged, because the cited text is the same.

## Fixes found by the probe

- **Wrong reason after the budget ran out.** Three rows with no candidate were reported
  `total-limit` once the budget was spent, but the spec says `no-candidate-test`. The fix: a
  modification hunk with no candidate is now `no-candidate-test` whatever bound was spent.
  - Pinned by `TestTCQV0062SpentBudgetKeepsNoCandidateReason`.
  - Mutation M8 (drop the clause) fails that test.

## Tests and mutations

The focused tests are the `TestTCQV0062*` tests in `internal/cem/verify`, `internal/cem/workflow`
and `internal/cem/cli`. At BASE `883caa3d2` the flag does not exist (`invalid-arguments`).

Each mutation below was applied alone and failed its test, and the file was restored afterwards:

| Mutation | Change |
| --- | --- |
| M1 | candidates ignore mentions |
| M2 | fails-without-revert counted as guarded |
| M3 | a build break counted as guarded |
| M4 | no drift-stability check on the citation |
| M5 | `status` accepts the flag |
| M6 | no same-directory closure |
| M7 | a hunk that already cites a test is still searched |
| M8 | spent budget overrides `no-candidate-test` |

Load averages ran from 13 to 55 (shared host with other lanes).

## Residuals

- **Help text.** `cmd/corvint/help.go` does not list `--guarding-tests`. That file is outside this
  lane's paths.
- **Automation.** `internal/dogfoodflow` does not run the step automatically.
- **Cross-package guards.** Guard discovery is limited to one directory, so a test in
  `cmd/corvint` is never a candidate.
- **Candidate narrowing (K5).** One idea is to cap or rank candidates per package, so that a
  package with 394 tests does not spend the whole budget.
- **Header inconsistency.** The V1-1266 section of `cem-stable-v1.md` says `Status: proposed`,
  while its rows say accepted. That section is an accepted surface, so this lane did not edit it.
